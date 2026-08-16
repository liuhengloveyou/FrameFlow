package webrtc

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/liuhengloveyou/FrameFlow/internal/api/ws"
	"github.com/liuhengloveyou/FrameFlow/internal/streams"
	"github.com/liuhengloveyou/FrameFlow/pkg/core"
	"github.com/liuhengloveyou/FrameFlow/pkg/webrtc"
	pion "github.com/pion/webrtc/v4"
)

// streamsHandler supports:
//  1. WHEP:    webrtc:http://192.168.1.123:1984/api/webrtc?src=camera1
//  2. go2rtc:  webrtc:ws://192.168.1.123:1984/api/ws?src=camera1
//  3. Wyze:    webrtc:http://192.168.1.123:5000/signaling/camera1?kvs#format=wyze
//  4. Kinesis: webrtc:wss://...amazonaws.com/?...#format=kinesis#client_id=...#ice_servers=[{...},{...}]
func streamsHandler(rawURL string) (core.Producer, error) {
	var query url.Values
	if i := strings.IndexByte(rawURL, '#'); i > 0 {
		query = streams.ParseQuery(rawURL[i+1:])
		rawURL = rawURL[:i]
	}

	rawURL = rawURL[7:] // remove webrtc:
	if i := strings.IndexByte(rawURL, ':'); i > 0 {
		scheme := rawURL[:i]
		format := query.Get("format")

		switch scheme {
		case "ws", "wss":
			if format == "kinesis" {
				return kinesisClient(rawURL, query, "webrtc/kinesis", nil)
			} else if format == "openipc" {
				return openIPCClient(rawURL, query)
			} else if format == "switchbot" {
				return switchbotClient(rawURL, query)
			} else {
				return go2rtcClient(rawURL)
			}

		case "http", "https":
			if format == "milestone" {
				return milestoneClient(rawURL, query)
			} else if format == "wyze" {
				return wyzeClient(rawURL)
			} else if format == "creality" {
				return crealityClient(rawURL)
			} else {
				return whepClient(rawURL)
			}
		}
	}
	return nil, errors.New("unsupported url: " + rawURL)
}

// go2rtcClient can connect only to go2rtc server
// ex: ws://localhost:1984/api/ws?src=camera1
func go2rtcClient(rawURL string) (core.Producer, error) {
	conn, _, err := Dial(rawURL)
	if err != nil {
		return nil, err
	}
	defer conn.CloseNow()

	pc, err := PeerConnection(true)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			_ = pc.Close()
		}
	}()

	var connState core.Waiter
	var connMu sync.Mutex
	ctx := context.Background()

	prod := webrtc.NewConn(pc)
	prod.Mode = core.ModeActiveProducer
	prod.Protocol = "ws"
	prod.URL = rawURL
	prod.Listen(func(msg any) {
		switch msg := msg.(type) {
		case *pion.ICECandidate:
			s := msg.ToJSON().Candidate
			log.Trace().Str("candidate", s).Msg("[webrtc] local ")
			connMu.Lock()
			_ = wsjson.Write(ctx, conn, &ws.Message{Type: "webrtc/candidate", Value: s})
			connMu.Unlock()

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

	medias := []*core.Media{
		{Kind: core.KindVideo, Direction: core.DirectionRecvonly},
		{Kind: core.KindAudio, Direction: core.DirectionRecvonly},
		{Kind: core.KindAudio, Direction: core.DirectionSendonly},
	}

	offer, err := prod.CreateOffer(medias)
	if err != nil {
		return nil, err
	}

	msg := &ws.Message{Type: "webrtc/offer", Value: offer}
	connMu.Lock()
	_ = wsjson.Write(ctx, conn, msg)
	connMu.Unlock()

	if err = wsjson.Read(ctx, conn, msg); err != nil {
		return nil, err
	}

	if msg.Type != "webrtc/answer" {
		err = errors.New("wrong answer: " + msg.String())
		return nil, err
	}

	answer := msg.String()
	if err = prod.SetAnswer(answer); err != nil {
		return nil, err
	}

	go func() {
		var err error
		for {
			var m ws.Message
			if err = wsjson.Read(ctx, conn, &m); err != nil {
				break
			}
			switch m.Type {
			case "webrtc/candidate":
				if m.Value != nil {
					_ = prod.AddCandidate(m.String())
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

func whepClient(rawURL string) (core.Producer, error) {
	pc, err := PeerConnection(true)
	if err != nil {
		log.Error().Err(err).Caller().Send()
		return nil, err
	}

	prod := webrtc.NewConn(pc)
	prod.Mode = core.ModeActiveProducer
	prod.Protocol = "http"
	prod.URL = rawURL

	medias := []*core.Media{
		{Kind: core.KindVideo, Direction: core.DirectionRecvonly},
		{Kind: core.KindAudio, Direction: core.DirectionRecvonly},
	}

	offer, err := prod.CreateCompleteOffer(medias)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", rawURL, strings.NewReader(offer))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", MimeSDP)

	client := http.Client{Timeout: time.Second * 5000}
	defer client.CloseIdleConnections()

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	answer, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if err = prod.SetAnswer(string(answer)); err != nil {
		return nil, err
	}

	return prod, nil
}

// Dial - websocket.Dial with Basic auth support
func Dial(rawURL string) (*websocket.Conn, *http.Response, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if u.User == nil {
		return websocket.Dial(ctx, rawURL, nil)
	}

	user := u.User.Username()
	pass, _ := u.User.Password()
	u.User = nil

	header := http.Header{
		"Authorization": []string{
			"Basic " + base64.StdEncoding.EncodeToString([]byte(user + ":" + pass)),
		},
	}

	return websocket.Dial(ctx, u.String(), &websocket.DialOptions{HTTPHeader: header})
}
