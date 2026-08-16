package rtsp

import (
	"fmt"
	"net"
	"strings"

	"github.com/liuhengloveyou/FrameFlow/internal/api"
	"github.com/liuhengloveyou/FrameFlow/internal/app"
)

type settingsBackend struct{}

func (settingsBackend) Get() api.RTSPSettings { return GetSettings() }

func (settingsBackend) Update(p api.RTSPSettingsPatch) (api.RTSPSettings, error) {
	return UpdateSettings(p)
}

func (settingsBackend) CheckListenPort(port int) (api.PortCheckResult, error) {
	return checkListenPort(port)
}

// SettingsBackend 供 api.WireProtocols 注入。
func SettingsBackend() api.RTSPSettingsBackend { return settingsBackend{} }

func isCurrentRTSPListenPort(port int) bool {
	row, err := app.GetRTSPConfig()
	if err != nil {
		return false
	}
	return row.Enabled && row.ListenPort == port
}

func checkListenPort(port int) (api.PortCheckResult, error) {
	if port <= 0 || port > 65535 {
		return api.PortCheckResult{}, fmt.Errorf("invalid port")
	}
	if isCurrentRTSPListenPort(port) {
		return api.PortCheckResult{
			Port:      port,
			Available: true,
			Message:   "当前 RTSP 服务正在监听此端口",
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

// GetSettings 将 app.RTSPConfig + 运行态映射为 HTTP DTO。
func GetSettings() api.RTSPSettings {
	row, err := app.GetRTSPConfig()
	if err != nil {
		log.Error().Err(err).Msg("[rtsp] get config")
		row = app.DefaultRTSPConfig()
	}
	port := row.ListenPort
	if port <= 0 {
		port = 8554
	}
	host := strings.TrimSpace(row.AdvertiseHost)
	cands := candidateHosts()
	if host == "" && len(cands) > 0 {
		host = cands[0]
	}
	if host == "" {
		host = "127.0.0.1"
	}
	auth := ""
	if u := strings.TrimSpace(row.Username); u != "" {
		auth = u + "@"
	}
	dq := strings.TrimSpace(row.DefaultQuery)
	if dq == "" {
		dq = "video&audio"
	}
	return api.RTSPSettings{
		Enabled:        row.Enabled,
		ListenPort:     port,
		AdvertiseHost:  strings.TrimSpace(row.AdvertiseHost),
		Username:       strings.TrimSpace(row.Username),
		HasPassword:    strings.TrimSpace(row.Password) != "",
		DefaultQuery:   dq,
		PlayExample:    fmt.Sprintf("rtsp://%s%s:%d/<stream>", auth, host, port),
		RunningListen:  row.ListenAddr(),
		CandidateHosts: cands,
	}
}

// AdvertiseHostForPublish 推流/播放 URL 主机：配置优先，否则候选网卡 IP，否则 127.0.0.1。
func AdvertiseHostForPublish() string {
	row, err := app.GetRTSPConfig()
	if err != nil {
		log.Error().Err(err).Msg("[rtsp] get config")
		row = app.DefaultRTSPConfig()
	}
	if h := strings.TrimSpace(row.AdvertiseHost); h != "" {
		return h
	}
	if cands := candidateHosts(); len(cands) > 0 {
		return cands[0]
	}
	return "127.0.0.1"
}

// ListenPortForPublish 当前配置的 RTSP 端口（未启用时仍返回默认 8554 供展示）。
func ListenPortForPublish() int {
	row, err := app.GetRTSPConfig()
	if err != nil {
		log.Error().Err(err).Msg("[rtsp] get config")
		row = app.DefaultRTSPConfig()
	}
	if row.ListenPort <= 0 {
		return 8554
	}
	return row.ListenPort
}

// UpdateSettings 应用 HTTP patch → 写库 → 按 enabled 启停监听。
func UpdateSettings(p api.RTSPSettingsPatch) (api.RTSPSettings, error) {
	row, err := app.GetRTSPConfig()
	if err != nil {
		return api.RTSPSettings{}, err
	}

	if p.Enabled != nil {
		row.Enabled = *p.Enabled
	}
	if p.ListenPort != nil {
		if *p.ListenPort <= 0 || *p.ListenPort > 65535 {
			return api.RTSPSettings{}, fmt.Errorf("invalid listen port")
		}
		row.ListenPort = *p.ListenPort
	}
	if p.AdvertiseHost != nil {
		row.AdvertiseHost = strings.TrimSpace(*p.AdvertiseHost)
	}
	if p.Username != nil {
		row.Username = strings.TrimSpace(*p.Username)
	}
	if p.Password != nil {
		row.Password = *p.Password
	}
	if p.DefaultQuery != nil {
		row.DefaultQuery = strings.TrimSpace(*p.DefaultQuery)
	}
	row.Normalize()

	if err := app.PutRTSPConfig(row); err != nil {
		return api.RTSPSettings{}, err
	}
	if err := applyListenState(row); err != nil {
		return api.RTSPSettings{}, fmt.Errorf("listen: %w", err)
	}
	return GetSettings(), nil
}
