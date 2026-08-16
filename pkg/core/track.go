package core

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/pion/rtp"
)

var ErrCantGetTrack = errors.New("can't get track")

type Receiver struct {
	Node

	// Deprecated: should be removed
	Media *Media `json:"-"`
	// Deprecated: should be removed
	ID byte `json:"-"` // Channel for RTSP, PayloadType for MPEG-TS

	Bytes   int `json:"bytes,omitempty"`
	Packets int `json:"packets,omitempty"`

	// ring: H264/H265 video history; nil for audio / other codecs (fan-out path).
	ring *AVRing

	// Last in-band parameter-set RTP payloads (H264 SPS/PPS). Used when mid-join
	// starts at IDR so RTPDepay can assemble a decodable AU (avoids green flash).
	paramMu sync.Mutex
	h264SPS []byte
	h264PPS []byte

	// RTP AU aggregation (monibuca: same Timestamp → one VideoFrame).
	auMu   sync.Mutex
	auTS   uint32
	auHas  bool
	auIDR  bool
	auPkts []Packet
}

func NewReceiver(media *Media, codec *Codec) *Receiver {
	r := &Receiver{
		Node:  Node{id: NewID(), Codec: codec},
		Media: media,
	}

	useRing := codec != nil && (codec.Name == CodecH264 || codec.Name == CodecH265)
	if useRing {
		r.ring = NewAVRing()
		codecName := codec.Name
		isRTP := codec.IsRTP()
		if isRTP && codecName == CodecH264 && codec.FmtpLine != "" {
			sps, pps := parameterSetFromFmtp(codec.FmtpLine)
			r.h264SPS, r.h264PPS = sps, pps
		}
		log.Info().
			Uint32("recv", r.id).
			Str("codec", codecName).
			Bool("rtp", isRTP).
			Bool("has_sprop", len(r.h264SPS) > 0 && len(r.h264PPS) > 0).
			Msg("[avring] receiver using ring")
		r.Input = func(packet *Packet) {
			r.Bytes += len(packet.Payload)
			r.Packets++
			if isRTP {
				r.noteH264ParameterRTP(packet.Payload)
				r.appendRTPAU(codecName, packet)
				return
			}
			// RAW AVCC：从带内 NALU 刷新 SPS/PPS，供 RepairAVCC / 中途入流注入参数集
			//（国标常把 SPS/PPS 放在更早的 AU 里）。
			if codecName == CodecH264 {
				r.noteH264ParameterAVCC(packet.Payload)
			}
			idr := packetIsIDR(codecName, isRTP, packet.Payload)
			// RAW：一包约等于一个 AU。不要把裸 SPS/PPS 拆成独立包前置——
			// 下游期望单个带长度前缀的 AVCC；参数集由 producer / RepairAVCC 写入 AU 内。
			r.ring.Write(packet, idr, nil)
		}
		return r
	}

	r.Input = func(packet *Packet) {
		r.Bytes += len(packet.Payload)
		r.Packets++

		// 与 AppendChild 同一把锁做快照：否则在弱内存序平台（如 Apple Silicon）上，
		// 晚加入的 consumer 可能永远看不到 childs 更新，表现为推流成功但 RTMP/FLV 拉流 vq=0。
		r.mu.Lock()
		childs := r.childs
		r.mu.Unlock()
		for _, child := range childs {
			child.Input(packet)
		}
	}
	return r
}

// HasAVRing reports whether this receiver uses the video history ring.
func (r *Receiver) HasAVRing() bool {
	return r != nil && r.ring != nil
}

// SnapshotFromIDR returns the newest IDR packet from the ring (Payload read-only).
func (r *Receiver) SnapshotFromIDR() (*Packet, bool) {
	if r == nil || r.ring == nil {
		return nil, false
	}
	return r.ring.SnapshotFromIDR()
}

