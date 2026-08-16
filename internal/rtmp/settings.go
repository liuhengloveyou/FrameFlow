package rtmp

import (
	"fmt"
	"net"
	"strings"

	"github.com/liuhengloveyou/FrameFlow/internal/api"
	"github.com/liuhengloveyou/FrameFlow/internal/app"
	"github.com/liuhengloveyou/FrameFlow/internal/streams"
)

// 实际 TCP listener 见 server.go；配置状态一律读库（app.RTMPConfig）。

type settingsBackend struct{}

func (settingsBackend) Get() api.RTMPSettings { return GetSettings() }

func (settingsBackend) Update(p api.RTMPSettingsPatch) (api.RTMPSettings, error) {
	return UpdateSettings(p)
}

func (settingsBackend) CheckListenPort(port int) (api.PortCheckResult, error) {
	return checkListenPort(port)
}

// SettingsBackend 供 api.WireProtocols 注入。
func SettingsBackend() api.RTMPSettingsBackend { return settingsBackend{} }

func isCurrentRTMPListenPort(port int) bool {
	row, err := app.GetRTMPConfig()
	if err != nil {
		return false
	}
	return row.Enabled && row.ListenPort == port
}

func checkListenPort(port int) (api.PortCheckResult, error) {
	if port <= 0 || port > 65535 {
		return api.PortCheckResult{}, fmt.Errorf("invalid port")
	}
	if isCurrentRTMPListenPort(port) {
		return api.PortCheckResult{
			Port:      port,
			Available: true,
			Message:   "当前 RTMP 服务正在监听此端口",
		}, nil
	}
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return api.PortCheckResult{
			Port:      port,
			Available: false,
			Message:   "端口已被占用或无法绑定",
		}, nil
	}
	_ = ln.Close()
	return api.PortCheckResult{
		Port:      port,
		Available: true,
		Message:   "端口可用",
	}, nil
}

func candidateHosts() []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(h string) {
		h = strings.TrimSpace(h)
		if h == "" || h == "127.0.0.1" || h == "::1" || strings.HasPrefix(h, "fe80:") {
			return
		}
		if _, ok := seen[h]; ok {
			return
		}
		seen[h] = struct{}{}
		out = append(out, h)
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return out
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if v4 := ip.To4(); v4 != nil {
				add(v4.String())
			}
		}
	}
	return out
}

// GetSettings 将 app.RTMPConfig + 运行态映射为 HTTP DTO。
func GetSettings() api.RTMPSettings {
	row, err := app.GetRTMPConfig()
	if err != nil {
		log.Error().Err(err).Msg("[rtmp] get config")
		row = app.DefaultRTMPConfig()
	}
	port := row.ListenPort
	if port <= 0 {
		port = 1935
	}
	host := strings.TrimSpace(row.AdvertiseHost)
	cands := candidateHosts()
	if host == "" && len(cands) > 0 {
		host = cands[0]
	}
	if host == "" {
		host = "127.0.0.1"
	}
	tq := strings.TrimSpace(row.PushTokenQuery)
	if tq == "" {
		tq = pushTokenQuery
	}
	if tq == "" {
		tq = streams.PushTokenQueryParam
	}
	return api.RTMPSettings{
		Enabled:        row.Enabled,
		ListenPort:     port,
		AdvertiseHost:  strings.TrimSpace(row.AdvertiseHost),
		PublishExample: fmt.Sprintf("rtmp://%s:%d/<stream_key>", host, port),
		RunningListen:  row.ListenAddr(),
		CandidateHosts: cands,
		PushTokenQuery: tq,
	}
}

// AdvertiseHostForPublish 推流 URL 主机：配置优先，否则候选网卡 IP，否则 127.0.0.1。
func AdvertiseHostForPublish() string {
	row, err := app.GetRTMPConfig()
	if err != nil {
		log.Error().Err(err).Msg("[rtmp] get config")
		row = app.DefaultRTMPConfig()
	}
	if h := strings.TrimSpace(row.AdvertiseHost); h != "" {
		return h
	}
	if cands := candidateHosts(); len(cands) > 0 {
		return cands[0]
	}
	return "127.0.0.1"
}

// ListenPortForPublish 当前配置的 RTMP 端口（未启用时仍返回默认 1935 供展示）。
func ListenPortForPublish() int {
	row, err := app.GetRTMPConfig()
	if err != nil {
		log.Error().Err(err).Msg("[rtmp] get config")
		row = app.DefaultRTMPConfig()
	}
	if row.ListenPort <= 0 {
		return 1935
	}
	return row.ListenPort
}

// UpdateSettings 应用 HTTP patch → 写库 → 按 enabled 启停监听。
func UpdateSettings(p api.RTMPSettingsPatch) (api.RTMPSettings, error) {
	row, err := app.GetRTMPConfig()
	if err != nil {
		return api.RTMPSettings{}, err
	}

	if p.Enabled != nil {
		row.Enabled = *p.Enabled
	}
	if p.ListenPort != nil {
		if *p.ListenPort <= 0 || *p.ListenPort > 65535 {
			return api.RTMPSettings{}, fmt.Errorf("invalid listen port")
		}
		row.ListenPort = *p.ListenPort
	}
	if p.AdvertiseHost != nil {
		row.AdvertiseHost = strings.TrimSpace(*p.AdvertiseHost)
	}
	row.Normalize()

	if err := app.PutRTMPConfig(row); err != nil {
		return api.RTMPSettings{}, err
	}
	if err := applyListenState(row.ListenAddr(), row.PushTokenQuery); err != nil {
		return api.RTMPSettings{}, fmt.Errorf("listen: %w", err)
	}
	return GetSettings(), nil
}
