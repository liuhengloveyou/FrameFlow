package tapo

import (
	"bytes"
	"strconv"

	"github.com/liuhengloveyou/FrameFlow/pkg/core"
	"github.com/liuhengloveyou/FrameFlow/pkg/mpegts"
	"github.com/pion/rtp"
)

func (c *Client) AddTrack(media *core.Media, _ *core.Codec, track *core.Receiver) error {
	if c.sender == nil {
		if err := c.SetupBackchannel(); err != nil {
			return err
		}

		muxer := mpegts.NewMuxer()
		pid := muxer.AddTrack(mpegts.StreamTypePCMATapo)
		if err := c.WriteBackchannel(muxer.GetHeader()); err != nil {
			return err
		}

		c.sender = core.NewSender(media, track.Codec)
		c.sender.Handler = func(packet *rtp.Packet) {
			b := muxer.GetPayload(pid, packet.Timestamp, 0, packet.Payload)
			_ = c.WriteBackchannel(b)
		}
	}

	c.sender.WithParent(track).Start()
	return nil
}

func (c *Client) SetupBackchannel() (err error) {
	// if conn1 is not used - we will use it for backchannel
	// or we need to start another conn for session2
	if c.session1 != "" {
		if c.conn2, err = c.newConn(); err != nil {
			return
		}
	} else {
		c.conn2 = c.conn1
	}

	c.session2, err = c.Request(c.conn2, []byte(`{"params":{"talk":{"mode":"aec"},"method":"get"},"seq":3,"type":"request"}`))
	return
}

func (c *Client) WriteBackchannel(body []byte) (err error) {
	// multipart part：与 Request 一致，body 后补 \r\n；Content-Length 仅为 body 字节数。
	const (
		partBoundary = "----client-stream-boundary--\r\n"
		partType     = "Content-Type: audio/mp2t\r\n"
		partEncrypt  = "X-If-Encrypt: 0\r\n"
	)
	partSession := "X-Session-Id: " + c.session2 + "\r\n"
	partLen := "Content-Length: " + strconv.Itoa(len(body)) + "\r\n\r\n"
	n := len(partBoundary) + len(partType) + len(partEncrypt) + len(partSession) + len(partLen) + len(body) + 2
	buf := bytes.NewBuffer(make([]byte, 0, n))
	buf.WriteString(partBoundary)
	buf.WriteString(partType)
	buf.WriteString(partEncrypt)
	buf.WriteString(partSession)
	buf.WriteString(partLen)
	buf.Write(body)
	buf.WriteString("\r\n")

	_, err = buf.WriteTo(c.conn2)
	return
}
