package onvif

import (
	"fmt"
	"strings"
	"time"

	"github.com/liuhengloveyou/FrameFlow/internal/api"
	"github.com/liuhengloveyou/FrameFlow/internal/app"
)

type settingsBackend struct{}

func (settingsBackend) Get() api.ONVIFSettings { return GetSettings() }

func (settingsBackend) Update(p api.ONVIFSettingsPatch) (api.ONVIFSettings, error) {
	return UpdateSettings(p)
}

// SettingsBackend 供 api.WireProtocols 注入。
func SettingsBackend() api.ONVIFSettingsBackend { return settingsBackend{} }

// GetSettings 将 app.ONVIFConfig 映射为 HTTP DTO。
func GetSettings() api.ONVIFSettings {
	row, err := app.GetONVIFConfig()
	if err != nil {
		log.Error().Err(err).Msg("[onvif] get config")
		row = app.DefaultONVIFConfig()
	}
	timeout := row.DiscoverTimeoutSec
	if timeout <= 0 {
		timeout = 5
	}
	return api.ONVIFSettings{
		ServerEnabled:      row.ServerEnabled,
		DefaultUsername:    strings.TrimSpace(row.DefaultUsername),
		HasDefaultPassword: strings.TrimSpace(row.DefaultPassword) != "",
		DiscoverTimeoutSec: timeout,
		ClientExample:      "onvif://user:pass@192.168.1.100?subtype=0",
		ServerPath:         "/onvif/",
	}
}

// DiscoverTimeout 发现等待时长。
func DiscoverTimeout() time.Duration {
	row, err := app.GetONVIFConfig()
	if err != nil {
		return 5 * time.Second
	}
	sec := row.DiscoverTimeoutSec
	if sec <= 0 {
		sec = 5
	}
	return time.Duration(sec) * time.Second
}

// DefaultCredentials 发现后填表用的默认账号（密码仅后端持有，API 不回传明文）。
func DefaultCredentials() (user, pass string) {
	row, err := app.GetONVIFConfig()
	if err != nil {
		return "", ""
	}
	return strings.TrimSpace(row.DefaultUsername), row.DefaultPassword
}

// ServerEnabled 是否对外提供 ONVIF Device/Media 服务。
func ServerEnabled() bool {
	row, err := app.GetONVIFConfig()
	if err != nil {
		return true
	}
	return row.ServerEnabled
}

// UpdateSettings 应用 HTTP patch → 写库。
func UpdateSettings(p api.ONVIFSettingsPatch) (api.ONVIFSettings, error) {
	row, err := app.GetONVIFConfig()
	if err != nil {
		return api.ONVIFSettings{}, err
	}
	if p.ServerEnabled != nil {
		row.ServerEnabled = *p.ServerEnabled
	}
	if p.DefaultUsername != nil {
		row.DefaultUsername = strings.TrimSpace(*p.DefaultUsername)
	}
	if p.DefaultPassword != nil {
		row.DefaultPassword = *p.DefaultPassword
	}
	if p.DiscoverTimeoutSec != nil {
		if *p.DiscoverTimeoutSec <= 0 || *p.DiscoverTimeoutSec > 60 {
			return api.ONVIFSettings{}, fmt.Errorf("discover timeout must be 1-60 seconds")
		}
		row.DiscoverTimeoutSec = *p.DiscoverTimeoutSec
	}
	if err := app.PutONVIFConfig(row); err != nil {
		return api.ONVIFSettings{}, err
	}
	return GetSettings(), nil
}
