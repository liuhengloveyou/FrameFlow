package flussonic

import (
	"github.com/liuhengloveyou/FrameFlow/internal/streams"
	"github.com/liuhengloveyou/FrameFlow/pkg/flussonic"
)

func Init() {
	streams.HandleFunc("flussonic", flussonic.Dial)
}