// noteH264ParameterRTP 缓存完整的 SPS/PPS RTP 载荷，并刷新 Codec.FmtpLine。
func (r *Receiver) noteH264ParameterRTP(payload []byte) {
	if r == nil || len(payload) < 1 {
		return
	}
	nalType := payload[0] & 0x1F
	switch nalType {
	case 7, 8:
		cp := append([]byte(nil), payload...)
		r.paramMu.Lock()
		if nalType == 7 {
			r.h264SPS = cp
		} else {
			r.h264PPS = cp
		}
		r.syncH264FmtpLocked()
		r.paramMu.Unlock()
	case 24: // STAP-A
		r.noteH264STAP(payload[1:])
	}
}

// noteH264ParameterAVCC 从 RAW AVCC 访问单元中缓存 SPS/PPS。
func (r *Receiver) noteH264ParameterAVCC(avcc []byte) {
	if r == nil {
		return
	}
	var sps, pps []byte
	b := avcc
	for len(b) >= 5 {
		size := 4 + int(binary.BigEndian.Uint32(b))
		if size < 5 || size > len(b) {
			break
		}
		nal := b[4:size]
		switch nal[0] & 0x1F {
		case 7:
			sps = append([]byte(nil), nal...)
		case 8:
			pps = append([]byte(nil), nal...)
		}
		if size == len(b) {
			break
		}
		b = b[size:]
	}
	if len(sps) == 0 && len(pps) == 0 {
		return
	}
	r.paramMu.Lock()
	if len(sps) > 0 {
		r.h264SPS = sps
	}
	if len(pps) > 0 {
		r.h264PPS = pps
	}
	r.syncH264FmtpLocked()
	r.paramMu.Unlock()
}

func (r *Receiver) noteH264STAP(b []byte) {
	for len(b) >= 2 {
		n := int(b[0])<<8 | int(b[1])
		b = b[2:]
		if n <= 0 || n > len(b) {
			return
		}
		nal := b[:n]
		b = b[n:]
		if len(nal) < 1 {
			continue
		}
		nt := nal[0] & 0x1F
		if nt != 7 && nt != 8 {
			continue
		}
		cp := append([]byte(nil), nal...)
		r.paramMu.Lock()
		if nt == 7 {
			r.h264SPS = cp
		} else {
			r.h264PPS = cp
		}
		r.syncH264FmtpLocked()
		r.paramMu.Unlock()
	}
}

// syncH264FmtpLocked updates Codec.FmtpLine from cached SPS/PPS (caller holds paramMu).
// Built here to avoid an import cycle with package h264.
func (r *Receiver) syncH264FmtpLocked() {
	if r.Codec == nil || len(r.h264SPS) < 4 || len(r.h264PPS) == 0 {
		return
	}
	r.Codec.FmtpLine = fmt.Sprintf(
		"packetization-mode=1;profile-level-id=%s;sprop-parameter-sets=%s,%s",
		hex.EncodeToString(r.h264SPS[1:4]),
		base64.StdEncoding.EncodeToString(r.h264SPS),
		base64.StdEncoding.EncodeToString(r.h264PPS),
	)
}

// appendRTPAU aggregates same-timestamp RTP into one ring AU (monibuca transceiver).
func (r *Receiver) appendRTPAU(codecName string, packet *Packet) {
	if r == nil || r.ring == nil || packet == nil {
		return
	}
	idr := packetIsIDR(codecName, true, packet.Payload)
	cp := *packet
	packet.Payload = nil // move

	r.auMu.Lock()
	defer r.auMu.Unlock()
	if r.auHas && cp.Timestamp != r.auTS {
		r.flushAULocked()
	}
	if !r.auHas {
		r.auTS = cp.Timestamp
		r.auHas = true
	}
	r.auPkts = append(r.auPkts, cp)
	if idr {
		r.auIDR = true
	}
}

