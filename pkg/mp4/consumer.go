package mp4

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/liuhengloveyou/FrameFlow/pkg/aac"
	"github.com/liuhengloveyou/FrameFlow/pkg/core"
	"github.com/liuhengloveyou/FrameFlow/pkg/h264"
	"github.com/liuhengloveyou/FrameFlow/pkg/h265"
	"github.com/pion/rtp"
)

type Consumer struct {
	core.Connection
	wr    *core.WriteBuffer
	muxer *Muxer
	mu    sync.Mutex
	start bool

	Rotate int `json:"-"`
	ScaleX int `json:"-"`
	ScaleY int `json:"-"`
}

func NewConsumer(medias []*core.Media) *Consumer {
	if medias == nil {
		// default local medias
		medias = []*core.Media{
			{
				Kind:      core.KindVideo,
				Direction: core.DirectionSendonly,
				Codecs: []*core.Codec{
					{Name: core.CodecH264},
					{Name: core.CodecH265},
				},
			},
			{
				Kind:      core.KindAudio,
				Direction: core.DirectionSendonly,
				Codecs: []*core.Codec{
					{Name: core.CodecAAC},
				},
			},
		}
	}

	wr := core.NewWriteBuffer(nil)
	return &Consumer{
		Connection: core.Connection{
			ID:         core.NewID(),
			FormatName: "mp4",
			Medias:     medias,
			Transport:  wr,
		},
		muxer: &Muxer{},
		wr:    wr,
	}
}

func (c *Consumer) AddTrack(media *core.Media, _ *core.Codec, track *core.Receiver) error {
	trackID := byte(len(c.Senders))

	codec := track.Codec.Clone()
	handler := core.NewSender(media, codec)

	switch track.Codec.Name {
	case core.CodecH264:
		handler.Handler = func(packet *rtp.Packet) {
			if !c.start {
				if !h264.IsKeyframe(packet.Payload) {
					return
				}
				c.start = true
			}

			// important to use Mutex because right fragment order
			c.mu.Lock()
			b := c.muxer.GetPayload(trackID, packet)
			if n, err := c.wr.Write(b); err == nil {
				c.Send += n
			}
			c.mu.Unlock()
		}

		if track.Codec.IsRTP() {
			handler.Handler = h264.RTPDepay(track.Codec, handler.Handler)
		} else {
			handler.Handler = h264.RepairAVCC(track.Codec, handler.Handler)
		}

	case core.CodecH265:
		handler.Handler = func(packet *rtp.Packet) {
			if !c.start {
				if !h265.IsKeyframe(packet.Payload) {
					return
				}
				c.start = true
			}

			// important to use Mutex because right fragment order
			c.mu.Lock()
			b := c.muxer.GetPayload(trackID, packet)
			if n, err := c.wr.Write(b); err == nil {
				c.Send += n
			}
			c.mu.Unlock()
		}

		if track.Codec.IsRTP() {
			handler.Handler = h265.RTPDepay(track.Codec, handler.Handler)
		} else {
			handler.Handler = h265.RepairAVCC(track.Codec, handler.Handler)
		}

	default:
		handler.Handler = func(packet *rtp.Packet) {
			if !c.start {
				return
			}

			// important to use Mutex because right fragment order
			c.mu.Lock()
			b := c.muxer.GetPayload(trackID, packet)
			if n, err := c.wr.Write(b); err == nil {
				c.Send += n
			}
			c.mu.Unlock()
		}

		switch track.Codec.Name {
		case core.CodecAAC:
			// 浏览器 MSE 对直播 fMP4 通常只稳支持 mono/stereo AAC；
			// 多声道、或 ASC channelConfiguration=0（PCE，常见于 5.1）会导致
			// CHUNK_DEMUXER_ERROR_APPEND_FAILED。跳过音轨，保留视频。
			ch := track.Codec.Channels
			confHex := core.Between(track.Codec.FmtpLine, "config=", ";")
			confLen := 0
			if confHex != "" {
				if b, err := hex.DecodeString(confHex); err == nil && len(b) > 0 {
					confLen = len(b)
					parsed := aac.ConfigToCodec(b)
					log.Info().
						Str("config", confHex).
						Int("config_len", confLen).
						Uint8("codec_ch", track.Codec.Channels).
						Uint8("asc_ch", parsed.Channels).
						Uint32("rate", parsed.ClockRate).
						Msg("[mp4] aac track probe")
					if parsed.ClockRate > 0 {
						codec.ClockRate = parsed.ClockRate
					}
					if parsed.Channels > 0 {
						ch = parsed.Channels
						codec.Channels = ch
					} else if confLen > 2 {
						// ASC 声道=0 且带 PCE → 多声道隐式布局，MSE 不支持
						ch = 255
					}
				} else {
					_, decErr := hex.DecodeString(confHex)
					log.Warn().Str("config", confHex).Err(decErr).Msg("[mp4] aac config decode failed")
				}
			} else {
				log.Info().
					Uint8("channels", ch).
					Uint32("rate", track.Codec.ClockRate).
					Str("fmtp", track.Codec.FmtpLine).
					Msg("[mp4] aac without config=")
			}
			if ch == 0 || ch > 2 {
				err := fmt.Errorf("mp4: AAC %dch unsupported for MSE (mono/stereo only; config_len=%d)", ch, confLen)
				log.Warn().Err(err).Msg("[mp4] skip aac track")
				return err
			}
			if track.Codec.IsRTP() {
				handler.Handler = aac.RTPDepay(handler.Handler)
			}
		case core.CodecOpus, core.CodecMP3: // no changes
		case core.CodecPCMA, core.CodecPCMU, core.CodecPCM, core.CodecPCML:
			// 直播 MSE 对 fMP4+FLAC 支持差（易 MEDIA_ERR_DECODE）；跳过音轨保视频。
			err := fmt.Errorf("mp4: PCM/FLAC unsupported for MSE live")
			log.Warn().Err(err).Str("codec", track.Codec.Name).Msg("[mp4] skip pcm track")
			return err

		default:
			handler.Handler = nil
		}
	}

	if handler.Handler == nil {
		s := "mp4: unsupported codec: " + track.Codec.String()
		println(s)
		return errors.New(s)
	}

	c.muxer.AddTrack(codec)
	log.Info().
		Str("codec", codec.String()).
		Uint8("track_id", trackID).
		Int("senders", len(c.Senders)+1).
		Msg("[mp4] add track")

	handler.HandleRTP(track)
	c.Senders = append(c.Senders, handler)

	return nil
}

func (c *Consumer) WriteTo(wr io.Writer) (int64, error) {
	if len(c.Senders) == 1 && c.Senders[0].Codec.IsAudio() {
		c.start = true
	}

	init, err := c.muxer.GetInit()
	if err != nil {
		return 0, err
	}

	if c.Rotate != 0 {
		PatchVideoRotate(init, c.Rotate)
	}
	if c.ScaleX != 0 && c.ScaleY != 0 {
		PatchVideoScale(init, c.ScaleX, c.ScaleY)
	}

	if _, err = wr.Write(init); err != nil {
		return 0, err
	}

	return c.wr.WriteTo(wr)
}
