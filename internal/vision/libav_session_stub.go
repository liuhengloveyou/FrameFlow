//go:build !cgo

package vision

type libavVisionSession struct{}

func newLibavVisionSession() *libavVisionSession { return &libavVisionSession{} }

func (s *libavVisionSession) close() {}

// tryDecodeAnnexB 在非 CGO 构建下恒为未解码，vision 将走 FFmpeg 子进程路径。
func (s *libavVisionSession) tryDecodeAnnexB(codecName string, annexB []byte) (rgb []byte, w, h int, ok bool, err error) {
	_, _ = codecName, annexB
	return nil, 0, 0, false, nil
}
