package cameras

import (
	"github.com/liuhengloveyou/FrameFlow/internal/api"
	"github.com/liuhengloveyou/FrameFlow/internal/app"
)

var svc *service

// Init 使用系统数据库初始化摄像头模块并注入 api.CamerasBackend。
func Init() {
	st, err := NewStore(app.DB())
	if err != nil {
		app.Logger.Error().Err(err).Msg("[cameras] open store")
		return
	}
	svc = &service{store: st}
	api.ProvideCameras(svc)
	scheduleStreamSync()
	app.Logger.Info().Msg("[cameras] ready")
}
