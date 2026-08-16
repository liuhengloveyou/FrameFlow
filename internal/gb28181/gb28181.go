package gb28181

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/liuhengloveyou/FrameFlow/internal/api"
	"github.com/liuhengloveyou/FrameFlow/internal/app"
	"github.com/liuhengloveyou/FrameFlow/internal/streams"
	"github.com/liuhengloveyou/FrameFlow/pkg/core"
	"github.com/liuhengloveyou/FrameFlow/pkg/gb28181"
)

var (
	gw                *gb28181.Gateway
	endpoint          *gb28181.SIPEndpoint
	keepaliveCancel   context.CancelFunc
	applyMu           sync.Mutex
	streamsRegistered bool
)

// Init 打开设备库、注入 HTTP、按 sys_config 启停 SIP（不再依赖 YAML listen）。
func Init() {
	if err := InitStore(); err != nil {
		app.Logger.Error().Err(err).Msg("[gb28181] store init")
		return
	}

	seedGB28181FromYAMLIfNeeded()

	row, err := app.GetGB28181Config()
	if err != nil {
		app.Logger.Warn().Err(err).Msg("[gb28181] get config")
		row = app.DefaultGB28181Config()
	}

	if !streamsRegistered {
		streams.HandleFunc("gb28181", dial)
		streamsRegistered = true
	}
	gb28181.SetLogger(app.GetLogger("gb28181"))
	provideHTTPBackend()
	api.ProvideGB28181Settings(settingsBackend{})

	refreshAppInfo(row)
	if err := applyListenState(row); err != nil {
		app.Logger.Error().Err(err).Msg("[gb28181] apply listen")
	}
}

// seedGB28181FromYAMLIfNeeded：库中尚无国标配置时，从旧 YAML 一次性导入。
func seedGB28181FromYAMLIfNeeded() {
	_, persisted, err := app.GetGB28181ConfigEx()
	if err != nil || persisted {
		return
	}
	var conf struct {
		Mod gb28181.Config `yaml:"gb28181"`
	}
	conf.Mod = gb28181.DefaultConfig()
	app.LoadConfig(&conf)
	if strings.TrimSpace(conf.Mod.Listen) == "" {
		return
	}
	row := fromPkgConfig(conf.Mod)
	if err := app.PutGB28181Config(row); err != nil {
		app.Logger.Warn().Err(err).Msg("[gb28181] seed from yaml failed")
		return
	}
	app.Logger.Info().
		Str("listen", row.ListenAddr()).
		Str("serial", row.Serial).
		Msg("[gb28181] seeded platform config from yaml → sys_config")
}

func fromPkgConfig(c gb28181.Config) app.GB28181Config {
	port := app.ParseListenPort(c.Listen)
	if port <= 0 {
		port = 5060
	}
	row := app.GB28181Config{
		Enabled:      strings.TrimSpace(c.Listen) != "",
		ListenPort:   port,
		Serial:       c.Serial,
		Realm:        c.Realm,
		Password:     c.Password,
		SipIP:        c.SipIP,
		MediaIP:      c.MediaIP,
		MediaPort:    c.MediaPort,
		Transport:    c.Transport,
		AllowUnknown: c.AllowUnknown,
	}
	row.Normalize()
	return row
}

func toPkgConfig(c app.GB28181Config) gb28181.Config {
	c.Normalize()
	return gb28181.NormalizeConfig(gb28181.Config{
		Listen:       c.ListenAddr(),
		Serial:       c.Serial,
		Realm:        c.Realm,
		Password:     c.Password,
		SipIP:        c.SipIP,
		MediaIP:      c.MediaIP,
		MediaPort:    c.MediaPort,
		Transport:    c.Transport,
		AllowUnknown: c.AllowUnknown,
	})
}

func refreshAppInfo(row app.GB28181Config) {
	pkg := toPkgConfig(row)
	dbInfo := ""
	if db := app.DB(); db != nil {
		dbInfo = db.Driver + ":" + db.DSN
	}
	app.Info["gb28181"] = map[string]any{
		"enabled":       row.Enabled,
		"listen":        pkg.Listen,
		"serial":        pkg.Serial,
		"realm":         pkg.Realm,
		"sip_ip":        pkg.SipIP,
		"media_ip":      pkg.MediaIP,
		"media_port":    pkg.MediaPort,
		"transport":     pkg.Transport,
		"db":            dbInfo,
		"allow_unknown": pkg.AllowUnknown,
	}
}

