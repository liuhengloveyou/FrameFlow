// Package aibiz 实现 FrameFlow AI 业务 API：模型仓库、算法能力、通道绑定、工作流（持久化 + /api/v1）。
// 模型与算法参数保存在数据库；流检测按「绑定 → 算法 → 上传模型」选文件，非配置文件固定单模型。
// HTTP 契约与挂载见 api.ProvideAIBiz / api.MountAIBizAPIs。
package aibiz

import (
	"github.com/liuhengloveyou/FrameFlow/internal/api"
	"github.com/liuhengloveyou/FrameFlow/internal/app"
)

var svc *Service

// Init 打开 SQLite 表并注入 API。
func Init() {
	db := app.DB()
	if db == nil {
		app.Logger.Error().Msg("[aibiz] database not ready")
		return
	}
	st, err := NewStore(db)
	if err != nil {
		app.Logger.Error().Err(err).Msg("[aibiz] store")
		return
	}
	svc = &Service{store: st}
	svc.registerStreamAIPolicy()
	svc.registerDetectionHook()
	svc.ensureORTReady()
	svc.reloadRunningBindingModels()
	svc.seedPresetWorkflows()
	api.ProvideAIBiz(svc)
	// 恢复进程内 running 集合（重启后仍标记为 running 的工作流）
	if rows, err := st.listWorkflows(); err == nil {
		for _, r := range rows {
			if r.Status == "running" {
				svc.runtimeSet(r.ID, true)
			}
		}
	}
	app.Logger.Info().Msg("[aibiz] ready")
}
