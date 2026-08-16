package ffdecode

// Frame is one decoded picture in interleaved RGB888 (row-major, len = Width*Height*3).
type Frame struct {
	RGB    []byte
	Width  int
	Height int
}

type decoderImpl interface {
	decode(data []byte) (*Frame, error)
	close()
}

// Decoder decodes H.264 / H.265 Annex-B using libavcodec (via CGO). Not safe for concurrent use.
type Decoder struct {
	impl decoderImpl
}

// Options configures the native decoder (CGO build). Stub builds ignore all fields.
type Options struct {
	// MaxOutWidth <= 0: RGB matches decoded frame size. >0 scales down when wider (height proportional, like scale=W:-1).
	MaxOutWidth int
}

// NewWithOptions creates a decoder for the given codec and options.
// Requires CGO and pkg-config for libavcodec, libavutil, libswscale.
func NewWithOptions(codec Codec, opt Options) (*Decoder, error) {
	impl, err := newDecoderImpl(codec, opt)
	if err != nil {
		return nil, err
	}
	return &Decoder{impl: impl}, nil
}

// New creates a decoder with default options (full-resolution RGB after decode).
func New(codec Codec) (*Decoder, error) {
	return NewWithOptions(codec, Options{})
}

// Decode 一次性解码完整 Annex-B 样本（建议含参数集 + 自 IDR 起可独立解码的片段），
// 返回本轮最后一帧 RGB。无帧时返回 [ErrNoFrame]。
func (d *Decoder) Decode(data []byte) (*Frame, error) {
	if d.impl == nil {
		return nil, ErrClosed
	}
	return d.impl.decode(data)
}

// Close releases native resources and clears any buffered Annex-B data.
func (d *Decoder) Close() {
	if d.impl == nil {
		return
	}
	d.impl.close()
	d.impl = nil
}
