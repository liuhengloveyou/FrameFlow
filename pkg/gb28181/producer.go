package gb28181

import (
	"encoding/binary"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/liuhengloveyou/FrameFlow/pkg/core"
	"github.com/liuhengloveyou/FrameFlow/pkg/fferrors"
	"github.com/liuhengloveyou/FrameFlow/pkg/h264"
	"github.com/liuhengloveyou/FrameFlow/pkg/h264/annexb"
	"github.com/liuhengloveyou/FrameFlow/pkg/h265"
	"github.com/liuhengloveyou/FrameFlow/pkg/mpegps"
	"github.com/pion/rtp"
)

// Producer 实现 gb28181://device/channel 实时预览的 core.Producer。
//
// 视频组帧对齐 monibuca MpegPsDemuxer：同一 PTS 的 PES 载荷拼成一个 Annex-B
// 访问单元，再一次性转成 AVCC。若按 PES 直接发帧会花屏（半截 NALU / 空 AVCC）。
// 参数集会缓存并在 IDR AU 中注入（对齐 monibuca AnnexB.Parse）。
type Producer struct {
	core.Connection

	deviceID  string
	channelID string
	transport string
	stream    string // main | sub
	pbStart   time.Time
	pbEnd     time.Time
	playback  bool

	gw      *Gateway
	session *InviteSession
	demux   *mpegps.Demuxer

	videoType byte
	audioType byte

	// 视频 AU 重组（优先按 PES PTS；无 PTS 时用 annex-B 起始码）
	vBuf    []byte
	vPTS    uint64
	vHasPTS bool
	vRTPTs  uint32

	// 此前 AU 缓存的 H264/H265 参数集（裸 NAL，无长度前缀）
	h264SPS []byte
	h264PPS []byte
	h265VPS []byte
	h265SPS []byte
	h265PPS []byte

	pktCh  chan rtpPacket
	errCh  chan error
	stopCh chan struct{}
	once   sync.Once

	demuxPkts        atomic.Uint64
	demuxDrop        atomic.Uint64
	dispatchVid      atomic.Uint64
	dispatchAud      atomic.Uint64
	emptyAVCC        atomic.Uint64
	noRecv           atomic.Uint64
	loggedFirstDemux atomic.Bool
	loggedFirstDisp  atomic.Bool
}

type rtpPacket struct {
	streamType byte
	ts         uint32 // RTP clock (fallback)
	pts        uint64 // PES PTS (90kHz), preferred for AU boundary
	hasPTS     bool
	payload    []byte
}

type videoAU struct {
	avcc []byte
	ts   uint32
}

