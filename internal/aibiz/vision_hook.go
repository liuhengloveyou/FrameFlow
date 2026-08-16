package aibiz

import (
	"github.com/liuhengloveyou/FrameFlow/internal/app"
	"github.com/liuhengloveyou/FrameFlow/internal/vision"
)

// ensureORTReady 仅预热 ONNX Runtime。
// 不加载任何“系统默认”ONNX：模型与算法参数均在数据库，由绑定启动 / 流检测按算法关联路径加载。
func (s *Service) ensureORTReady() {
	vision.EnsureORTReady()
	app.Logger.Info().Msg("[aibiz] vision: ORT ready; models load per algorithm binding from DB")
}
