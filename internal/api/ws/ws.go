package ws

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/liuhengloveyou/FrameFlow/internal/api"
	"github.com/liuhengloveyou/FrameFlow/internal/app"
	"github.com/liuhengloveyou/FrameFlow/pkg/creds"
	"github.com/rs/zerolog"
)

func Init() {
	var cfg struct {
		Mod struct {
			Origin string `yaml:"origin"`
		} `yaml:"api"`
	}

	app.LoadConfig(&cfg)

	log = app.GetLogger("api")

	wsOrigin = cfg.Mod.Origin

	api.RegisterBizAPI("", "api/ws", false, "", apiWS)
}

var (
	log      zerolog.Logger
	wsOrigin string // "" same-origin; "*" any; else library default
)

// Message - struct for data exchange in Web API
type Message struct {
	Type  string `json:"type"`
	Value any    `json:"value,omitempty"`
}

func (m *Message) String() (value string) {
	if s, ok := m.Value.(string); ok {
		return s
	}
	return
}

func (m *Message) Unmarshal(v any) error {
	b, err := json.Marshal(m.Value)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

type WSHandler func(tr *Transport, msg *Message) error

func HandleFunc(msgType string, handler WSHandler) {
	wsHandlers[msgType] = handler
}

var wsHandlers = make(map[string]WSHandler)

func hostnameOnly(hostport string) string {
	// strip port; keep IPv6 bracket form host as-is when no port
	if host, _, err := net.SplitHostPort(hostport); err == nil {
		return host
	}
	return hostport
}

// loopbackAlias maps common loopback names so Vite (localhost:5173) → API (127.0.0.1:8888) works.
func loopbackAlias(host string) string {
	switch host {
	case "localhost", "127.0.0.1", "::1", "[::1]":
		return "loopback"
	default:
		return host
	}
}

func sameWSOriginHost(originHost, requestHost string) bool {
	oh, rh := hostnameOnly(originHost), hostnameOnly(requestHost)
	if oh == rh {
		return true
	}
	return loopbackAlias(oh) == loopbackAlias(rh) && loopbackAlias(oh) == "loopback"
}

func checkOrigin(r *http.Request) bool {
	switch wsOrigin {
	case "*":
		return true
	case "":
		// same host + ignore port (https://github.com/liuhengloveyou/FrameFlow/issues/118)
		// also treat localhost / 127.0.0.1 / ::1 as same (Vite proxy rewrite Host)
		origin := r.Header["Origin"]
		if len(origin) == 0 {
			return true
		}
		o, err := url.Parse(origin[0])
		if err != nil {
			return false
		}
		if sameWSOriginHost(o.Host, r.Host) {
			return true
		}
		log.Trace().Msgf("[api] ws origin=%s, host=%s", o.Host, r.Host)
		return false
	default:
		return true // let Accept apply default host check
	}
}

func acceptOptions() *websocket.AcceptOptions {
	switch wsOrigin {
	case "*", "":
		// we already validated (or allow all); skip library Origin check
		return &websocket.AcceptOptions{InsecureSkipVerify: true}
	default:
		return nil
	}
}

func apiWS(w http.ResponseWriter, r *http.Request) {
	if !checkOrigin(r) {
		origin := r.Header.Get("Origin")
		log.Error().Str("host", r.Host).Str("origin", origin).Msg("[api] ws origin rejected")
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}

	conn, err := websocket.Accept(w, r, acceptOptions())
	if err != nil {
		origin := r.Header.Get("Origin")
		log.Error().Err(err).Caller().Msgf("host=%s origin=%s", r.Host, origin)
		return
	}
	defer conn.CloseNow()

	// SDP / MSE binary frames can be large
	conn.SetReadLimit(512 << 10)

	ctx := context.Background()

	tr := &Transport{Request: r}
	tr.OnWrite(func(msg any) error {
		wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if data, ok := msg.([]byte); ok {
			return conn.Write(wctx, websocket.MessageBinary, data)
		}
		return wsjson.Write(wctx, conn, msg)
	})

	for {
		msg := new(Message)
		if err = wsjson.Read(ctx, conn, msg); err != nil {
			status := websocket.CloseStatus(err)
			if status != websocket.StatusNormalClosure &&
				status != websocket.StatusGoingAway &&
				status != websocket.StatusNoStatusRcvd {
				log.Trace().Err(err).Caller().Send()
			}
			_ = conn.Close(websocket.StatusNormalClosure, "")
			break
		}

		log.Trace().Str("type", msg.Type).Msg("[api] ws msg")

		if handler := wsHandlers[msg.Type]; handler != nil {
			go func(msg *Message) {
				if err := handler(tr, msg); err != nil {
					errMsg := creds.SecretString(err.Error())
					tr.Write(&Message{Type: "error", Value: msg.Type + ": " + errMsg})
				}
			}(msg)
		}
	}

	tr.Close()
}

type Transport struct {
	Request *http.Request

	ctx map[any]any

	closed bool
	mx     sync.Mutex
	wrmx   sync.Mutex

	onChange func()
	onWrite  func(msg any) error
	onClose  []func()
}

func (t *Transport) OnWrite(f func(msg any) error) {
	t.mx.Lock()
	if t.onChange != nil {
		t.onChange()
	}
	t.onWrite = f
	t.mx.Unlock()
}

func (t *Transport) Write(msg any) {
	t.wrmx.Lock()
	_ = t.onWrite(msg)
	t.wrmx.Unlock()
}

func (t *Transport) Close() {
	t.mx.Lock()
	for _, f := range t.onClose {
		f()
	}
	t.closed = true
	t.mx.Unlock()
}

func (t *Transport) OnChange(f func()) {
	t.mx.Lock()
	t.onChange = f
	t.mx.Unlock()
}

func (t *Transport) OnClose(f func()) {
	t.mx.Lock()
	if t.closed {
		f()
	} else {
		t.onClose = append(t.onClose, f)
	}
	t.mx.Unlock()
}

// WithContext - run function with Context variable
func (t *Transport) WithContext(f func(ctx map[any]any)) {
	t.mx.Lock()
	if t.ctx == nil {
		t.ctx = map[any]any{}
	}
	f(t.ctx)
	t.mx.Unlock()
}

func (t *Transport) Writer() io.Writer {
	return &writer{t: t}
}

type writer struct {
	t *Transport
}

func (w *writer) Write(p []byte) (n int, err error) {
	w.t.wrmx.Lock()
	if err = w.t.onWrite(p); err == nil {
		n = len(p)
	}
	w.t.wrmx.Unlock()
	return
}
