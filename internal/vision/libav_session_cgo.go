//go:build cgo

package vision

import (
	"errors"

	"github.com/liuhengloveyou/FrameFlow/internal/ffdecode"
	"github.com/liuhengloveyou/FrameFlow/pkg/core"
)

type libavVisionSession struct {
	h264, h265 *ffdecode.Decoder
}

func newLibavVisionSession() *libavVisionSession { return &libavVisionSession{} }

func (s *libavVisionSession) close() {
	if s == nil {
		return
	}
	if s.h264 != nil {
		s.h264.Close()
		s.h264 = nil
	}
	if s.h265 != nil {
		s.h265.Close()
		s.h265 = nil
	}
}

func (s *libavVisionSession) decoder(codecName string) (*ffdecode.Decoder, error) {
	switch codecName {
	case core.CodecH264:
		if s.h264 == nil {
			d, err := ffdecode.New(ffdecode.CodecH264)
			if err != nil {
				return nil, err
			}
			s.h264 = d
		}
		return s.h264, nil
	case core.CodecH265:
		if s.h265 == nil {
			d, err := ffdecode.New(ffdecode.CodecH265)
			if err != nil {
				return nil, err
			}
			s.h265 = d
		}
		return s.h265, nil
	default:
		return nil, errors.New("vision: libav only for H264/H265")
	}
}

// tryDecodeAnnexB：ffdecode 每次 feed 自包含（AU 解码 + drain + 重置），直接喂完整样本即可。
func (s *libavVisionSession) tryDecodeAnnexB(codecName string, annexB []byte) (rgb []byte, w, h int, ok bool, err error) {
	if s == nil || len(annexB) == 0 {
		return nil, 0, 0, false, nil
	}
	d, err := s.decoder(codecName)
	if err != nil {
		return nil, 0, 0, false, err
	}
	fr, err := d.Decode(annexB)
	if err != nil {
		if errors.Is(err, ffdecode.ErrNoFrame) {
			return nil, 0, 0, false, nil
		}
		return nil, 0, 0, false, err
	}
	return fr.RGB, fr.Width, fr.Height, true, nil
}
