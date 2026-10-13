package ffdecode

// Frame is one decoded picture in interleaved RGB888 (row-major, len = Width*Height*3).
type Frame struct {
	RGB    []byte
	Width  int
	Height int
}

type decoderImpl interface {
	decode(data []byte) (*Frame, error)
	decodeStream(data []byte, wantFrame bool) (*Frame, error)
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

	// Streaming 持久解码：DecodeStream 之间不 drain、不 reset，参考帧状态跨调用保留。
	// 配合「只送增量 AU」的采样器，单次解码成本 ∝ 本次新增帧数（而非整个 GOP），
	// 是实时取帧链路的关键；代价是丢包后需等下一个 IDR 才能彻底清洁（见 DecodeStream 注释）。
	Streaming bool
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

// DecodeStream 增量解码（需 [Options.Streaming]）：把 data 中的访问单元依次送入持久解码器，
// 返回本轮最后一帧 RGB；wantFrame=false 时只推进解码状态、不做 RGB 转换（返回 nil, nil）。
//
// 无帧时返回 [ErrNoFrame]；解码错误会内部重置解码器——此时必须从下一个 IDR（自包含样本）
// 重新对齐，调用方应先重新同步采样端再继续。
func (d *Decoder) DecodeStream(data []byte, wantFrame bool) (*Frame, error) {
	if d.impl == nil {
		return nil, ErrClosed
	}
	return d.impl.decodeStream(data, wantFrame)
}

// Close releases native resources and clears any buffered Annex-B data.
func (d *Decoder) Close() {
	if d.impl == nil {
		return
	}
	d.impl.close()
	d.impl = nil
}
