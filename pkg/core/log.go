package core

import "github.com/rs/zerolog"

// package logger (wired from streams.Init via SetLogger).
var log = zerolog.Nop()

// SetLogger installs the package logger used for AVRing diagnostics.
func SetLogger(l zerolog.Logger) {
	log = l
}
