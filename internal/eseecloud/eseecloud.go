package eseecloud

import (
	"github.com/liuhengloveyou/FrameFlow/internal/streams"
	"github.com/liuhengloveyou/FrameFlow/pkg/eseecloud"
)

func Init() {
	streams.HandleFunc("eseecloud", eseecloud.Dial)
}
