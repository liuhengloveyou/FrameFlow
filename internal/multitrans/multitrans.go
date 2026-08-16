package multitrans

import (
	"github.com/liuhengloveyou/FrameFlow/internal/streams"
	"github.com/liuhengloveyou/FrameFlow/pkg/multitrans"
)

func Init() {
	streams.HandleFunc("multitrans", multitrans.Dial)
}
