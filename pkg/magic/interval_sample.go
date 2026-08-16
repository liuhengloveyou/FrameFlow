package magic

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/liuhengloveyou/FrameFlow/pkg/core"
	"github.com/liuhengloveyou/FrameFlow/pkg/h264"
	"github.com/liuhengloveyou/FrameFlow/pkg/h264/annexb"
	"github.com/liuhengloveyou/FrameFlow/pkg/h265"
	"github.com/liuhengloveyou/FrameFlow/pkg/mjpeg"
	"github.com/pion/rtp"
)

// IntervalSampleConsumer 是通用视频流 consumer：可长期挂在任意流（AddConsumer）上，按 minInterval
// 节流输出可独立解码的 Annex-B 样本（自上一 IDR 起累积的完整片段，含后续 P/B）；
// MJPEG/RAW 为逐帧。适用于 vision 等周期性抓帧场景。
type IntervalSampleConsumer struct {
	core.Connection
	wr *core.WriteBuffer

	minInterval time.Duration
	maxGOPBytes int

	mu       sync.Mutex
	gop      []byte
	haveIDR  bool
	lastEmit time.Time

	sampleCh chan []byte

	lastRTP atomic.Uint32
	haveRTP atomic.Uint32
}

// NewIntervalSampleConsumer 构造间隔采样 consumer。minInterval 为相邻两次推送的最小时间间隔；
// maxGOPBytes 为 H264/H265 GOP 累积字节上限，超出则丢弃并等待下一 IDR，避免异常流占满内存。
func NewIntervalSampleConsumer(minInterval time.Duration, maxGOPBytes int) *IntervalSampleConsumer {
	if minInterval < 0 {
		minInterval = 0
	}
	if maxGOPBytes <= 0 {
		maxGOPBytes = 4 << 20
	}
	medias := []*core.Media{
		{
			Kind:      core.KindVideo,
			Direction: core.DirectionSendonly,
			Codecs: []*core.Codec{
				{Name: core.CodecJPEG},
				{Name: core.CodecRAW},
				{Name: core.CodecH264},
				{Name: core.CodecH265},
			},
		},
	}
	wr := core.NewWriteBuffer(nil)
	return &IntervalSampleConsumer{
		Connection: core.Connection{
			ID:         core.NewID(),
			FormatName: "interval_sample",
			Medias:     medias,
			Transport:  wr,
		},
		wr:          wr,
		minInterval: minInterval,
		maxGOPBytes: maxGOPBytes,
		sampleCh:    make(chan []byte, 1),
	}
}

func (s *IntervalSampleConsumer) pushSample(b []byte) {
	if len(b) == 0 {
		return
	}
	cp := append([]byte(nil), b...)
	select {
	case s.sampleCh <- cp:
	default:
		select {
		case <-s.sampleCh:
		default:
		}
		s.sampleCh <- append([]byte(nil), b...)
	}
}

func (s *IntervalSampleConsumer) tryEmit(ts uint32, b []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if !s.lastEmit.IsZero() && s.minInterval > 0 && now.Sub(s.lastEmit) < s.minInterval {
		return
	}
	s.lastEmit = now
	s.lastRTP.Store(ts)
	s.haveRTP.Store(1)
	s.pushSample(b)
}

func (s *IntervalSampleConsumer) onH264AU(packet *rtp.Packet, au []byte) {
	// IsKeyframe 仅适用于 AVCC（与 [Keyframe.AddTrack] 一致）；勿在 annexb.DecodeAVCC 之后对 Annex-B 调用，
	// 否则按 4 字节长度步进会错位，SPS+PPS+IDR 等 AU 无法识别 IDR，haveIDR 永远为 false。
	isIDR := h264.IsKeyframe(au)
	b := annexb.DecodeAVCC(au, true)
	if len(b) == 0 {
		return
	}

	s.mu.Lock()
	if isIDR {
		s.gop = append(s.gop[:0], b...)
		s.haveIDR = true
	} else if s.haveIDR {
		s.gop = append(s.gop, b...)
	} else {
		s.mu.Unlock()
		return
	}
	if len(s.gop) > s.maxGOPBytes {
		s.haveIDR = false
		s.gop = s.gop[:0]
		s.mu.Unlock()
		return
	}
	// 推送自 IDR 起的完整片段，供一次性解码（ffdecode 每次 feed 自包含）。
	snap := append([]byte(nil), s.gop...)
	s.mu.Unlock()

	s.tryEmit(packet.Timestamp, snap)
}

