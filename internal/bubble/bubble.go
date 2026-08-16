package bubble

import (
	"github.com/liuhengloveyou/FrameFlow/internal/streams"
	"github.com/liuhengloveyou/FrameFlow/pkg/bubble"
	"github.com/liuhengloveyou/FrameFlow/pkg/core"
)

func Init() {
	streams.HandleFunc("bubble", func(source string) (core.Producer, error) {
		return bubble.Dial(source)
	})
}
