package api

import (
	"net"
	"net/http"
	"strconv"
	"syscall"

	"github.com/liuhengloveyou/FrameFlow/internal/app"
)

// Init 使用 app.Init 已加载的 app.APICfg，挂载静态与内置路由、装配中间件并启动监听。
func Init() {
	cfg := app.APICfg

	if cfg.Listen == "" && cfg.UnixListen == "" && cfg.TLSListen == "" {
		return
	}

	allowPaths = cfg.AllowPaths
	basePath = cfg.BasePath
	log = app.GetLogger("api")

	initStatic(cfg.StaticDir)
	registerBuiltinHTTPRoutes()
	// 业务路由：main 在模块 Init 后调用 WireProtocols / Provide*，再 Mount*
	// passport /usercenter 由 main.initPassport 在本 Init 之前挂载

	Handler = http.DefaultServeMux

	// 开发前端需 cookie：允许带凭证的 CORS（勿用 *）
	Handler = middlewareCORS(Handler)

	if cfg.Username != "" {
		Handler = middlewareAuth(cfg.Username, cfg.Password, cfg.LocalAuth, Handler)
	}

	if log.Trace().Enabled() {
		Handler = middlewareLog(Handler)
	}

	if cfg.Listen != "" {
		_, port, _ := net.SplitHostPort(cfg.Listen)
		Port, _ = strconv.Atoi(port)
		go listen("tcp", cfg.Listen)
	}

	if cfg.UnixListen != "" {
		_ = syscall.Unlink(cfg.UnixListen)
		go listen("unix", cfg.UnixListen)
	}

	if cfg.TLSListen != "" && cfg.TLSCert != "" && cfg.TLSKey != "" {
		go tlsListen("tcp", cfg.TLSListen, cfg.TLSCert, cfg.TLSKey)
	}
}

// WireProtocols 在 rtmp/rtsp 等模块 Init 之后由 main 调用，集中注入协议相关 HTTP 后端。
// 不能放进 Init：此时协议模块尚未装配完成；且 api 不能 import rtmp/rtsp（循环依赖）。
func WireProtocols(rtmpB RTMPSettingsBackend, streamFLV http.HandlerFunc, rtspB RTSPSettingsBackend, onvifB ONVIFSettingsBackend) {
	ProvideRTMPSettings(rtmpB)
	ProvideRTMPStreamFLV(streamFLV)
	ProvideRTSPSettings(rtspB)
	ProvideONVIFSettings(onvifB)
}
