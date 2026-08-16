package api

import (
	"net/http"
	"strconv"

	"github.com/liuhengloveyou/FrameFlow/pkg/fferrors"
)

// RTSPSettings HTTP 出参：RTSP 服务配置视图（含运行时只读字段）。
// 持久化实体在 internal/rtsp.Config，不在本包。
type RTSPSettings struct {
	Enabled        bool     `json:"enabled"`
	ListenPort     int      `json:"listenPort"`
	AdvertiseHost  string   `json:"advertiseHost"`
	Username       string   `json:"username"`
	HasPassword    bool     `json:"hasPassword"`
	DefaultQuery   string   `json:"defaultQuery"`
	PlayExample    string   `json:"playExample,omitempty"`
	RunningListen  string   `json:"runningListen,omitempty"`
	RestartHint    string   `json:"restartHint,omitempty"`
	CandidateHosts []string `json:"candidateHosts,omitempty"`
}

// RTSPSettingsPatch HTTP 入参。
type RTSPSettingsPatch struct {
	Enabled       *bool   `json:"enabled"`
	ListenPort    *int    `json:"listenPort"`
	AdvertiseHost *string `json:"advertiseHost"`
	Username      *string `json:"username"`
	Password      *string `json:"password"` // 空字符串清空；nil 不改
	DefaultQuery  *string `json:"defaultQuery"`
}

// RTSPSettingsBackend 由 internal/rtsp 实现。
type RTSPSettingsBackend interface {
	Get() RTSPSettings
	Update(RTSPSettingsPatch) (RTSPSettings, error)
	CheckListenPort(int) (PortCheckResult, error)
}

var rtspSettingsBackend RTSPSettingsBackend

// ProvideRTSPSettings 注入 RTSP 配置后端（不挂路由）。由 WireProtocols 集中调用。
func ProvideRTSPSettings(b RTSPSettingsBackend) {
	rtspSettingsBackend = b
}

// MountRTSPSettings 注册 /api/rtsp/settings（须在 Provide 之后由 main 调用）。
func MountRTSPSettings() {
	if rtspSettingsBackend == nil {
		log.Debug().Msg("[rtsp] skip mount settings http: backend nil")
		return
	}
	const login = true
	RegisterBizAPI("GET", "api/rtsp/settings", login, "RTSP 服务配置", handleRTSPSettingsGet)
	RegisterBizAPI("GET", "api/rtsp/port-check", login, "检测 RTSP 监听端口可用性", handleRTSPSettingsPortCheck)
	RegisterBizAPI("PUT", "api/rtsp/settings", login, "更新 RTSP 服务配置", handleRTSPSettingsPut)
	log.Info().Msg("[rtsp] http routes registered (/api/rtsp/settings)")
}

func handleRTSPSettingsGet(w http.ResponseWriter, r *http.Request) {
	ResponseJSON(w, rtspSettingsBackend.Get())
}

func handleRTSPSettingsPortCheck(w http.ResponseWriter, r *http.Request) {
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
	log.Info().Int("port", port).Msg("[rtsp] port-check request")
	out, err := rtspSettingsBackend.CheckListenPort(port)
	if err != nil {
		log.Error().Err(err).Int("port", port).Msg("[rtsp] port-check failed")
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg(err.Error()))
		return
	}
	log.Info().Int("port", port).Bool("available", out.Available).Msg("[rtsp] port-check result")
	ResponseJSON(w, out)
}

func handleRTSPSettingsPut(w http.ResponseWriter, r *http.Request) {
	var in RTSPSettingsPatch
	if err := DecodeJSON(r, &in); err != nil {
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg(err.Error()))
		return
	}
	out, err := rtspSettingsBackend.Update(in)
	if err != nil {
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg(err.Error()))
		return
	}
	ResponseJSON(w, out)
}
