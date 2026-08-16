package yandex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
	"github.com/liuhengloveyou/FrameFlow/internal/webrtc"
	"github.com/liuhengloveyou/FrameFlow/pkg/core"
	xwebrtc "github.com/liuhengloveyou/FrameFlow/pkg/webrtc"
	pion "github.com/pion/webrtc/v4"
)

func goloomClient(serviceURL, serviceName, roomId, participantId, credentials string) (core.Producer, error) {
	dctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	conn, _, err := websocket.Dial(dctx, serviceURL, nil)
	cancel()
	if err != nil {
		return nil, err
	}
	defer func() {
		time.Sleep(time.Second)
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}()

	bg := context.Background()

	s := fmt.Sprintf(`{"hello": {
"credentials":"%s","participantId":"%s","roomId":"%s","serviceName":"%s","sdkInitializationId":"%s",
"capabilitiesOffer":{},"sendAudio":false,"sendSharing":false,"sendVideo":false,
"sdkInfo":{"hwConcurrency":4,"implementation":"browser","version":"5.4.0"},
"participantAttributes":{"description":"","name":"mike","role":"SPEAKER"},
"participantMeta":{"description":"","name":"mike","role":"SPEAKER","sendAudio":false,"sendVideo":false}
},"uid":"%s"}`,
		credentials, participantId, roomId, serviceName,
		uuid.NewString(), uuid.NewString(),
	)

	err = conn.Write(bg, websocket.MessageText, []byte(s))
	if err != nil {
		return nil, err
	}

	if _, _, err = conn.Read(bg); err != nil {
		return nil, err
	}

	pc, err := webrtc.PeerConnection(true)
	if err != nil {
		return nil, err
	}

	prod := xwebrtc.NewConn(pc)
	prod.FormatName = "yandex"
	prod.Mode = core.ModeActiveProducer
	prod.Protocol = "wss"

	var connState core.Waiter

	prod.Listen(func(msg any) {
		switch msg := msg.(type) {
		case pion.PeerConnectionState:
			switch msg {
			case pion.PeerConnectionStateConnecting:
			case pion.PeerConnectionStateConnected:
				connState.Done(nil)
			default:
				connState.Done(errors.New("webrtc: " + msg.String()))
			}
		}
	})

	go func() {
		for {
			var msg map[string]json.RawMessage
			if err = wsjson.Read(bg, conn, &msg); err != nil {
				return
			}

			for k, v := range msg {
				switch k {
				case "uid":
					continue
				case "serverHello":
				case "subscriberSdpOffer":
					var sdp subscriberSdp
					if err = json.Unmarshal(v, &sdp); err != nil {
						return
					}
					if err = prod.SetOffer(sdp.Sdp); err != nil {
						return
					}
					if sdp.Sdp, err = prod.GetAnswer(); err != nil {
						return
					}

					var raw []byte
					if raw, err = json.Marshal(sdp); err != nil {
						return
					}
					s = fmt.Sprintf(`{"uid":"%s","subscriberSdpAnswer":%s}`, uuid.NewString(), raw)
					if err = conn.Write(bg, websocket.MessageText, []byte(s)); err != nil {
						return
					}
				case "webrtcIceCandidate":
					var candidate webrtcIceCandidate
					if err = json.Unmarshal(v, &candidate); err != nil {
						return
					}
					if err = prod.AddCandidate(candidate.Candidate); err != nil {
						return
					}
				}
			}

			if msg["ack"] != nil {
				continue
			}

			s = fmt.Sprintf(`{"uid":%s,"ack":{"status":{"code":"OK"}}}`, msg["uid"])
			if err = conn.Write(bg, websocket.MessageText, []byte(s)); err != nil {
				return
			}
		}
	}()

	if err = connState.Wait(); err != nil {
		return nil, err
	}

	s = fmt.Sprintf(`{"uid":"%s","setSlots":{"slots":[{"width":0,"height":0}],"audioSlotsCount":0,"key":1,"shutdownAllVideo":false,"withSelfView":false,"selfViewVisibility":"ON_LOADING_THEN_HIDE","gridConfig":{}}}`, uuid.NewString())
	if err = conn.Write(bg, websocket.MessageText, []byte(s)); err != nil {
		return nil, err
	}

	return prod, nil
}

type subscriberSdp struct {
	PcSeq int    `json:"pcSeq"`
	Sdp   string `json:"sdp"`
}

type webrtcIceCandidate struct {
	PcSeq         int    `json:"pcSeq"`
	Target        string `json:"target"`
	Candidate     string `json:"candidate"`
	SdpMid        string `json:"sdpMid"`
	SdpMlineIndex int    `json:"sdpMlineIndex"`
}