// Dial opens a GB28181 live producer from URL
// gb28181://deviceID/channelID[?transport=udp|tcp-passive|tcp-active|auto]
func Dial(rawURL string) (core.Producer, error) {
	gw := DefaultGateway()
	if gw == nil {
		return nil, fferrors.ErrGbGatewayDisabled
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	deviceID := u.Host
	channelID := strings.Trim(u.Path, "/")
	if deviceID == "" {
		rest := strings.Trim(u.Path, "/")
		if rest == "" {
			rest = strings.Trim(u.Opaque, "/")
		}
		parts := strings.SplitN(rest, "/", 2)
		deviceID = parts[0]
		if len(parts) > 1 {
			channelID = parts[1]
		}
	}
	if i := strings.IndexByte(channelID, '/'); i >= 0 {
		channelID = channelID[:i]
	}
	if deviceID == "" {
		return nil, fmt.Errorf("gb28181: missing device id in %q", rawURL)
	}
	if channelID == "" {
		channelID = deviceID
	}
	transport := u.Query().Get("transport")
	if transport == "" {
		transport = gw.Config.Transport
	}
	transport = NormalizeMediaTransport(transport)
	if transport == TransportAuto {
		transport = NormalizeMediaTransport(gw.Config.Transport)
		if transport == TransportAuto {
			transport = TransportUDP
		}
	}
	streamProfile := NormalizeStreamProfile(u.Query().Get("stream"))
	var pbStart, pbEnd time.Time
	playback := false
	if s := u.Query().Get("start"); s != "" {
		pbStart = parseURLTime(s)
	}
	if s := u.Query().Get("end"); s != "" {
		pbEnd = parseURLTime(s)
	}
	if !pbStart.IsZero() && !pbEnd.IsZero() && pbEnd.After(pbStart) {
		playback = true
	}

	log.Info().
		Str("url", rawURL).
		Str("device", deviceID).
		Str("channel", channelID).
		Str("transport", transport).
		Str("stream", streamProfile).
		Bool("playback", playback).
		Msg("[gb28181] dial")

	p := &Producer{
		Connection: core.Connection{
			ID:         core.NewID(),
			FormatName: "gb28181",
			Protocol:   transport,
			URL:        rawURL,
		},
		deviceID:  deviceID,
		channelID: channelID,
		transport: transport,
		stream:    streamProfile,
		pbStart:   pbStart,
		pbEnd:     pbEnd,
		playback:  playback,
		gw:        gw,
		demux:     mpegps.NewDemuxer(),
		pktCh:     make(chan rtpPacket, 256),
		errCh:     make(chan error, 1),
		stopCh:    make(chan struct{}),
	}
	if err := p.inviteAndProbe(); err != nil {
		log.Error().
			Err(err).
			Str("device", deviceID).
			Str("channel", channelID).
			Msg("[gb28181] dial failed")
		_ = p.Stop()
		return nil, err
	}
	return p, nil
}

func (p *Producer) label() string {
	return p.deviceID + "/" + p.channelID
}

// parseURLTime accepts unix seconds/millis or GB/RFC3339 wall-clock strings.
func parseURLTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	if unix, err := strconv.ParseInt(s, 10, 64); err == nil {
		// 13+ digits → millis; 10 digits → seconds
		if unix >= 1_000_000_000_000 {
			return time.UnixMilli(unix)
		}
		return time.Unix(unix, 0)
	}
	if t, err := ParseRecordTime(s); err == nil {
		return t
	}
	return time.Time{}
}