func (r *Receiver) flushAULocked() {
	if !r.auHas || len(r.auPkts) == 0 {
		r.auHas = false
		r.auIDR = false
		r.auPkts = nil
		return
	}
	pkts := r.auPkts
	idr := r.auIDR
	r.auPkts = nil
	r.auIDR = false
	r.auHas = false

	if idr && !rtpAUHasH264PS(pkts) {
		sets := r.cloneParamSets()
		if len(sets) > 0 {
			ts := pkts[0].Timestamp
			prepend := make([]Packet, 0, len(sets)+len(pkts))
			for _, ps := range sets {
				if len(ps) == 0 {
					continue
				}
				p := Packet{Payload: append([]byte(nil), ps...)}
				p.Timestamp = ts
				// Marker=false — do not flush PS alone on RTSP interleaved.
				prepend = append(prepend, p)
			}
			pkts = append(prepend, pkts...)
		}
	}
	r.ring.WriteAU(pkts, idr)
}

func rtpAUHasH264PS(pkts []Packet) bool {
	hasSPS, hasPPS := false, false
	for i := range pkts {
		p := pkts[i].Payload
		if len(p) < 1 {
			continue
		}
		switch p[0] & 0x1F {
		case 7:
			hasSPS = true
		case 8:
			hasPPS = true
		case 24: // STAP-A
			b := p[1:]
			for len(b) >= 2 {
				n := int(b[0])<<8 | int(b[1])
				b = b[2:]
				if n <= 0 || n > len(b) {
					break
				}
				nt := b[0] & 0x1F
				if nt == 7 {
					hasSPS = true
				} else if nt == 8 {
					hasPPS = true
				}
				b = b[n:]
			}
		}
	}
	return hasSPS && hasPPS
}

// cloneParamSets returns cached SPS/PPS RTP NAL payloads (or fmtp sprop) for IDR binding.
func (r *Receiver) cloneParamSets() [][]byte {
	if r == nil {
		return nil
	}
	r.paramMu.Lock()
	sps := append([]byte(nil), r.h264SPS...)
	pps := append([]byte(nil), r.h264PPS...)
	fmtp := ""
	if r.Codec != nil {
		fmtp = r.Codec.FmtpLine
	}
	r.paramMu.Unlock()

	if len(sps) == 0 || len(pps) == 0 {
		sps2, pps2 := parameterSetFromFmtp(fmtp)
		if len(sps) == 0 {
			sps = sps2
		}
		if len(pps) == 0 {
			pps = pps2
		}
	}
	var out [][]byte
	if len(sps) > 0 {
		out = append(out, sps)
	}
	if len(pps) > 0 {
		out = append(out, pps)
	}
	return out
}

// ParameterSetRTPPackets returns cached SPS/PPS as RTP packets (fallback inject).
func (r *Receiver) ParameterSetRTPPackets() []*Packet {
	sets := r.cloneParamSets()
	if len(sets) == 0 {
		return nil
	}
	out := make([]*Packet, 0, len(sets))
	for _, ps := range sets {
		p := &Packet{Payload: ps}
		p.Marker = true
		out = append(out, p)
	}
	return out
}

func parameterSetFromFmtp(fmtp string) (sps, pps []byte) {
	if fmtp == "" {
		return nil, nil
	}
	const key = "sprop-parameter-sets="
	i := strings.Index(fmtp, key)
	if i < 0 {
		return nil, nil
	}
	s := fmtp[i+len(key):]
	if j := strings.IndexByte(s, ';'); j >= 0 {
		s = s[:j]
	}
	k := strings.IndexByte(s, ',')
	if k < 0 {
		return nil, nil
	}
	sps, _ = base64.StdEncoding.DecodeString(s[:k])
	pps, _ = base64.StdEncoding.DecodeString(s[k+1:])
	return sps, pps
}

// Deprecated: should be removed
func (r *Receiver) WriteRTP(packet *rtp.Packet) {
	r.Input(packet)
}

// Deprecated: should be removed
func (r *Receiver) Senders() []*Sender {
	if len(r.childs) > 0 {
		return []*Sender{{}}
	} else {
		return nil
	}
}

