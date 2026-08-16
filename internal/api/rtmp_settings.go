package api

import (
	"net/http"
	"strconv"

	"github.com/liuhengloveyou/FrameFlow/pkg/fferrors"
)

// RTMPSettings HTTP 出参：RTMP 接入配置视图（含运行时只读字段）。
// 持久化实体在 internal/rtmp.Config，不在本包。
type RTMPSettings struct {
	Enabled        bool     `json:"enabled"`
	ListenPort     int      `json:"listenPort"`
	AdvertiseHost  string   `json:"advertiseHost"`
	PushTokenQuery string   `json:"pushTokenQuery,omitempty"`
	PublishExample string   `json:"publishExample,omitempty"`
	RunningListen  string   `json:"runningListen,omitempty"`
	RestartHint    string   `json:"restartHint,omitempty"`
	CandidateHosts []string `json:"candidateHosts,omitempty"`
}

// RTMPSettingsPatch HTTP 入参（指针字段表示可选）。
type RTMPSettingsPatch struct {
	Enabled       *bool   `json:"enabled"`
	ListenPort    *int    `json:"listenPort"`
	AdvertiseHost *string `json:"advertiseHost"`
}

// PortCheckResult 端口可用性检测结果。
type PortCheckResult struct {
	Port      int    `json:"port"`
	Available bool   `json:"available"`
	Message   string `json:"message,omitempty"`
}

// RTMPSettingsBackend 由 internal/rtmp 实现。
type RTMPSettingsBackend interface {
	Get() RTMPSettings
	Update(RTMPSettingsPatch) (RTMPSettings, error)
	CheckListenPort(int) (PortCheckResult, error)
}

var rtmpSettingsBackend RTMPSettingsBackend
var rtmpStreamFLV http.HandlerFunc

// ProvideRTMPSettings 注入 RTMP 配置后端（不挂路由）。由 WireProtocols 集中调用。
func ProvideRTMPSettings(b RTMPSettingsBackend) {
	rtmpSettingsBackend = b
}

// ProvideRTMPStreamFLV 注入 api/stream.flv 处理函数（不挂路由）。
func ProvideRTMPStreamFLV(h http.HandlerFunc) {
	rtmpStreamFLV = h
}

// MountRTMPSettings 注册 RTMP HTTP（须在 WireProtocols / Provide 之后由 main 调用）。
func MountRTMPSettings() {
	if rtmpStreamFLV != nil {
		RegisterBizAPI("", "api/stream.flv", false, "FLV 流出口", rtmpStreamFLV)
	}
	if rtmpSettingsBackend == nil {
		log.Debug().Msg("[rtmp] skip mount settings http: backend nil")
		return
	}
	const login = true
	RegisterBizAPI("GET", "api/rtmp/settings", login, "RTMP 接入配置", handleRTMPSettingsGet)
	RegisterBizAPI("GET", "api/rtmp/port-check", login, "检测 RTMP 监听端口可用性", handleRTMPSettingsPortCheck)
	RegisterBizAPI("PUT", "api/rtmp/settings", login, "更新 RTMP 接入配置", handleRTMPSettingsPut)
	log.Info().Msg("[rtmp] http routes registered (/api/stream.flv, /api/rtmp/settings)")
}

func handleRTMPSettingsGet(w http.ResponseWriter, r *http.Request) {
	ResponseJSON(w, rtmpSettingsBackend.Get())
}

func handleRTMPSettingsPortCheck(w http.ResponseWriter, r *http.Request) {
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
	log.Info().Int("port", port).Msg("[rtmp] port-check request")
	out, err := rtmpSettingsBackend.CheckListenPort(port)
	if err != nil {
		log.Error().Err(err).Int("port", port).Msg("[rtmp] port-check failed")
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg(err.Error()))
		return
	}
	log.Info().Int("port", port).Bool("available", out.Available).Msg("[rtmp] port-check result")
	ResponseJSON(w, out)
}

func handleRTMPSettingsPut(w http.ResponseWriter, r *http.Request) {
	var in RTMPSettingsPatch
	if err := DecodeJSON(r, &in); err != nil {
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg(err.Error()))
		return
	}
	out, err := rtmpSettingsBackend.Update(in)
	if err != nil {
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg(err.Error()))
		return
	}
	ResponseJSON(w, out)
}
