package api

import (
	"net/http"

	"github.com/liuhengloveyou/FrameFlow/pkg/fferrors"
)

// ONVIFSettings HTTP 出参：ONVIF 协议配置视图。
type ONVIFSettings struct {
	ServerEnabled      bool   `json:"serverEnabled"`
	DefaultUsername    string `json:"defaultUsername"`
	HasDefaultPassword bool   `json:"hasDefaultPassword"`
	DiscoverTimeoutSec int    `json:"discoverTimeoutSec"`
	ClientExample      string `json:"clientExample,omitempty"`
	ServerPath         string `json:"serverPath,omitempty"`
}

// ONVIFSettingsPatch HTTP 入参。
type ONVIFSettingsPatch struct {
	ServerEnabled      *bool   `json:"serverEnabled"`
	DefaultUsername    *string `json:"defaultUsername"`
	DefaultPassword    *string `json:"defaultPassword"` // 空字符串清空；nil 不改
	DiscoverTimeoutSec *int    `json:"discoverTimeoutSec"`
}

// ONVIFSettingsBackend 由 internal/onvif 实现。
type ONVIFSettingsBackend interface {
	Get() ONVIFSettings
	Update(ONVIFSettingsPatch) (ONVIFSettings, error)
}

var onvifSettingsBackend ONVIFSettingsBackend

// ProvideONVIFSettings 注入 ONVIF 配置后端。由 WireProtocols 集中调用。
func ProvideONVIFSettings(b ONVIFSettingsBackend) {
	onvifSettingsBackend = b
}

// MountONVIFSettings 注册 /api/onvif/settings。
func MountONVIFSettings() {
	if onvifSettingsBackend == nil {
		log.Debug().Msg("[onvif] skip mount settings http: backend nil")
		return
	}
	const login = true
	RegisterBizAPI("GET", "api/onvif/settings", login, "ONVIF 协议配置", handleONVIFSettingsGet)
	RegisterBizAPI("PUT", "api/onvif/settings", login, "更新 ONVIF 协议配置", handleONVIFSettingsPut)
	log.Info().Msg("[onvif] http routes registered (/api/onvif/settings)")
}

func handleONVIFSettingsGet(w http.ResponseWriter, r *http.Request) {
	ResponseJSON(w, onvifSettingsBackend.Get())
}

func handleONVIFSettingsPut(w http.ResponseWriter, r *http.Request) {
	var in ONVIFSettingsPatch
	if err := DecodeJSON(r, &in); err != nil {
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg(err.Error()))
		return
	}
	out, err := onvifSettingsBackend.Update(in)
	if err != nil {
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg(err.Error()))
		return
	}
	ResponseJSON(w, out)
}