func (s *IntervalSampleConsumer) onH265AU(packet *rtp.Packet, au []byte) {
	isIDR := h265.IsKeyframe(au)
	b := annexb.DecodeAVCC(au, true)
	if len(b) == 0 {
		return
	}

	s.mu.Lock()
	if isIDR {
		s.gop = append(s.gop[:0], b...)
		s.haveIDR = true
	} else if s.haveIDR {
		s.gop = append(s.gop, b...)
	} else {
		s.mu.Unlock()
		return
	}
	if len(s.gop) > s.maxGOPBytes {
		s.haveIDR = false
		s.gop = s.gop[:0]
		s.mu.Unlock()
		return
	}
	snap := append([]byte(nil), s.gop...)
	s.mu.Unlock()

	s.tryEmit(packet.Timestamp, snap)
}

func (s *IntervalSampleConsumer) onJPEGFrame(packet *rtp.Packet, payload []byte) {
	if len(payload) == 0 {
		return
	}
	s.tryEmit(packet.Timestamp, payload)
}

// AddTrack 与 [NewVideoKeyframeConsumer] 相同媒体声明，按间隔推送可解码片段或 MJPEG 帧。
func (s *IntervalSampleConsumer) AddTrack(media *core.Media, _ *core.Codec, track *core.Receiver) error {
	sender := core.NewSender(media, track.Codec)

	switch track.Codec.Name {
	case core.CodecH264:
		sender.Handler = func(packet *rtp.Packet) {
			s.onH264AU(packet, packet.Payload)
		}
		if track.Codec.IsRTP() {
			sender.Handler = h264.RTPDepay(track.Codec, sender.Handler)
		} else {
			sender.Handler = h264.RepairAVCC(track.Codec, sender.Handler)
		}

	case core.CodecH265:
		sender.Handler = func(packet *rtp.Packet) {
			s.onH265AU(packet, packet.Payload)
		}
		if track.Codec.IsRTP() {
			sender.Handler = h265.RTPDepay(track.Codec, sender.Handler)
		}

	case core.CodecJPEG:
		sender.Handler = func(packet *rtp.Packet) {
			s.onJPEGFrame(packet, packet.Payload)
		}
		if track.Codec.IsRTP() {
			sender.Handler = mjpeg.RTPDepay(sender.Handler)
		}

	case core.CodecRAW:
		sender.Handler = func(packet *rtp.Packet) {
			s.onJPEGFrame(packet, packet.Payload)
		}
		sender.Handler = mjpeg.Encoder(track.Codec, 5, sender.Handler)
	}

	sender.HandleRTP(track)
	s.Senders = append(s.Senders, sender)
	return nil
}

func (s *IntervalSampleConsumer) CodecName() string {
	if len(s.Senders) != 1 {
		return ""
	}
	return s.Senders[0].Codec.Name
}

// WaitSample 阻塞直到 ctx 取消或收到下一份样本（可能因 minInterval 与上游帧率晚于预期）。
func (s *IntervalSampleConsumer) WaitSample(ctx context.Context) ([]byte, error) {
	select {
	case b := <-s.sampleCh:
		return b, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// LastRTP 返回最近一次推送样本对应的 RTP timestamp。
func (s *IntervalSampleConsumer) LastRTP() (ts uint32, ok bool) {
	if s.haveRTP.Load() == 0 {
		return 0, false
	}
	return s.lastRTP.Load(), true
}

// VideoIntervalConsumer 已重命名为 [IntervalSampleConsumer]；保留类型别名以免外部引用断裂。
//
// Deprecated: use [IntervalSampleConsumer].
type VideoIntervalConsumer = IntervalSampleConsumer

// NewVideoIntervalConsumer 已重命名为 [NewIntervalSampleConsumer]。
//
// Deprecated: use [NewIntervalSampleConsumer].
func NewVideoIntervalConsumer(minInterval time.Duration, maxGOPBytes int) *IntervalSampleConsumer {
	return NewIntervalSampleConsumer(minInterval, maxGOPBytes)
}
