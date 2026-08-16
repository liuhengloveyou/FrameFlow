package flv

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"sync/atomic"
	"time"

	"github.com/liuhengloveyou/FrameFlow/pkg/aac"
	"github.com/liuhengloveyou/FrameFlow/pkg/core"
	"github.com/liuhengloveyou/FrameFlow/pkg/h264"
	"github.com/liuhengloveyou/FrameFlow/pkg/h265"
	"github.com/pion/rtp"
)

type Producer struct {
	core.Connection
	rd *core.ReadBuffer

	// RTMP 推流时 Start() 先于消费者挂轨运行；须原子读写，否则 Apple Silicon 上等平台
	// 可能一直看不到 GetTrack 写入的 receiver，表现为推流成功但播放/预览无画面。
	video, audio atomic.Pointer[core.Receiver]
}

func Open(rd io.Reader) (*Producer, error) {
	prod := &Producer{
		Connection: core.Connection{
			ID:         core.NewID(),
			FormatName: "flv",
			Transport:  rd,
		},
		rd: core.NewReadBuffer(rd),
	}
	if err := prod.probe(); err != nil {
		return nil, err
	}
	return prod, nil
}

const (
	Signature = "FLV"

	TagAudio = 8
	TagVideo = 9
	TagData  = 18

	CodecAAC = 10

	CodecH264 = 7
	CodecHEVC = 12
)

const (
	PacketTypeAVCHeader = iota
	PacketTypeAVCNALU
	PacketTypeAVCEnd
)

const (
	PacketTypeSequenceStart = iota
	PacketTypeCodedFrames
	PacketTypeSequenceEnd
	PacketTypeCodedFramesX
	PacketTypeMetadata
	PacketTypeMPEG2TSSequenceStart
)

func (c *Producer) GetTrack(media *core.Media, codec *core.Codec) (*core.Receiver, error) {
	receiver, _ := c.Connection.GetTrack(media, codec)
	if media.Kind == core.KindVideo {
		c.video.Store(receiver)
	} else {
		c.audio.Store(receiver)
	}
	return receiver, nil
}

func (c *Producer) Start() error {
	for {
		pkt, err := c.readPacket()
		if err != nil {
			return err
		}

		c.Recv += len(pkt.Payload)

		switch pkt.PayloadType {
		case TagAudio:
			audio := c.audio.Load()
			if audio == nil || pkt.Payload[1] == 0 {
				continue
			}

			pkt.Timestamp = TimeToRTP(pkt.Timestamp, audio.Codec.ClockRate)
			pkt.Payload = pkt.Payload[2:]
			audio.WriteRTP(pkt)

		case TagVideo:
			video := c.video.Load()
			if video == nil {
				continue
			}

			var ctsMS int32
			if isExHeader(pkt.Payload) {
				switch packetType := pkt.Payload[0] & 0b1111; packetType {
				case PacketTypeCodedFrames:
					// frame type 4b, packet type 4b, fourCC 32b, composition time 24b
					if len(pkt.Payload) < 8 {
						continue
					}
					ctsMS = readSI24(pkt.Payload[5:8])
					pkt.Payload = pkt.Payload[8:]
				case PacketTypeCodedFramesX:
					// frame type 4b, packet type 4b, fourCC 32b
					if len(pkt.Payload) < 5 {
						continue
					}
					pkt.Payload = pkt.Payload[5:]
				default:
					continue
				}
			} else {
				switch pkt.Payload[1] {
				case PacketTypeAVCNALU:
					// frame type 4b, codecID 4b, avc packet type 8b, composition time 24b
					if len(pkt.Payload) < 5 {
						continue
					}
					ctsMS = readSI24(pkt.Payload[2:5])
					pkt.Payload = pkt.Payload[5:]
				default:
					continue
				}
			}

			// Tag timestamp = DTS；composition time = PTS-DTS（毫秒）。
			// MSE fMP4 经 ExtensionProfile 写 sample CTS（与 mpegts demuxer 约定一致）。
			clock := video.Codec.ClockRate
			pkt.Timestamp = TimeToRTP(pkt.Timestamp, clock)
			if ctsMS != 0 {
				pkt.ExtensionProfile = compositionToRTP(ctsMS, clock)
			}
			video.WriteRTP(pkt)
		}
	}
}

// readSI24 解析 FLV / Enhanced-RTMP 中的有符号 24 位 composition time（毫秒）。
func readSI24(b []byte) int32 {
	v := int32(b[0])<<16 | int32(b[1])<<8 | int32(b[2])
	if v&0x800000 != 0 {
		v |= ^0xFFFFFF
	}
	return v
}

// compositionToRTP 将毫秒 CTS 转到 RTP 时钟域，写入 ExtensionProfile（uint16，与 mpegts 路径一致）。
func compositionToRTP(ctsMS int32, clockRate uint32) uint16 {
	if clockRate == 0 {
		return 0
	}
	cts := int64(ctsMS) * int64(clockRate) / 1000
	if cts <= 0 {
		return 0
	}
	if cts > 0xFFFF {
		return 0xFFFF
	}
	return uint16(cts)
}

