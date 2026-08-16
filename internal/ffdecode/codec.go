package ffdecode

import "errors"

// Codec selects the libavcodec decoder (Annex-B input).
type Codec int

const (
	CodecH264 Codec = iota
	CodecH265
)

var (
	// ErrCGODisabled is returned when this package is built with CGO_ENABLED=0.
	ErrCGODisabled = errors.New("ffdecode: requires CGO_ENABLED=1 and bundled/system FFmpeg libraries")

	// ErrNoFrame means this sample produced no picture.
	ErrNoFrame = errors.New("ffdecode: no complete frame yet")

	// ErrOpenFailed indicates missing codec or avcodec_open2 failure.
	ErrOpenFailed = errors.New("ffdecode: failed to open libavcodec decoder")

	// ErrClosed is returned after [Decoder.Close].
	ErrClosed = errors.New("ffdecode: decoder closed")
)