func (p *Producer) inviteAndProbe() error {
	dev, ch, ok := p.gw.Registry.Channel(p.deviceID, p.channelID)
	if !ok || dev == nil {
		return fferrors.ErrGbDeviceChannelNotFound
	}
	if !dev.Online {
		return fferrors.ErrGbDeviceOffline
	}
	_ = ch

	port, err := p.gw.Ports.Acquire()
	if err != nil {
		return err
	}
	mediaIP := p.gw.Config.MediaIP
	if mediaIP == "" {
		mediaIP = p.gw.Config.SipIP
	}
	ssrc := p.gw.NextSSRC()

	media := &MediaReceiver{
		Transport: p.transport,
		LocalIP:   mediaIP,
		Port:      port,
		Label:     p.label(),
	}
	media.OnPayload = func(payload []byte, ts uint32) {
		_, _ = p.demux.Write(payload)
		for {
			pkt, e := p.demux.Next()
			if e != nil {
				break
			}
			n := p.demuxPkts.Add(1)
			if !p.loggedFirstDemux.Swap(true) {
				log.Info().
					Str("device", p.deviceID).
					Str("channel", p.channelID).
					Uint8("stream_type", pkt.StreamType).
					Bool("is_video", mpegps.IsVideo(pkt.StreamType)).
					Bool("is_audio", mpegps.IsAudio(pkt.StreamType)).
					Bool("has_pts", pkt.HasPTS).
					Uint64("pts", pkt.PTS).
					Int("pes", len(pkt.Payload)).
					Uint32("rtp_ts", ts).
					Msg("[gb28181] demux: first pes")
			} else if n == 50 || n == 200 || n%500 == 0 {
				log.Info().
					Str("device", p.deviceID).
					Str("channel", p.channelID).
					Uint64("demux", n).
					Uint64("drop", p.demuxDrop.Load()).
					Uint8("stream_type", pkt.StreamType).
					Int("pes", len(pkt.Payload)).
					Msg("[gb28181] demux: stats")
			}
			select {
			case p.pktCh <- rtpPacket{
				streamType: pkt.StreamType,
				ts:         ts,
				pts:        pkt.PTS,
				hasPTS:     pkt.HasPTS,
				payload:    pkt.Payload,
			}:
			case <-p.stopCh:
				return
			default:
				dropped := p.demuxDrop.Add(1)
				if dropped == 1 || dropped%100 == 0 {
					log.Warn().
						Str("device", p.deviceID).
						Str("channel", p.channelID).
						Uint64("drop", dropped).
						Msg("[gb28181] demux: pktCh full, dropping")
				}
			}
		}
	}
	needListen := NormalizeMediaTransport(p.transport) != TransportTCPActive
	if needListen {
		if err := media.Listen(); err != nil {
			p.gw.Ports.Release(port)
			return err
		}
	}

	if p.gw.InviteLive == nil && !p.playback {
		_ = media.Close()
		p.gw.Ports.Release(port)
		return fferrors.ErrGbSipNotStarted
	}
	if p.playback && p.gw.InvitePlayback == nil {
		_ = media.Close()
		p.gw.Ports.Release(port)
		return fferrors.ErrGbSipNotStarted
	}

	log.Info().
		Str("device", p.deviceID).
		Str("channel", p.channelID).
		Str("transport", p.transport).
		Str("stream", p.stream).
		Bool("playback", p.playback).
		Str("media_ip", mediaIP).
		Uint16("media_port", port).
		Str("ssrc", ssrc).
		Str("remote", dev.RemoteAddr).
		Bool("online", dev.Online).
		Str("status", dev.Status).
		Msg("[gb28181] invite: sending")

	var callID string
	var sipBye func() error
	var answer []byte
	var errInvite error
	if p.playback {
		callID, sipBye, answer, errInvite = p.gw.InvitePlayback(p.deviceID, p.channelID, p.transport, port, ssrc, p.pbStart, p.pbEnd)
	} else {
		callID, sipBye, answer, errInvite = p.gw.InviteLive(p.deviceID, p.channelID, p.transport, port, ssrc, p.stream)
	}
	if errInvite != nil {
		_ = media.Close()
		p.gw.Ports.Release(port)
		return errInvite
	}
	if NormalizeMediaTransport(p.transport) == TransportTCPActive {
		ip, rport, ok := ParseAnswerMedia(answer)
		if !ok {
			_ = media.Close()
			p.gw.Ports.Release(port)
			if sipBye != nil {
				_ = sipBye()
			}
			return fmt.Errorf("gb28181: tcp-active missing media address in answer SDP")
		}
		if err := media.DialTCP(ip, rport); err != nil {
			_ = media.Close()
			p.gw.Ports.Release(port)
			if sipBye != nil {
				_ = sipBye()
			}
			return err
		}
	}
	session := &InviteSession{
		DeviceID:  p.deviceID,
		ChannelID: p.channelID,
		CallID:    callID,
		SSRC:      ssrc,
		Media:     media,
	}
	session.SetBye(func() error {
		var e1, e2 error
		if sipBye != nil {
			e1 = sipBye()
		}
		p.gw.Ports.Release(port)
		e2 = media.Close()
		if e1 != nil {
			return e1
		}
		return e2
	})
	p.session = session

	deadline := time.Now().Add(8 * time.Second)
	var gotVideo, gotAudio bool
	for time.Now().Before(deadline) && (!gotVideo || !gotAudio) {
		select {
		case pkt := <-p.pktCh:
			p.consumeProbe(pkt, &gotVideo, &gotAudio)
		case err := <-p.errCh:
			return err
		case <-time.After(200 * time.Millisecond):
		}
	}
	// 冲刷 probe 期间缓冲的残留视频 AU
	if au, ok := p.flushVideoAU(); ok {
		p.consumeProbeAU(au, &gotVideo)
	}

	rtpN := uint64(0)
	if media != nil {
		rtpN = media.rtpCount.Load()
	}
	fallback := false
	if len(p.Medias) == 0 {
		fallback = true
		// 必须是 PayloadTypeRAW：载荷是 AVCC，不是 RFC6184 RTP。
		// 把 AVCC 当 RTP 解会出现 NALU type 0、花屏、绿屏。
		codec := &core.Codec{
			Name:        core.CodecH264,
			ClockRate:   90000,
			PayloadType: core.PayloadTypeRAW,
		}
		p.Medias = append(p.Medias, &core.Media{
			Kind: core.KindVideo, Direction: core.DirectionRecvonly, Codecs: []*core.Codec{codec},
		})
		p.videoType = mpegps.StreamTypeH264
	}

	codecs := make([]string, 0, len(p.Medias))
	for _, m := range p.Medias {
		for _, c := range m.Codecs {
			codecs = append(codecs, m.Kind+":"+c.Name)
		}
	}
	log.Info().
		Str("device", p.deviceID).
		Str("channel", p.channelID).
		Str("callid", callID).
		Bool("got_video", gotVideo).
		Bool("got_audio", gotAudio).
		Bool("fallback_h264", fallback).
		Uint64("rtp", rtpN).
		Uint64("demux", p.demuxPkts.Load()).
		Strs("medias", codecs).
		Msg("[gb28181] probe done")

	if fallback && rtpN == 0 {
		log.Warn().
			Str("device", p.deviceID).
			Str("channel", p.channelID).
			Str("media_ip", mediaIP).
			Uint16("media_port", port).
			Str("listen", media.LocalAddr()).
			Msg("[gb28181] probe: no rtp received — check media_ip reachable from device")
		// 无码流仍占着 INVITE，会导致后续 486 Busy；失败并 BYE
		return fferrors.ErrGbNoRtp
	} else if fallback {
		log.Warn().
			Str("device", p.deviceID).
			Str("channel", p.channelID).
			Uint64("rtp", rtpN).
			Uint64("demux", p.demuxPkts.Load()).
			Msg("[gb28181] probe: rtp arrived but no video/audio AU demuxed")
	}
	return nil
}