func (c *Producer) probe() error {
	if err := c.readHeader(); err != nil {
		return err
	}

	c.rd.BufferSize = core.ProbeSize
	defer c.rd.Reset()

	// Normal software sends:
	// 1. Video/audio flag in header
	// 2. MetaData as first tag (with video/audio codec info)
	// 3. Video/audio headers in 2nd and 3rd tag

	// Reolink camera sends:
	// 1. Empty video/audio flag
	// 2. MedaData without stereo key for AAC
	// 3. Audio header after Video keyframe tag

	// OpenIPC camera (on old firmwares) sends:
	// 1. Empty video/audio flag
	// 2. No MetaData packet
	// 3. Sends a video packet in more than 3 seconds
	waitVideo := true
	waitAudio := true
	timeout := time.Now().Add(time.Second * 5)

	for (waitVideo || waitAudio) && time.Now().Before(timeout) {
		pkt, err := c.readPacket()
		if err != nil {
			return err
		}

		//log.Printf("%d %0.20s", pkt.PayloadType, pkt.Payload)

		switch pkt.PayloadType {
		case TagAudio:
			if !waitAudio {
				continue
			}

			_ = pkt.Payload[1] // bounds

			codecID := pkt.Payload[0] >> 4 // SoundFormat
			_ = pkt.Payload[0] & 0b1100    // SoundRate
			_ = pkt.Payload[0] & 0b0010    // SoundSize
			_ = pkt.Payload[0] & 0b0001    // SoundType

			if codecID != CodecAAC {
				continue
			}

			if pkt.Payload[1] != 0 { // check if header
				continue
			}

			codec := aac.ConfigToCodec(pkt.Payload[2:])
			media := &core.Media{
				Kind:      core.KindAudio,
				Direction: core.DirectionRecvonly,
				Codecs:    []*core.Codec{codec},
			}
			c.Medias = append(c.Medias, media)
			waitAudio = false

		case TagVideo:
			if !waitVideo {
				continue
			}

			var codec *core.Codec

			if isExHeader(pkt.Payload) {
				if string(pkt.Payload[1:5]) != "hvc1" {
					continue
				}

				if packetType := pkt.Payload[0] & 0b1111; packetType != PacketTypeSequenceStart {
					continue
				}

				codec = h265.ConfigToCodec(pkt.Payload[5:])
			} else {
				_ = pkt.Payload[0] >> 4 // FrameType

				if packetType := pkt.Payload[1]; packetType != PacketTypeAVCHeader { // check if header
					continue
				}

				switch codecID := pkt.Payload[0] & 0b1111; codecID {
				case CodecH264:
					codec = h264.ConfigToCodec(pkt.Payload[5:])
				case CodecHEVC:
					codec = h265.ConfigToCodec(pkt.Payload[5:])
				default:
					continue
				}
			}

			media := &core.Media{
				Kind:      core.KindVideo,
				Direction: core.DirectionRecvonly,
				Codecs:    []*core.Codec{codec},
			}
			c.Medias = append(c.Medias, media)
			waitVideo = false

		case TagData:
			if !bytes.Contains(pkt.Payload, []byte("onMetaData")) {
				continue
			}
			// Dahua cameras doesn't send videocodecid
			if !bytes.Contains(pkt.Payload, []byte("videocodecid")) &&
				!bytes.Contains(pkt.Payload, []byte("width")) &&
				!bytes.Contains(pkt.Payload, []byte("framerate")) {
				waitVideo = false
			}
			if !bytes.Contains(pkt.Payload, []byte("audiocodecid")) {
				waitAudio = false
			}
		}
	}

	return nil
}

func (c *Producer) readHeader() error {
	b := make([]byte, 9)
	if _, err := io.ReadFull(c.rd, b); err != nil {
		return err
	}

	if string(b[:3]) != Signature {
		return errors.New("flv: wrong header")
	}

	_ = b[4] // flags (skip because unsupported by Reolink cameras)

	if skip := binary.BigEndian.Uint32(b[5:]) - 9; skip > 0 {
		if _, err := io.ReadFull(c.rd, make([]byte, skip)); err != nil {
			return err
		}
	}

	return nil
}

func (c *Producer) readPacket() (*rtp.Packet, error) {
	// https://rtmp.veriskope.com/pdf/video_file_format_spec_v10.pdf
	b := make([]byte, 4+11)
	if _, err := io.ReadFull(c.rd, b); err != nil {
		return nil, err
	}

	b = b[4 : 4+11] // skip previous tag size

	size := uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])

	pkt := &rtp.Packet{
		Header: rtp.Header{
			PayloadType: b[0],
			Timestamp:   uint32(b[4])<<16 | uint32(b[5])<<8 | uint32(b[6]) | uint32(b[7])<<24,
		},
		Payload: make([]byte, size),
	}

	if _, err := io.ReadFull(c.rd, pkt.Payload); err != nil {
		return nil, err
	}

	//log.Printf("[FLV] %d %.40x", pkt.PayloadType, pkt.Payload)

	return pkt, nil
}

// TimeToRTP convert time in milliseconds to RTP time
func TimeToRTP(timeMS, clockRate uint32) uint32 {
	// for clockRates 90000, 16000, 8000, etc. - we can use:
	//     return timeMS * (clockRate / 1000)
	// but for clockRates 44100, 22050, 11025 - we should use:
	return uint32(uint64(timeMS) * uint64(clockRate) / 1000)
}

func isExHeader(data []byte) bool {
	return data[0]&0b1000_0000 != 0
}
