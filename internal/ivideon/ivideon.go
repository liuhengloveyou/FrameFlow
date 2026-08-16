package ivideon

import (
	"github.com/liuhengloveyou/FrameFlow/internal/streams"
	"github.com/liuhengloveyou/FrameFlow/pkg/ivideon"
)

func Init() {
	streams.HandleFunc("ivideon", ivideon.Dial)
}