// pushVideoPES 追加一路视频 PES。新访问单元开始时（PTS 变化，或缓冲后出现
// annex-B 起始码）把上一 AU 转成 AVCC 返回。对齐 monibuca MpegPsDemuxer 视频路径。
func (p *Producer) pushVideoPES(pkt rtpPacket) (au videoAU, ready bool) {
	flush := false
	// monibuca：仅当 Pts != 0 时按 PTS 切帧（0 视为未设置）。
	if pkt.hasPTS && pkt.pts != 0 && p.vHasPTS && p.vPTS != 0 && pkt.pts != p.vPTS && len(p.vBuf) > 0 {
		flush = true
	} else if !pkt.hasPTS && bytesHasStartCode(pkt.payload) && len(p.vBuf) > 0 {
		// 无 PTS：把 annex-B 起始码当作 AU 边界
		flush = true
	}

	if flush {
		au, ready = p.flushVideoAU()
	}

	p.vBuf = append(p.vBuf, pkt.payload...)
	if pkt.hasPTS && pkt.pts != 0 {
		p.vPTS = pkt.pts
		p.vHasPTS = true
	}
	if len(p.vBuf) == len(pkt.payload) {
		// 本 AU 起始字节 — 保留 AU 起点的 RTP 时间戳
		p.vRTPTs = pkt.ts
	}
	return au, ready
}

func (p *Producer) flushVideoAU() (au videoAU, ready bool) {
	if len(p.vBuf) == 0 {
		return videoAU{}, false
	}
	avcc := annexb.EncodeToAVCC(p.vBuf)
	ts := p.vRTPTs
	annexbLen := len(p.vBuf)
	p.vBuf = p.vBuf[:0]
	p.vHasPTS = false
	if len(avcc) < 5 {
		n := p.emptyAVCC.Add(1)
		if n == 1 || n%50 == 0 {
			log.Warn().
				Str("device", p.deviceID).
				Str("channel", p.channelID).
				Uint64("count", n).
				Int("annexb", annexbLen).
				Int("avcc", len(avcc)).
				Msg("[gb28181] flush: empty/short avcc (skipped)")
		}
		return videoAU{}, false
	}
	avcc = p.prepareVideoAVCC(avcc)
	if len(avcc) < 5 {
		return videoAU{}, false
	}
	return videoAU{avcc: avcc, ts: ts}, true
}

