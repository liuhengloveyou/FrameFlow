package magic

import (
	"io"
	"sync/atomic"

	"github.com/liuhengloveyou/FrameFlow/pkg/core"
	"github.com/liuhengloveyou/FrameFlow/pkg/h264"
	"github.com/liuhengloveyou/FrameFlow/pkg/h264/annexb"
	"github.com/liuhengloveyou/FrameFlow/pkg/h265"
	"github.com/liuhengloveyou/FrameFlow/pkg/mjpeg"
	"github.com/pion/rtp"
)

type Keyframe struct {
	core.Connection
	wr *core.WriteBuffer
	// lastRTP / haveRTP：最近写入缓冲的关键帧对应 RTP 时间戳，供 vision 等与浏览器解码帧对齐。
	lastRTP atomic.Uint32
	haveRTP atomic.Uint32
}

// NewVideoKeyframeConsumer 构造短时 consumer：只把下一帧关键视频（H264/H265/MJPEG/RAW）
// 写入内部缓冲，供 WriteTo / OnceBuffer 等快照式读取。
func NewVideoKeyframeConsumer() *Keyframe {
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
	return &Keyframe{
		Connection: core.Connection{
			ID:         core.NewID(),
			FormatName: "keyframe",
			Medias:     medias,
			Transport:  wr,
		},
		wr: wr,
	}
}

// Deprecated: use [NewVideoKeyframeConsumer].
func NewKeyframe() *Keyframe {
	return NewVideoKeyframeConsumer()
}

func (k *Keyframe) AddTrack(media *core.Media, _ *core.Codec, track *core.Receiver) error {
	sender := core.NewSender(media, track.Codec)

	switch track.Codec.Name {
	case core.CodecH264:
		sender.Handler = func(packet *rtp.Packet) {
			if !h264.IsKeyframe(packet.Payload) {
				return
			}
			k.lastRTP.Store(packet.Timestamp)
			k.haveRTP.Store(1)
			b := annexb.DecodeAVCC(packet.Payload, true)
			if n, err := k.wr.Write(b); err == nil {
				k.Send += n
			}
		}

		if track.Codec.IsRTP() {
			sender.Handler = h264.RTPDepay(track.Codec, sender.Handler)
		} else {
			sender.Handler = h264.RepairAVCC(track.Codec, sender.Handler)
		}

	case core.CodecH265:
		sender.Handler = func(packet *rtp.Packet) {
			if !h265.IsKeyframe(packet.Payload) {
				return
			}
			k.lastRTP.Store(packet.Timestamp)
			k.haveRTP.Store(1)
			b := annexb.DecodeAVCC(packet.Payload, true)
			if n, err := k.wr.Write(b); err == nil {
				k.Send += n
			}
		}

		if track.Codec.IsRTP() {
			sender.Handler = h265.RTPDepay(track.Codec, sender.Handler)
		}

	case core.CodecJPEG:
		sender.Handler = func(packet *rtp.Packet) {
			k.lastRTP.Store(packet.Timestamp)
			k.haveRTP.Store(1)
			if n, err := k.wr.Write(packet.Payload); err == nil {
				k.Send += n
			}
		}

		if track.Codec.IsRTP() {
			sender.Handler = mjpeg.RTPDepay(sender.Handler)
		}

	case core.CodecRAW:
		sender.Handler = func(packet *rtp.Packet) {
			k.lastRTP.Store(packet.Timestamp)
			k.haveRTP.Store(1)
			if n, err := k.wr.Write(packet.Payload); err == nil {
				k.Send += n
			}
		}

		sender.Handler = mjpeg.Encoder(track.Codec, 5, sender.Handler)
	}

	sender.HandleRTP(track)
	k.Senders = append(k.Senders, sender)
	return nil
}

func (k *Keyframe) CodecName() string {
	if len(k.Senders) != 1 {
		return ""
	}
	return k.Senders[0].Codec.Name
}

func (k *Keyframe) WriteTo(wr io.Writer) (int64, error) {
	return k.wr.WriteTo(wr)
}

// LastRTP 返回最近关键帧对应的 RTP timestamp（与 WebRTC 入站视频同一时钟域）；ok=false 表示尚无或未走 RTP。
func (k *Keyframe) LastRTP() (ts uint32, ok bool) {
	if k.haveRTP.Load() == 0 {
		return 0, false
	}
	return k.lastRTP.Load(), true
}
