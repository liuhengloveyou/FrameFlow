//go:build !cgo

package ffdecode

func newDecoderImpl(codec Codec, opt Options) (decoderImpl, error) {
	_, _ = codec, opt
	return nil, ErrCGODisabled
}
