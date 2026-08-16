package gb28181

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
	"github.com/icholy/digest"
	"github.com/rs/zerolog"

	"github.com/liuhengloveyou/FrameFlow/pkg/fferrors"
)

// SIPEndpoint is the GB28181 SIP protocol endpoint (REGISTER / MESSAGE / INVITE).
// Business policy is injected via Handler; this type must not import application packages.
type SIPEndpoint struct {
	GW      *Gateway
	Handler Handler
	Log     zerolog.Logger

	// AutoCatalogAfterRegister sends a Catalog query ~300ms after successful register.
	AutoCatalogAfterRegister bool

	ua  *sipgo.UserAgent
	srv *sipgo.Server
	cli *sipgo.Client

	nonceMu sync.Mutex
	nonces  map[string]digest.Challenge

	recordMu   sync.Mutex
	recordWait map[string]*recordWait

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

type recordWait struct {
	sumNum int
	items  []RecordItem
	ch     chan struct{}
	done   bool
}

// NewSIPEndpoint builds a GB28181 SIP protocol endpoint bound to gw. Handler may be nil (NopHandler).
func NewSIPEndpoint(gw *Gateway, handler Handler, log zerolog.Logger) *SIPEndpoint {
	if handler == nil {
		handler = NopHandler{}
	}
	return &SIPEndpoint{
		GW:                       gw,
		Handler:                  handler,
		Log:                      log,
		AutoCatalogAfterRegister: true,
		nonces:                   map[string]digest.Challenge{},
		recordWait:               map[string]*recordWait{},
	}
}

// Start creates the sipgo UA/server/client and begins listening on gw.Config.Listen (UDP).
func (ep *SIPEndpoint) Start(userAgent string) error {
	if ep.GW == nil {
		return fferrors.ErrGbGatewayDisabled
	}
	cfg := ep.GW.Config
	if cfg.Listen == "" {
		return fferrors.ErrGbGatewayDisabled
	}
	if userAgent == "" {
		userAgent = "FrameFlow-GB28181"
	}

	ua, err := sipgo.NewUA(sipgo.WithUserAgent(userAgent))
	if err != nil {
		return err
	}
	srv, err := sipgo.NewServer(ua)
	if err != nil {
		_ = ua.Close()
		return err
	}
	cli, err := sipgo.NewClient(ua, sipgo.WithClientHostname(cfg.SipIP))
	if err != nil {
		_ = ua.Close()
		return err
	}

	ep.ua = ua
	ep.srv = srv
	ep.cli = cli

	srv.OnRegister(ep.onRegister)
	srv.OnMessage(ep.onMessage)
	srv.OnNotify(ep.onMessage)
	// 未单独注册的方法也打一条入口日志，方便对照摄像头侧「已注册」却看不到业务日志的情况。
	for _, m := range []sip.RequestMethod{sip.OPTIONS, sip.SUBSCRIBE, sip.INFO, sip.BYE, sip.INVITE, sip.ACK, sip.CANCEL} {
		method := m
		srv.OnRequest(method, func(req *sip.Request, tx sip.ServerTransaction) {
			ep.Log.Info().
				Str("method", string(req.Method)).
				Str("device", deviceIDFromFrom(req)).
				Str("remote", req.Source()).
				Msg("[gb28181] sip: unhandled")
			_ = tx.Respond(sip.NewResponseFromRequest(req, 200, "OK", nil))
		})
	}

	ctx, cancel := context.WithCancel(context.Background())
	ep.cancel = cancel
	ep.wg.Add(1)
	go func() {
		defer ep.wg.Done()
		ep.Log.Info().Str("listen", cfg.Listen).Str("network", "udp").Msg("[gb28181] sip: serving")
		if err := srv.ListenAndServe(ctx, "udp", cfg.Listen); err != nil {
			ep.Log.Error().Err(err).Str("listen", cfg.Listen).Msg("[gb28181] sip serve")
		}
	}()
	return nil
}

// Close stops the SIP listener.
func (ep *SIPEndpoint) Close() error {
	if ep.cancel != nil {
		ep.cancel()
	}
	ep.wg.Wait()
	if ep.ua != nil {
		return ep.ua.Close()
	}
	return nil
}

func deviceIDFromFrom(req *sip.Request) string {
	from := req.From()
	if from == nil {
		return ""
	}
	return from.Address.User
}

func listenPort(listen string) string {
	_, port, err := net.SplitHostPort(listen)
	if err != nil {
		return "5060"
	}
	return port
}

func detectIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}

// NormalizeConfig fills defaults and auto-detects SipIP/MediaIP when empty.
func NormalizeConfig(cfg Config) Config {
	def := DefaultConfig()
	if cfg.Serial == "" {
		cfg.Serial = def.Serial
	}
	if cfg.Realm == "" {
		cfg.Realm = def.Realm
	}
	if cfg.MediaPort == "" {
		cfg.MediaPort = def.MediaPort
	}
	if cfg.Transport == "" {
		cfg.Transport = def.Transport
	}
	if cfg.SipIP == "" {
		cfg.SipIP = detectIP()
	}
	if cfg.MediaIP == "" {
		cfg.MediaIP = cfg.SipIP
	}
	return cfg
}

// RunKeepaliveSweeper marks devices offline when keepalive exceeds Gateway timeout
// and notifies Handler so business can persist / sync presence.
func (ep *SIPEndpoint) RunKeepaliveSweeper(ctx context.Context) {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if ep.GW == nil || ep.GW.Registry == nil {
				continue
			}
			for _, id := range ep.GW.Registry.SweepOffline(ep.GW.KeepaliveTimeout()) {
				ep.Log.Info().Str("device", id).Msg("[gb28181] keepalive timeout → offline")
				if ep.Handler != nil {
					ep.Handler.OnUnregistered(id)
				}
			}
		}
	}
}