// prepareVideoAVCC 缓存参数集，并在 IDR AU 中注入。
// 仅含参数集的 AU 只进缓存、不发布（对齐 monibuca）。
func (p *Producer) prepareVideoAVCC(avcc []byte) []byte {
	switch p.videoType {
	case mpegps.StreamTypeH265:
		return p.prepareH265AVCC(avcc)
	default:
		return p.prepareH264AVCC(avcc)
	}
}

func (p *Producer) prepareH264AVCC(avcc []byte) []byte {
	if sps, pps := h264.ExtractParameterSets(avcc); len(sps) > 0 || len(pps) > 0 {
		if len(sps) > 0 {
			p.h264SPS = sps
		}
		if len(pps) > 0 {
			p.h264PPS = pps
		}
		p.refreshVideoFmtp()
	}
	if h264.ParamSetsOnly(avcc) {
		return nil
	}
	return h264.EnsureKeyframeParamSets(avcc, p.h264SPS, p.h264PPS)
}

func (p *Producer) prepareH265AVCC(avcc []byte) []byte {
	b := avcc
	onlyPS := true
	hasVPS, hasSPS, hasPPS := false, false, false
	for len(b) >= 5 {
		size := 4 + int(binary.BigEndian.Uint32(b))
		if size < 5 || size > len(b) {
			onlyPS = false
			break
		}
		nal := b[4:size]
		nt := h265.NALUType(b)
		switch nt {
		case h265.NALUTypeVPS:
			p.h265VPS = append([]byte(nil), nal...)
			hasVPS = true
		case h265.NALUTypeSPS:
			p.h265SPS = append([]byte(nil), nal...)
			hasSPS = true
		case h265.NALUTypePPS:
			p.h265PPS = append([]byte(nil), nal...)
			hasPPS = true
		default:
			onlyPS = false
		}
		if size == len(b) {
			break
		}
		b = b[size:]
	}
	if onlyPS {
		p.refreshVideoFmtp()
		return nil
	}
	if h265.IsKeyframe(avcc) && !(hasVPS && hasSPS && hasPPS) &&
		len(p.h265VPS) > 0 && len(p.h265SPS) > 0 && len(p.h265PPS) > 0 {
		ps := h264.JoinNALU(p.h265VPS, p.h265SPS, p.h265PPS)
		avcc = h264.Join(ps, avcc)
	}
	p.refreshVideoFmtp()
	return avcc
}

func (p *Producer) refreshVideoFmtp() {
	for _, m := range p.Medias {
		if m == nil || m.Kind != core.KindVideo {
			continue
		}
		for _, c := range m.Codecs {
			if c == nil {
				continue
			}
			switch c.Name {
			case core.CodecH264:
				if len(p.h264SPS) > 0 && len(p.h264PPS) > 0 {
					c.FmtpLine = h264.GetFmtpLine(h264.JoinNALU(p.h264SPS, p.h264PPS))
					c.PayloadType = core.PayloadTypeRAW
				}
			case core.CodecH265:
				if len(p.h265VPS) > 0 && len(p.h265SPS) > 0 && len(p.h265PPS) > 0 {
					c.FmtpLine = h265.AVCCToCodec(h264.JoinNALU(p.h265VPS, p.h265SPS, p.h265PPS)).FmtpLine
					c.PayloadType = core.PayloadTypeRAW
				}
			}
		}
	}
}

