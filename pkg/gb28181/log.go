package gb28181

import "github.com/rs/zerolog"

// package logger for media ingest diagnostics (set from internal Init).
var log = zerolog.Nop()

func SetLogger(l zerolog.Logger) {
	log = l
}