// Deprecated: should be removed
func (r *Receiver) Replace(target *Receiver) {
	MoveNode(&target.Node, &r.Node)
	if r.ring != nil {
		// Move ring ownership to the replacement receiver.
		target.ring = r.ring
		r.ring = nil
		codecName := ""
		isRTP := false
		if target.Codec != nil {
			codecName = target.Codec.Name
			isRTP = target.Codec.IsRTP()
		}
		ring := target.ring
		target.Input = func(packet *Packet) {
			target.Bytes += len(packet.Payload)
			target.Packets++
			if isRTP {
				target.noteH264ParameterRTP(packet.Payload)
				target.appendRTPAU(codecName, packet)
				return
			}
			if codecName == CodecH264 {
				target.noteH264ParameterAVCC(packet.Payload)
			}
			idr := packetIsIDR(codecName, isRTP, packet.Payload)
			ring.Write(packet, idr, nil)
		}
	}
}

func (r *Receiver) Close() {
	if r.ring != nil {
		r.auMu.Lock()
		r.flushAULocked()
		r.auMu.Unlock()
		r.ring.Close()
	}
	r.Node.Close()
}

type Sender struct {
	Node

	// Deprecated:
	Media *Media `json:"-"`
	// Deprecated:
	Handler HandlerFunc `json:"-"`

	Bytes   int `json:"bytes,omitempty"`
	Packets int `json:"packets,omitempty"`
	Drops   int `json:"drops,omitempty"`

	// needKeyframe：channel 丢包后置位，出口应丢弃非关键帧直至下一 IDR，避免马赛克粘连。
	needKeyframe atomic.Bool

	buf  chan *Packet
	done chan struct{}

	// Video ring binding (H264/H265): pump from ring into buf instead of fan-out Input.
	recv       *Receiver
	ringCancel context.CancelFunc
}

func NewSender(media *Media, codec *Codec) *Sender {
	var bufSize uint16

	if GetKind(codec.Name) == KindVideo {
		if codec.IsRTP() {
			// in my tests 40Mbit/s 4K-video can generate up to 1500 items
			// for the h264.RTPDepay => RTPPay queue
			bufSize = 4096
		} else {
			// FLV/RTMP 等 RawPacket：单 tag 一帧，但同一 Receiver 上多 consumer（如 WebRTC + interval 采样）
			// 时 Handler 若偶发变慢，64 易丢包并长期等不到 IDR；略放大以换内存。
			bufSize = 2048
		}
	} else {
		bufSize = 128
	}

	buf := make(chan *Packet, bufSize)
	s := &Sender{
		Node:  Node{id: NewID(), Codec: codec},
		Media: media,
		buf:   buf,
	}
	// 新 Sender 默认等关键帧，避免中途入流从 P 帧起解。
	if GetKind(codec.Name) == KindVideo {
		s.needKeyframe.Store(true)
	}
	s.Input = func(packet *Packet) {
		s.mu.Lock()
		// unblock write to nil chan - OK, write to closed chan - panic
		select {
		case s.buf <- packet:
			s.Bytes += len(packet.Payload)
			s.Packets++
		default:
			s.Drops++
			s.needKeyframe.Store(true)
		}
		s.mu.Unlock()
	}
	s.Output = func(packet *Packet) {
		s.Handler(packet)
	}
	return s
}

// NeedKeyframe reports whether the sender should skip until the next IDR.
func (s *Sender) NeedKeyframe() bool {
	return s.needKeyframe.Load()
}

// ClearNeedKeyframe marks that a keyframe has been accepted after a wait/drop.
func (s *Sender) ClearNeedKeyframe() {
	s.needKeyframe.Store(false)
}

// Deprecated: should be removed
func (s *Sender) HandleRTP(parent *Receiver) {
	s.WithParent(parent)
	s.Start()
}

// Deprecated: should be removed
func (s *Sender) Bind(parent *Receiver) {
	s.WithParent(parent)
}

func (s *Sender) WithParent(parent *Receiver) *Sender {
	s.recv = parent
	s.Node.WithParent(&parent.Node)
	return s
}

