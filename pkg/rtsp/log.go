package rtsp

import "github.com/rs/zerolog"

// package logger（由 internal/rtsp Init 注入）。
var log = zerolog.Nop()

func SetLogger(l zerolog.Logger) {
	log = l
}
