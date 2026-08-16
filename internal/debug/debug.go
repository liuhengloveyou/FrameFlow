package debug

import (
	"github.com/liuhengloveyou/FrameFlow/internal/api"
)

func Init() {
	api.RegisterBizAPI("", "api/stack", false, "", stackHandler)
}