func (s *Sender) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.buf == nil || s.done != nil {
		return
	}
	s.done = make(chan struct{})

	// Video ring path: pump history from IDR then follow live writes.
	if s.recv != nil && s.recv.ring != nil {
		ctx, cancel := context.WithCancel(context.Background())
		s.ringCancel = cancel
		// Keep needKeyframe until the first assembled IDR AU passes the consumer
		// filter (SPS/PPS inject alone is not a keyframe).
		s.needKeyframe.Store(true)
		codecName := ""
		if s.Codec != nil {
			codecName = s.Codec.Name
		}
		log.Info().
			Uint32("sender", s.id).
			Uint32("recv", s.recv.id).
			Str("codec", codecName).
			Msg("[avring] sender pump from ring (BindFromIDR)")
		go s.pumpRing(ctx, s.recv.ring)
	}

	// pass buf directly so that it's impossible for buf to be nil
	go func(buf chan *Packet) {
		for packet := range buf {
			s.Output(packet)
		}
		close(s.done)
	}(s.buf)
}

func (s *Sender) pumpRing(ctx context.Context, ring *AVRing) {
	rr := NewAVRingReader(ring)
	defer rr.Close()

	// Emit every RTP packet of the locked AU (SPS/PPS already prepended on IDR).
	emitAU := func() {
		pkts := rr.Value.MediaPackets()
		for i := range pkts {
			cp := pkts[i]
			s.Input(&cp)
		}
		rr.pktIdx = len(pkts)
	}

	seekPending := false
	for {
		if s.NeedKeyframe() {
			if !seekPending {
				if err := rr.SeekLatestIDR(ctx); err == nil {
					seekPending = true
					emitAU()
					continue // AU fully emitted; do not ReadFrame the same slot again
				}
			}
		} else {
			seekPending = false
		}
		pkt, err := rr.ReadFrame(ctx)
		if err != nil {
			return
		}
		s.Input(pkt)
	}
}

func (s *Sender) Wait() {
	if done := s.done; done != nil {
		<-done
	}
}

func (s *Sender) State() string {
	if s.buf == nil {
		return "closed"
	}
	if s.done == nil {
		return "new"
	}
	return "connected"
}

func (s *Sender) Close() {
	if s.ringCancel != nil {
		s.ringCancel()
		s.ringCancel = nil
	}

	// close buffer if exists
	s.mu.Lock()
	if s.buf != nil {
		close(s.buf) // exit from for range loop
		s.buf = nil  // prevent writing to closed chan
	}
	s.mu.Unlock()

	s.Node.Close()
}

func (r *Receiver) MarshalJSON() ([]byte, error) {
	v := struct {
		ID      uint32   `json:"id"`
		Codec   *Codec   `json:"codec"`
		Childs  []uint32 `json:"childs,omitempty"`
		Bytes   int      `json:"bytes,omitempty"`
		Packets int      `json:"packets,omitempty"`
		AVRing  bool     `json:"avring,omitempty"`
	}{
		ID:      r.Node.id,
		Codec:   r.Node.Codec,
		Bytes:   r.Bytes,
		Packets: r.Packets,
		AVRing:  r.ring != nil,
	}
	for _, child := range r.childs {
		v.Childs = append(v.Childs, child.id)
	}
	return json.Marshal(v)
}

func (s *Sender) MarshalJSON() ([]byte, error) {
	v := struct {
		ID      uint32 `json:"id"`
		Codec   *Codec `json:"codec"`
		Parent  uint32 `json:"parent,omitempty"`
		Bytes   int    `json:"bytes,omitempty"`
		Packets int    `json:"packets,omitempty"`
		Drops   int    `json:"drops,omitempty"`
	}{
		ID:      s.Node.id,
		Codec:   s.Node.Codec,
		Bytes:   s.Bytes,
		Packets: s.Packets,
		Drops:   s.Drops,
	}
	if s.parent != nil {
		v.Parent = s.parent.id
	}
	return json.Marshal(v)
}
