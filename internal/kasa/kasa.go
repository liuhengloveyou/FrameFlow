package kasa

import (
	"github.com/liuhengloveyou/FrameFlow/internal/streams"
	"github.com/liuhengloveyou/FrameFlow/pkg/core"
	"github.com/liuhengloveyou/FrameFlow/pkg/kasa"
)

func Init() {
	streams.HandleFunc("kasa", func(source string) (core.Producer, error) {
		return kasa.Dial(source)
	})
}
