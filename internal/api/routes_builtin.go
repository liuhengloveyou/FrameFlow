package api

// registerBuiltinHTTPRoutes 注册内置 REST 路由（各业务模块仍通过 RegisterBizAPI 自行登记）。
func registerBuiltinHTTPRoutes() {
	RegisterBizAPI("", "api", false, "", apiHandler)
	RegisterBizAPI("", "api/config", false, "", configHandler)
	RegisterBizAPI("", "api/exit", false, "", exitHandler)
	RegisterBizAPI("", "api/restart", false, "", restartHandler)
	RegisterBizAPI("", "api/log", false, "", logHandler)
	RegisterBizAPI("", "api/webrtc", false, "", serveWebRTCHTTP)
}
