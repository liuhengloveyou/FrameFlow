package webrtc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/liuhengloveyou/FrameFlow/pkg/core"
	"github.com/liuhengloveyou/FrameFlow/pkg/webrtc"
	pion "github.com/pion/webrtc/v4"
)

func openIPCClient(rawURL string, query url.Values) (core.Producer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	conn, _, err := websocket.Dial(ctx, rawURL, nil)
	cancel()
	if err != nil {
		return nil, err
	}

	var conf pion.Configuration

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

	var sendAnswer core.Waiter
	defer sendAnswer.Done(nil)

	var connState core.Waiter
	bg := context.Background()

	prod := webrtc.NewConn(pc)
	prod.FormatName = "webrtc/openipc"
	prod.Mode = core.ModeActiveProducer
	prod.Protocol = "ws"
	prod.URL = rawURL
	prod.Listen(func(msg any) {
		switch msg := msg.(type) {
		case *pion.ICECandidate:
			_ = sendAnswer.Wait()

			req := openIPCReq{
				Data: msg.ToJSON().Candidate,
				Req:  "candidate",
			}
			if err = wsjson.Write(bg, conn, &req); err != nil {
				connState.Done(err)
				return
			}

			log.Trace().Msgf("[webrtc] openipc send: %s", req)

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
		var err error

		for err == nil {
			var rep openIPCReply
			if err = wsjson.Read(bg, conn, &rep); err != nil {
				if errors.Is(err, io.ErrUnexpectedEOF) {
					err = nil
					continue
				}
				break
			}

			log.Trace().Msgf("[webrtc] openipc recv: %s", rep)

			switch rep.Reply {
			case "webrtc_answer":
				var sd pion.SessionDescription
				if err = json.Unmarshal(rep.Data, &sd); err != nil {
					break
				}

				if err = prod.SetOffer(sd.SDP); err != nil {
					break
				}

				var answer string
				if answer, err = prod.GetAnswer(); err != nil {
					break
				}

				req := openIPCReq{Data: answer, Req: "answer"}
				if err = wsjson.Write(bg, conn, req); err != nil {
					break
				}

				log.Trace().Msgf("[webrtc] kinesis send: %s", req)

				sendAnswer.Done(nil)

			case "webrtc_candidate":
				var ci pion.ICECandidateInit
				if err = json.Unmarshal(rep.Data, &ci); err != nil {
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

type openIPCReply struct {
	Data  json.RawMessage `json:"data"`
	Reply string          `json:"reply"`
}

func (r openIPCReply) String() string {
	b, _ := json.Marshal(r)
	return string(b)
}

type openIPCReq struct {
	Data string `json:"data"`
	Req  string `json:"req"`
}

func (r openIPCReq) String() string {
	b, _ := json.Marshal(r)
	return string(b)
}