// applyListenState 按配置启停 SIP；可热更新。
func applyListenState(row app.GB28181Config) error {
	applyMu.Lock()
	defer applyMu.Unlock()

	row.Normalize()
	refreshAppInfo(row)

	wantListen := row.ListenAddr()
	if wantListen == "" {
		stopSIPLocked()
		app.Logger.Info().Msg("[gb28181] sip disabled")
		return nil
	}

	pkg := toPkgConfig(row)
	ports, err := gb28181.NewPortPool(pkg.MediaPort)
	if err != nil {
		return fmt.Errorf("media_port: %w", err)
	}

	// 已在跑且关键参数未变则跳过重启（密码/allow_unknown 等仍需重建）
	if endpoint != nil && gw != nil && gw.Config.Listen == pkg.Listen &&
		gw.Config.Serial == pkg.Serial && gw.Config.Realm == pkg.Realm &&
		gw.Config.Password == pkg.Password && gw.Config.SipIP == pkg.SipIP &&
		gw.Config.MediaIP == pkg.MediaIP && gw.Config.MediaPort == pkg.MediaPort &&
		gw.Config.Transport == pkg.Transport && gw.Config.AllowUnknown == pkg.AllowUnknown {
		return nil
	}

	stopSIPLocked()

	gw = &gb28181.Gateway{
		Config:   pkg,
		Registry: gb28181.NewRegistry(),
		Ports:    ports,
	}
	gb28181.SetDefaultGateway(gw)

	if err := loadDevicesFromStore(); err != nil {
		app.Logger.Error().Err(err).Msg("[gb28181] load devices")
	}

	endpoint = gb28181.NewSIPEndpoint(gw, storeHandler{}, app.Logger)
	endpoint.AutoCatalogAfterRegister = false
	gw.SendMessage = endpoint.SendMessage
	gw.InviteLive = endpoint.InviteLive
	gw.InvitePlayback = endpoint.InvitePlayback
	gw.QueryRecordInfo = endpoint.QueryRecordInfo

	if err := endpoint.Start("FrameFlow/" + app.Version); err != nil {
		endpoint = nil
		gw = nil
		gb28181.SetDefaultGateway(nil)
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	keepaliveCancel = cancel
	go endpoint.RunKeepaliveSweeper(ctx)

	app.Logger.Info().
		Str("listen", pkg.Listen).
		Str("serial", pkg.Serial).
		Str("sip_ip", pkg.SipIP).
		Msg("[gb28181] listen")
	return nil
}

func stopSIPLocked() {
	if keepaliveCancel != nil {
		keepaliveCancel()
		keepaliveCancel = nil
	}
	if endpoint != nil {
		_ = endpoint.Close()
		endpoint = nil
	}
	gw = nil
	gb28181.SetDefaultGateway(nil)
}

func loadDevicesFromStore() error {
	if DefaultStore() == nil || gw == nil {
		return nil
	}
	list, err := DefaultStore().ListDevices()
	if err != nil {
		return err
	}
	for _, d := range list {
		d.Online = false
		gw.Registry.UpsertDevice(d)
	}
	app.Logger.Info().Int("count", len(list)).Msg("[gb28181] devices loaded")
	return nil
}

func dial(rawURL string) (core.Producer, error) {
	return gb28181.Dial(rawURL)
}

func isCurrentSIPListenPort(port int) bool {
	row, err := app.GetGB28181Config()
	if err != nil {
		return false
	}
	return row.Enabled && row.ListenPort == port
}

func checkListenPort(port int) (api.PortCheckResult, error) {
	if port <= 0 || port > 65535 {
		return api.PortCheckResult{}, fmt.Errorf("invalid port")
	}
	if isCurrentSIPListenPort(port) {
		return api.PortCheckResult{
			Port:      port,
			Available: true,
			Message:   "当前国标 SIP 正在监听此端口",
		}, nil
	}
	// SIP 默认 UDP；也探测 TCP 占用情况作为提示
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return api.PortCheckResult{
			Port:      port,
			Available: false,
			Message:   "端口已被占用或无法绑定",
		}, nil
	}
	_ = ln.Close()
	pc, err := net.ListenPacket("udp", ":"+strconv.Itoa(port))
	if err != nil {
		return api.PortCheckResult{
			Port:      port,
			Available: false,
			Message:   "UDP 端口已被占用或无法绑定",
		}, nil
	}
	_ = pc.Close()
	return api.PortCheckResult{
		Port:      port,
		Available: true,
		Message:   "端口可用",
	}, nil
}
