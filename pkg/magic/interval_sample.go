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
// 节流输出样本。两种输出语义：
//
//   - 自包含（[NewIntervalSampleConsumer]）：每次推送「自上一 IDR 起累积的完整片段」，
//     接收端可对每份样本独立解码（解码器每份样本重置）。代价是每一次解码都要重放整个 GOP。
//   - 增量（[NewIncrementalSampleConsumer]）：只推送「上次推送之后新到的 AU」，接收端需
//     常驻解码器（ffdecode Options.Streaming）跨样本保持参考帧。丢包后需等下一个 IDR 才能
//     彻底清洁；成本 ∝ 节拍内新增帧数，适合周期性抓帧的实时链路。
//
// MJPEG/RAW 为逐帧，两种模式行为一致。
type IntervalSampleConsumer struct {
	core.Connection
	wr *core.WriteBuffer

	minInterval time.Duration
	maxGOPBytes int

	// incremental：增量推送模式（见类型注释）。
	incremental bool

	mu       sync.Mutex
	gop      []byte // 自包含模式：完整 GOP；增量模式：尚未成功送出的 AU
	emitted  int    // 增量模式：gop 中已成功送出（已写入 sampleCh）的字节数
	haveIDR  bool
	lastEmit time.Time

	sampleCh chan []byte

	lastRTP atomic.Uint32
	haveRTP atomic.Uint32
}

// NewIntervalSampleConsumer 构造间隔采样 consumer。minInterval 为相邻两次推送的最小时间间隔；
// maxGOPBytes 为 H264/H265 GOP 累积字节上限，超出则丢弃并等待下一 IDR，避免异常流占满内存。
func NewIntervalSampleConsumer(minInterval time.Duration, maxGOPBytes int) *IntervalSampleConsumer {
	return newIntervalSampleConsumer(minInterval, maxGOPBytes, false)
}

// NewIncrementalSampleConsumer 构造增量采样 consumer（见类型注释）。
// maxPendingBytes 为「尚未送出」的累积上限，超出则丢弃并等待下一 IDR。
// 接收端必须用持久解码器（ffdecode.Options{Streaming: true}）配合，否则参考帧缺失无法解码。
func NewIncrementalSampleConsumer(minInterval time.Duration, maxPendingBytes int) *IntervalSampleConsumer {
	return newIntervalSampleConsumer(minInterval, maxPendingBytes, true)
}

func newIntervalSampleConsumer(minInterval time.Duration, maxBytes int, incremental bool) *IntervalSampleConsumer {
	if minInterval < 0 {
		minInterval = 0
	}
	if maxBytes <= 0 {
		maxBytes = 4 << 20
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
		maxGOPBytes: maxBytes,
		incremental: incremental,
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

// tryEmit 按 minInterval 节流推送；返回是否真的写入了 sampleCh。
func (s *IntervalSampleConsumer) tryEmit(ts uint32, b []byte) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if !s.lastEmit.IsZero() && s.minInterval > 0 && now.Sub(s.lastEmit) < s.minInterval {
		return false
	}
	// 增量模式：上一份增量还没被取走，就绝不能覆盖它——那段码流会永远丢失，
	// 解码器出现参考帧缺口（花屏直到下一个 IDR）。宁可把窗口拉长，等消费端取走。
	if s.incremental && len(s.sampleCh) > 0 {
		return false
	}
	s.lastEmit = now
	s.lastRTP.Store(ts)
	s.haveRTP.Store(1)
	s.pushSample(b)
	return true
}

// RequestResync 丢弃未送出的增量与通道里压着的旧样本，改为等待下一个 IDR 作为新的自包含起点。
// 解码器出错重置后调用，可让链路重新对齐（之后自动恢复增量推送）。
func (s *IntervalSampleConsumer) RequestResync() {
	s.mu.Lock()
	defer s.mu.Unlock()
	// 与 pushSample 同在 s.mu 下，保证不会把 resync 之后新推的样本误丢。
	select {
	case <-s.sampleCh:
	default:
	}
	s.haveIDR = false
	s.gop = s.gop[:0]
	s.emitted = 0
}

// consumeNALU 累积一个访问单元并尝试推送（H264/H265 共用）。
func (s *IntervalSampleConsumer) consumeNALU(ts uint32, isIDR bool, b []byte) {
	s.mu.Lock()
	if isIDR {
		// 新 IDR：自包含起点（含参数集）。此前的未送数据已无参考价值，直接丢弃。
		s.gop = append(s.gop[:0], b...)
		s.emitted = 0
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
		s.emitted = 0
		s.mu.Unlock()
		return
	}

	var snap []byte
	if s.incremental {
		if len(s.gop) <= s.emitted {
			s.mu.Unlock()
			return
		}
		snap = append([]byte(nil), s.gop[s.emitted:]...)
	} else {
		// 推送自 IDR 起的完整片段，供一次性解码（ffdecode 每次 feed 自包含）。
		snap = append([]byte(nil), s.gop...)
	}
	s.mu.Unlock()

	if !s.tryEmit(ts, snap) || !s.incremental {
		return
	}
	// 只有真的送出去了才推进已送偏移，并把已送前缀从缓冲里摘掉（容量保留）。
	s.mu.Lock()
	s.emitted += len(snap)
	if s.emitted >= len(s.gop) {
		s.gop = s.gop[:0]
		s.emitted = 0
	} else {
		s.gop = append(s.gop[:0], s.gop[s.emitted:]...)
		s.emitted = 0
	}
	s.mu.Unlock()
}

func (s *IntervalSampleConsumer) onH264AU(packet *rtp.Packet, au []byte) {
	// IsKeyframe 仅适用于 AVCC（与 [Keyframe.AddTrack] 一致）；勿在 annexb.DecodeAVCC 之后对 Annex-B 调用，
	// 否则按 4 字节长度步进会错位，SPS+PPS+IDR 等 AU 无法识别 IDR，haveIDR 永远为 false。
	isIDR := h264.IsKeyframe(au)
	b := annexb.DecodeAVCC(au, true)
	if len(b) == 0 {
		return
	}
	s.consumeNALU(packet.Timestamp, isIDR, b)
}

func (s *IntervalSampleConsumer) onH265AU(packet *rtp.Packet, au []byte) {
	isIDR := h265.IsKeyframe(au)
	b := annexb.DecodeAVCC(au, true)
	if len(b) == 0 {
		return
	}
	s.consumeNALU(packet.Timestamp, isIDR, b)
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
