// Package api 提供 HTTP API 的初始化入口（Init）、路由注册及通用响应工具。
// 位于 internal，仅本仓库使用。
// 依赖 app.Init：须先加载全局配置（内含 yaml `api:` → app.APICfg），再调用本包 Init。
//
// 路由：统一用 RegisterBizAPI（方法可空、NeedLogin、comment）。
// 业务 HTTP：模块 Init 只做能力装配；Provide* 集中在 WireProtocols / 各业务 Wire（由 main 在模块就绪后调用），
// 再 MountCameras / MountGB28181 / MountAIBizAPIs / MountRTMPSettings / MountRTSPSettings。
package api