func (p *Producer) consumeProbe(pkt rtpPacket, gotVideo, gotAudio *bool) {
	switch {
	case mpegps.IsVideo(pkt.streamType) || pkt.streamType == 0 && looksLikeVideo(pkt.payload):
		if au, ok := p.pushVideoPES(pkt); ok {
			p.consumeProbeAU(au, gotVideo)
		}
		// 尽早标记流类型
		if p.videoType == 0 {
			if pkt.streamType == mpegps.StreamTypeH265 {
				p.videoType = mpegps.StreamTypeH265
			} else if mpegps.IsVideo(pkt.streamType) {
				p.videoType = pkt.streamType
			} else {
				p.videoType = mpegps.StreamTypeH264
			}
		}
	case mpegps.IsAudio(pkt.streamType):
		if *gotAudio {
			return
		}
		name := core.CodecPCMA
		if pkt.streamType == mpegps.StreamTypeG711U {
			name = core.CodecPCMU
		}
		codec := &core.Codec{Name: name, ClockRate: 8000, Channels: 1}
		p.Medias = append(p.Medias, &core.Media{
			Kind: core.KindAudio, Direction: core.DirectionRecvonly, Codecs: []*core.Codec{codec},
		})
		p.audioType = pkt.streamType
		*gotAudio = true
		log.Info().
			Str("device", p.deviceID).
			Str("channel", p.channelID).
			Str("codec", name).
			Uint8("stream_type", pkt.streamType).
			Int("payload", len(pkt.payload)).
			Msg("[gb28181] probe: audio ready")
	default:
		log.Debug().
			Str("device", p.deviceID).
			Uint8("stream_type", pkt.streamType).
			Int("payload", len(pkt.payload)).
			Bool("startcode", looksLikeVideo(pkt.payload)).
			Msg("[gb28181] probe: unmatched pes")
	}
}

func (p *Producer) consumeProbeAU(au videoAU, gotVideo *bool) {
	if len(au.avcc) < 5 {
		return
	}
	// probe 阶段始终刷新参数集缓存。
	_ = p.prepareVideoAVCC(append([]byte(nil), au.avcc...))

	if *gotVideo {
		return
	}
	var codec *core.Codec
	st := p.videoType
	if st == 0 {
		st = mpegps.StreamTypeH264
	}
	switch {
	case st == mpegps.StreamTypeH265 || h265.NALUType(au.avcc) == h265.NALUTypeVPS:
		codec = h265.AVCCToCodec(au.avcc)
		st = mpegps.StreamTypeH265
	default:
		codec = h264.AVCCToCodec(au.avcc)
		st = mpegps.StreamTypeH264
	}
	if codec == nil || codec.Name == "" {
		codec = &core.Codec{Name: core.CodecH264, ClockRate: 90000, PayloadType: core.PayloadTypeRAW}
	}
	codec.PayloadType = core.PayloadTypeRAW

	// 优先采用带 SPS 的首个 AU，便于 fmtp/sprop 供 RepairAVCC 使用。
	hasPS := strings.Contains(codec.FmtpLine, "sprop-parameter-sets=") ||
		strings.Contains(codec.FmtpLine, "sprop-sps=")
	if !hasPS && !h264.IsKeyframe(au.avcc) && !h265.IsKeyframe(au.avcc) {
		// probe 窗口内再等一会儿带 SPS 的 AU。
		return
	}

	p.Medias = append(p.Medias, &core.Media{
		Kind: core.KindVideo, Direction: core.DirectionRecvonly, Codecs: []*core.Codec{codec},
	})
	p.videoType = st
	*gotVideo = true
	log.Info().
		Str("device", p.deviceID).
		Str("channel", p.channelID).
		Str("codec", codec.Name).
		Uint8("stream_type", st).
		Int("avcc", len(au.avcc)).
		Uint8("nalu", h264.NALUType(au.avcc)).
		Bool("has_ps", hasPS).
		Msg("[gb28181] probe: video ready")
}

func looksLikeVideo(b []byte) bool {
	return bytesHasStartCode(b)
}

func bytesHasStartCode(b []byte) bool {
	for i := 0; i+3 < len(b); i++ {
		if b[i] == 0 && b[i+1] == 0 && (b[i+2] == 1 || (b[i+2] == 0 && b[i+3] == 1)) {
			return true
		}
	}
	return false
}

func (p *Producer) Start() error {
	log.Info().
		Str("device", p.deviceID).
		Str("channel", p.channelID).
		Int("receivers", len(p.Receivers)).
		Uint8("video_type", p.videoType).
		Msg("[gb28181] producer start")
	for {
		select {
		case <-p.stopCh:
			if au, ok := p.flushVideoAU(); ok {
				p.writeVideoAU(au)
			}
			log.Info().
				Str("device", p.deviceID).
				Str("channel", p.channelID).
				Uint64("demux", p.demuxPkts.Load()).
				Uint64("drop", p.demuxDrop.Load()).
				Uint64("video_out", p.dispatchVid.Load()).
				Uint64("audio_out", p.dispatchAud.Load()).
				Uint64("empty_avcc", p.emptyAVCC.Load()).
				Uint64("no_recv", p.noRecv.Load()).
				Msg("[gb28181] producer stop")
			return nil
		case err := <-p.errCh:
			log.Error().Err(err).Str("device", p.deviceID).Msg("[gb28181] producer error")
			return err
		case pkt := <-p.pktCh:
			p.dispatch(pkt)
		}
	}
}

