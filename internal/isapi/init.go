package isapi

import (
	"github.com/liuhengloveyou/FrameFlow/internal/streams"
	"github.com/liuhengloveyou/FrameFlow/pkg/core"
	"github.com/liuhengloveyou/FrameFlow/pkg/isapi"
)

func Init() {
	streams.HandleFunc("isapi", func(source string) (core.Producer, error) {
		return isapi.Dial(source)
	})
}
