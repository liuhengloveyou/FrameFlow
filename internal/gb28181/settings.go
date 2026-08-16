package gb28181

import (
	"fmt"
	"strings"

	"github.com/liuhengloveyou/FrameFlow/internal/api"
	"github.com/liuhengloveyou/FrameFlow/internal/app"
)

type settingsBackend struct{}

func (settingsBackend) Get() api.GB28181Settings { return GetSettings() }

func (settingsBackend) Update(p api.GB28181SettingsPatch) (api.GB28181Settings, error) {
	return UpdateSettings(p)
}

func (settingsBackend) CheckListenPort(port int) (api.PortCheckResult, error) {
	return checkListenPort(port)
}

// SettingsBackend 供 WireProtocols / Provide 注入（Init 内已 Provide）。
func SettingsBackend() api.GB28181SettingsBackend { return settingsBackend{} }

// GetSettings 将 app.GB28181Config + 运行态映射为 HTTP DTO。
func GetSettings() api.GB28181Settings {
	row, err := app.GetGB28181Config()
	if err != nil {
		app.Logger.Error().Err(err).Msg("[gb28181] get config")
		row = app.DefaultGB28181Config()
	}
	pkg := toPkgConfig(row)
	return api.GB28181Settings{
		Enabled:         row.Enabled,
		ListenPort:      row.ListenPort,
		Serial:          row.Serial,
		Realm:           row.Realm,
		Password:        row.Password,
		HasPassword:     strings.TrimSpace(row.Password) != "",
		SipIP:           row.SipIP,
		MediaIP:         row.MediaIP,
		MediaPort:       row.MediaPort,
		Transport:       row.Transport,
		AllowUnknown:    row.AllowUnknown,
		RunningListen:   pkg.Listen,
		ResolvedSipIP:   pkg.SipIP,
		ResolvedMediaIP: pkg.MediaIP,
	}
}

// UpdateSettings 应用 HTTP patch → 写库 → 按 enabled 启停 SIP。
func UpdateSettings(p api.GB28181SettingsPatch) (api.GB28181Settings, error) {
	row, err := app.GetGB28181Config()
	if err != nil {
		return api.GB28181Settings{}, err
	}

	if p.Enabled != nil {
		row.Enabled = *p.Enabled
	}
	if p.ListenPort != nil {
		if *p.ListenPort <= 0 || *p.ListenPort > 65535 {
			return api.GB28181Settings{}, fmt.Errorf("invalid listen port")
		}
		row.ListenPort = *p.ListenPort
	}
	if p.Serial != nil {
		row.Serial = strings.TrimSpace(*p.Serial)
	}
	if p.Realm != nil {
		row.Realm = strings.TrimSpace(*p.Realm)
	}
	if p.Password != nil {
		row.Password = *p.Password
	}
	if p.SipIP != nil {
		row.SipIP = strings.TrimSpace(*p.SipIP)
	}
	if p.MediaIP != nil {
		row.MediaIP = strings.TrimSpace(*p.MediaIP)
	}
	if p.MediaPort != nil {
		row.MediaPort = strings.TrimSpace(*p.MediaPort)
	}
	if p.Transport != nil {
		row.Transport = strings.TrimSpace(*p.Transport)
	}
	if p.AllowUnknown != nil {
		row.AllowUnknown = *p.AllowUnknown
	}
	row.Normalize()

	if err := app.PutGB28181Config(row); err != nil {
		return api.GB28181Settings{}, err
	}
	if err := applyListenState(row); err != nil {
		return api.GB28181Settings{}, fmt.Errorf("listen: %w", err)
	}
	return GetSettings(), nil
}
