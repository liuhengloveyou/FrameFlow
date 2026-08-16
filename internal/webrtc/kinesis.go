package webrtc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/liuhengloveyou/FrameFlow/pkg/core"
	"github.com/liuhengloveyou/FrameFlow/pkg/webrtc"
	pion "github.com/pion/webrtc/v4"
)

type kinesisRequest struct {
	Action   string `json:"action"`
	ClientID string `json:"recipientClientId"`
	Payload  []byte `json:"messagePayload"`
}

func (k kinesisRequest) String() string {
	return fmt.Sprintf("action=%s, payload=%s", k.Action, k.Payload)
}

type kinesisResponse struct {
	Payload []byte `json:"messagePayload"`
	Type    string `json:"messageType"`
}

func (k kinesisResponse) String() string {
	return fmt.Sprintf("type=%s, payload=%s", k.Type, k.Payload)
}

func kinesisClient(
	rawURL string, query url.Values, format string,
	sdpOffer func(prod *webrtc.Conn, query url.Values) (any, error),
) (core.Producer, error) {
	dctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	conn, _, err := websocket.Dial(dctx, rawURL, nil)
	cancel()
	if err != nil {
		return nil, err
	}

	conf := pion.Configuration{}

	if s := query.Get("ice_servers"); s != "" {
		conf.ICEServers, err = webrtc.UnmarshalICEServers([]byte(s))
		if err != nil {
			log.Warn().Err(err).Caller().Send()
		}
	}

	defer conn.CloseNow()

	api, err := webrtc.NewAPI()
	if err != nil {
		return nil, err
	}

	pc, err := api.NewPeerConnection(conf)
	if err != nil {
		return nil, err
	}

	var sendOffer core.Waiter
	defer sendOffer.Done(nil)

	var connState core.Waiter
	bg := context.Background()

	req := kinesisRequest{
		ClientID: query.Get("client_id"),
	}

	prod := webrtc.NewConn(pc)
	prod.FormatName = format
	prod.Mode = core.ModeActiveProducer
	prod.Protocol = "ws"
	prod.URL = rawURL
	prod.Listen(func(msg any) {
		switch msg := msg.(type) {
		case *pion.ICECandidate:
			_ = sendOffer.Wait()

			req.Action = "ICE_CANDIDATE"
			req.Payload, _ = json.Marshal(msg.ToJSON())
			if err = wsjson.Write(bg, conn, &req); err != nil {
				connState.Done(err)
				return
			}

			log.Trace().Msgf("[webrtc] kinesis send: %s", req)

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

	var payload any

	if sdpOffer == nil {
		medias := []*core.Media{
			{Kind: core.KindVideo, Direction: core.DirectionRecvonly},
			{Kind: core.KindAudio, Direction: core.DirectionRecvonly},
		}

		var offer string
		if offer, err = prod.CreateOffer(medias); err != nil {
			return nil, err
		}

		payload = pion.SessionDescription{
			Type: pion.SDPTypeOffer,
			SDP:  offer,
		}
	} else {
		if payload, err = sdpOffer(prod, query); err != nil {
			return nil, err
		}
	}

	req.Action = "SDP_OFFER"
	req.Payload, _ = json.Marshal(payload)
	if err = wsjson.Write(bg, conn, req); err != nil {
		return nil, err
	}

	log.Trace().Msgf("[webrtc] kinesis send: %s", req)

	sendOffer.Done(nil)

	go func() {
		var err error

		for {
			var res kinesisResponse
			if err = wsjson.Read(bg, conn, &res); err != nil {
				if errors.Is(err, io.ErrUnexpectedEOF) {
					continue
				}
				break
			}

			log.Trace().Msgf("[webrtc] kinesis recv: %s", res)

			switch res.Type {
			case "SDP_ANSWER":
				var sd pion.SessionDescription
				if err = json.Unmarshal(res.Payload, &sd); err != nil {
					break
				}

				if err = prod.SetAnswer(sd.SDP); err != nil {
					break
				}

			case "ICE_CANDIDATE":
				var ci pion.ICECandidateInit
				if err = json.Unmarshal(res.Payload, &ci); err != nil {
					break
				}

				if err = prod.AddCandidate(ci.Candidate); err != nil {
					break
				}
			}
		}

		connState.Done(err)
	}()

	if err = connState.Wait(); err != nil {
		return nil, err
	}

	return prod, nil
}

type wyzeKVS struct {
	ClientId string          `json:"ClientId"`
	Cam      string          `json:"cam"`
	Result   string          `json:"result"`
	Servers  json.RawMessage `json:"servers"`
	URL      string          `json:"signalingUrl"`
}

func wyzeClient(rawURL string) (core.Producer, error) {
	client := http.Client{Timeout: 5 * time.Second}
	res, err := client.Get(rawURL)
	if err != nil {
		return nil, err
	}

	b, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var kvs wyzeKVS
	if err = json.Unmarshal(b, &kvs); err != nil {
		return nil, err
	}

	if kvs.Result != "ok" {
		return nil, errors.New("wyse: wrong result: " + kvs.Result)
	}

	query := url.Values{
		"client_id":   []string{kvs.ClientId},
		"ice_servers": []string{string(kvs.Servers)},
	}

	return kinesisClient(kvs.URL, query, "webrtc/wyze", nil)
}
