package api

import (
	"net/http"
	"strconv"

	"github.com/liuhengloveyou/FrameFlow/pkg/fferrors"
)

// GB28181Settings HTTP 出参：国标平台配置视图。
type GB28181Settings struct {
	Enabled         bool   `json:"enabled"`
	ListenPort      int    `json:"listenPort"`
	Serial          string `json:"serial"`
	Realm           string `json:"realm"`
	Password        string `json:"password,omitempty"`
	HasPassword     bool   `json:"hasPassword"`
	SipIP           string `json:"sipIp"`
	MediaIP         string `json:"mediaIp"`
	MediaPort       string `json:"mediaPort"`
	Transport       string `json:"transport"`
	AllowUnknown    bool   `json:"allowUnknown"`
	RunningListen   string `json:"runningListen,omitempty"`
	ResolvedSipIP   string `json:"resolvedSipIp,omitempty"`
	ResolvedMediaIP string `json:"resolvedMediaIp,omitempty"`
	RestartHint     string `json:"restartHint,omitempty"`
}

// GB28181SettingsPatch HTTP 入参（指针字段表示可选）。
type GB28181SettingsPatch struct {
	Enabled      *bool   `json:"enabled"`
	ListenPort   *int    `json:"listenPort"`
	Serial       *string `json:"serial"`
	Realm        *string `json:"realm"`
	Password     *string `json:"password"`
	SipIP        *string `json:"sipIp"`
	MediaIP      *string `json:"mediaIp"`
	MediaPort    *string `json:"mediaPort"`
	Transport    *string `json:"transport"`
	AllowUnknown *bool   `json:"allowUnknown"`
}

// GB28181SettingsBackend 由 internal/gb28181 实现。
type GB28181SettingsBackend interface {
	Get() GB28181Settings
	Update(GB28181SettingsPatch) (GB28181Settings, error)
	CheckListenPort(int) (PortCheckResult, error)
}

var gb28181SettingsBackend GB28181SettingsBackend

// ProvideGB28181Settings 注入国标平台配置后端（不挂路由）。
func ProvideGB28181Settings(b GB28181SettingsBackend) {
	gb28181SettingsBackend = b
}

// MountGB28181Settings 注册国标平台配置 HTTP（须在 Provide 之后由 main 调用）。
func MountGB28181Settings() {
	if gb28181SettingsBackend == nil {
		log.Debug().Msg("[gb28181] skip mount settings http: backend nil")
		return
	}
	const login = true
	RegisterBizAPI("GET", "api/gb28181/settings", login, "国标平台配置", handleGB28181SettingsGet)
	RegisterBizAPI("GET", "api/gb28181/port-check", login, "检测国标 SIP 端口可用性", handleGB28181SettingsPortCheck)
	RegisterBizAPI("PUT", "api/gb28181/settings", login, "更新国标平台配置", handleGB28181SettingsPut)
	log.Info().Msg("[gb28181] http routes registered (/api/gb28181/settings)")
}

func handleGB28181SettingsGet(w http.ResponseWriter, r *http.Request) {
	ResponseJSON(w, gb28181SettingsBackend.Get())
}

func handleGB28181SettingsPortCheck(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("port")
	if q == "" {
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg("missing port"))
		return
	}
	port, err := strconv.Atoi(q)
	if err != nil {
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg("invalid port"))
		return
	}
	out, err := gb28181SettingsBackend.CheckListenPort(port)
	if err != nil {
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg(err.Error()))
		return
	}
	ResponseJSON(w, out)
}

func handleGB28181SettingsPut(w http.ResponseWriter, r *http.Request) {
	var in GB28181SettingsPatch
	if err := DecodeJSON(r, &in); err != nil {
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg(err.Error()))
		return
	}
	out, err := gb28181SettingsBackend.Update(in)
	if err != nil {
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg(err.Error()))
		return
	}
	ResponseJSON(w, out)
}
