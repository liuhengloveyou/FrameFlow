package rtmp

import (
	"io"

	"github.com/liuhengloveyou/FrameFlow/pkg/flv"
)

func (c *Conn) Producer() (*flv.Producer, error) {
	c.rdBuf = []byte{
		'F', 'L', 'V', // signature
		1,          // version
		0,          // flags (has video/audio)
		0, 0, 0, 9, // header size
	}

	prod, err := flv.Open(c)
	if err != nil {
		return nil, err
	}

	prod.FormatName = "rtmp"
	prod.Protocol = "rtmp"
	prod.RemoteAddr = c.conn.RemoteAddr().String()
	prod.URL = c.url

	return prod, nil
}

// Read - convert RTMP to FLV format
func (c *Conn) Read(p []byte) (n int, err error) {
	// 1. Check temporary tempbuffer
	if len(c.rdBuf) == 0 {
		for {
			msgType, timeMS, payload, err2 := c.readMessage()
			if err2 != nil {
				return 0, err2
			}

			// 协议控制消息不得打成 FLV tag；尤其 SetChunkSize 必须更新，否则大帧重组错乱。
			switch msgType {
			case TypeSetPacketSize:
				if len(payload) >= 4 {
					v := uint32(payload[0])<<24 | uint32(payload[1])<<16 | uint32(payload[2])<<8 | uint32(payload[3])
					if v > 0 {
						c.rdPacketSize = v
					}
				}
				continue
			case TypeServerBandwidth, TypeClientBandwidth:
				continue
			case TypeAudio, TypeVideo, TypeData:
				// ok
			case TypeCommand:
				// publish 后仍可能有 onStatus 等，忽略
				continue
			default:
				// UserControl(4) 等
				if msgType < TypeAudio {
					continue
				}
			}

			// previous tag size (4 byte) + header (11 byte) + payload
			n = 4 + 11 + len(payload)

			// 2. Check if the message fits in the buffer
			if n <= len(p) {
				encodeFLV(p, msgType, timeMS, payload)
				return
			}

			// 3. Put the message into a temporary buffer
			c.rdBuf = make([]byte, n)
			encodeFLV(c.rdBuf, msgType, timeMS, payload)
			break
		}
	}

	// 4. Send temporary buffer
	n = copy(p, c.rdBuf)
	c.rdBuf = c.rdBuf[n:]
	return
}

func encodeFLV(b []byte, msgType byte, time uint32, payload []byte) {
	_ = b[4+11]

	b[0] = 0
	b[1] = 0
	b[2] = 0
	b[3] = 0
	b[4+0] = msgType
	PutUint24(b[4+1:], uint32(len(payload)))
	PutUint24(b[4+4:], time)
	b[4+7] = byte(time >> 24)

	copy(b[4+11:], payload)
}

// Write - convert FLV format to RTMP format
func (c *Conn) Write(p []byte) (n int, err error) {
	n = len(p)

	if p[0] == 'F' {
		p = p[9+4:] // skip first msg with FLV header

		for len(p) > 0 {
			// FLV tag: 11 header + 24-bit payload size + 4 prevTagSize
			payloadSize := uint32(p[1])<<16 | uint32(p[2])<<8 | uint32(p[3])
			size := 11 + int(payloadSize) + 4
			if size > len(p) {
				return n, io.ErrUnexpectedEOF
			}
			if _, err = c.Write(p[:size]); err != nil {
				return 0, err
			}
			p = p[size:]
		}
		return
	}

	// decode FLV: 11 bytes header + payload + 4 byte size
	tagType := p[0]
	timeMS := uint32(p[4])<<16 | uint32(p[5])<<8 | uint32(p[6]) | uint32(p[7])<<24
	payload := p[11 : len(p)-4]

	err = c.writeMessage(4, tagType, timeMS, payload)
	return
}
