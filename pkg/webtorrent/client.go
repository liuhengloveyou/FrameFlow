package webtorrent

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/liuhengloveyou/FrameFlow/pkg/core"
	"github.com/liuhengloveyou/FrameFlow/pkg/webrtc"
	pion "github.com/pion/webrtc/v4"
)

func NewClient(tracker, share, pwd string, pc *pion.PeerConnection) (*webrtc.Conn, error) {
	// 1. Create WebRTC producer
	prod := webrtc.NewConn(pc)
	prod.FormatName = "webtorrent"
	prod.Mode = core.ModeActiveProducer
	prod.Protocol = "ws"

	medias := []*core.Media{
		{Kind: core.KindVideo, Direction: core.DirectionRecvonly},
		{Kind: core.KindAudio, Direction: core.DirectionRecvonly},
	}

	// 2. Create offer
	offer, err := prod.CreateCompleteOffer(medias)
	if err != nil {
		return nil, err
	}

	// 3. Encrypt offer
	nonce := strconv.FormatInt(time.Now().UnixNano(), 36)

	cipher, err := NewCipher(share, pwd, nonce)
	if err != nil {
		return nil, err
	}

	enc := cipher.Encrypt([]byte(offer))

	// 4. Connect to Tracker
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ws, _, err := websocket.Dial(ctx, tracker, nil)
	if err != nil {
		return nil, err
	}
	defer ws.CloseNow()

	// 5. Send offer
	msg := fmt.Sprintf(
		`{"action":"announce","info_hash":"%s","peer_id":"%s","offers":[{"offer_id":"%s","offer":{"type":"offer","sdp":"%s"}}],"numwant":1}`,
		InfoHash(share), core.RandString(16, 36), nonce, base64.StdEncoding.EncodeToString(enc),
	)
	if err = ws.Write(ctx, websocket.MessageText, []byte(msg)); err != nil {
		return nil, err
	}

	for {
		// 6. Read answer (ctx already has 30s deadline)
		var v Message
		if err = wsjson.Read(ctx, ws, &v); err != nil {
			return nil, err
		}

		if v.Answer == nil {
			continue
		}

		// 7. Decrypt answer
		enc, err = base64.StdEncoding.DecodeString(v.Answer.SDP)
		if err != nil {
			return nil, err
		}

		answer, err := cipher.Decrypt(enc)
		if err != nil {
			return nil, err
		}

		// 8. Set answer
		if err = prod.SetAnswer(string(answer)); err != nil {
			return nil, err
		}

		return prod, nil
	}
}