func (p *Producer) dispatch(pkt rtpPacket) {
	isVideo := mpegps.IsVideo(pkt.streamType) ||
		pkt.streamType == p.videoType ||
		(p.videoType != 0 && looksLikeVideo(pkt.payload) && !mpegps.IsAudio(pkt.streamType))

	if isVideo {
		if au, ok := p.pushVideoPES(pkt); ok {
			p.writeVideoAU(au)
		}
		return
	}

	if mpegps.IsAudio(pkt.streamType) || pkt.streamType == p.audioType {
		for _, recv := range p.Receivers {
			matchAudio := recv.Codec != nil && (recv.Codec.Name == core.CodecPCMA || recv.Codec.Name == core.CodecPCMU)
			if !matchAudio {
				continue
			}
			rtpPkt := &rtp.Packet{
				Header:  rtp.Header{Timestamp: pkt.ts, PayloadType: 8},
				Payload: pkt.payload,
			}
			p.Recv += len(pkt.payload)
			p.dispatchAud.Add(1)
			recv.WriteRTP(rtpPkt)
			return
		}
		n := p.noRecv.Add(1)
		if n == 1 || n%200 == 0 {
			log.Warn().
				Str("device", p.deviceID).
				Uint64("count", n).
				Uint8("stream_type", pkt.streamType).
				Msg("[gb28181] dispatch: no matching audio receiver")
		}
	}
}

func (p *Producer) writeVideoAU(au videoAU) {
	if len(au.avcc) < 5 {
		return
	}
	for _, recv := range p.Receivers {
		matchVideo := recv.Codec != nil && (recv.Codec.Name == core.CodecH264 || recv.Codec.Name == core.CodecH265)
		if !matchVideo {
			continue
		}
		rtpPkt := &rtp.Packet{
			Header: rtp.Header{
				Version:        h264.RTPPacketVersionAVC, // 0：RTPPay 按 AVCC 分包
				Timestamp:      au.ts,
				PayloadType:    core.PayloadTypeRAW,
			},
			Payload: au.avcc,
		}
		p.Recv += len(au.avcc)
		n := p.dispatchVid.Add(1)
		if !p.loggedFirstDisp.Swap(true) {
			log.Info().
				Str("device", p.deviceID).
				Str("channel", p.channelID).
				Str("codec", recv.Codec.Name).
				Int("avcc", len(au.avcc)).
				Uint8("nalu", h264.NALUType(au.avcc)).
				Bool("keyframe", h264.IsKeyframe(au.avcc) || h265.IsKeyframe(au.avcc)).
				Int("receivers", len(p.Receivers)).
				Msg("[gb28181] dispatch: first video AU to track")
		} else if n == 50 || n == 200 || n%500 == 0 {
			log.Info().
				Str("device", p.deviceID).
				Str("channel", p.channelID).
				Uint64("video_out", n).
				Uint64("empty_avcc", p.emptyAVCC.Load()).
				Int("avcc", len(au.avcc)).
				Msg("[gb28181] dispatch: video stats")
		}
		recv.WriteRTP(rtpPkt)
		return
	}
	n := p.noRecv.Add(1)
	if n == 1 || n%200 == 0 {
		log.Warn().
			Str("device", p.deviceID).
			Str("channel", p.channelID).
			Uint64("count", n).
			Int("avcc", len(au.avcc)).
			Int("receivers", len(p.Receivers)).
			Msg("[gb28181] dispatch: no matching video receiver")
	}
}

func (p *Producer) Stop() error {
	p.once.Do(func() { close(p.stopCh) })
	if p.session != nil {
		_ = p.session.Close()
	}
	return p.Connection.Stop()
}
