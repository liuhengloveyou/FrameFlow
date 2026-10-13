//go:build cgo

package vision

import (
	"errors"

	"github.com/liuhengloveyou/FrameFlow/internal/ffdecode"
	"github.com/liuhengloveyou/FrameFlow/pkg/core"
)

// libavVisionSession 一路流的解码会话：H264/H265 各一个常驻解码器。
//
// 解码器以 [ffdecode.Options.Streaming] 打开，两种喂入方式共用一个上下文：
//   - tryDecodeStream：增量 AU（持久状态，成本 ∝ 新增帧数）——interval 取帧主路径；
//   - tryDecodeAnnexB：自包含样本（feed 后 drain + reset，上下文回到干净态）——keyframe 路径。
//
// 旧实现每拍都把整个 GOP 从 IDR 重放一遍（208ms@1080p/5s GOP），这是实时性差的主因。
type libavVisionSession struct {
	h264, h265 *ffdecode.Decoder
}

func newLibavVisionSession() *libavVisionSession { return &libavVisionSession{} }

func (s *libavVisionSession) close() { s.reset() }

// reset 丢弃解码器状态（下次用时重新打开）。解码出错后调用：
// 之后必须从 IDR（自包含样本）重新对齐，采样端应同时 RequestResync。
func (s *libavVisionSession) reset() {
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
			d, err := ffdecode.NewWithOptions(ffdecode.CodecH264, ffdecode.Options{Streaming: true})
			if err != nil {
				return nil, err
			}
			s.h264 = d
		}
		return s.h264, nil
	case core.CodecH265:
		if s.h265 == nil {
			d, err := ffdecode.NewWithOptions(ffdecode.CodecH265, ffdecode.Options{Streaming: true})
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

// tryDecodeAnnexB：自包含样本（含参数集 + 自 IDR 起可独立解码的片段）一次性解码。
func (s *libavVisionSession) tryDecodeAnnexB(codecName string, annexB []byte) (rgb []byte, w, h int, ok bool, err error) {
	if s == nil || len(annexB) == 0 {
		return nil, 0, 0, false, nil
	}
	d, err := s.decoder(codecName)
	if err != nil {
		return nil, 0, 0, false, err
	}
	fr, err := d.Decode(annexB)
	return frameResult(fr, err)
}

// tryDecodeStream：增量样本送入持久解码器，返回最后一帧。
// wantFrame=false 时只推进解码状态（不分配/拷贝 RGB），用于「这一拍不需要画面」的场景。
func (s *libavVisionSession) tryDecodeStream(codecName string, annexB []byte, wantFrame bool) (rgb []byte, w, h int, ok bool, err error) {
	if s == nil || len(annexB) == 0 {
		return nil, 0, 0, false, nil
	}
	d, err := s.decoder(codecName)
	if err != nil {
		return nil, 0, 0, false, err
	}
	fr, err := d.DecodeStream(annexB, wantFrame)
	if err != nil {
		return nil, 0, 0, false, err
	}
	if !wantFrame || fr == nil {
		return nil, 0, 0, false, nil
	}
	return fr.RGB, fr.Width, fr.Height, true, nil
}

// frameResult 统一 [ffdecode.Decoder] 返回值语义：ErrNoFrame 不算错误。
func frameResult(fr *ffdecode.Frame, decErr error) (rgb []byte, w, h int, ok bool, err error) {
	if decErr != nil {
		if errors.Is(decErr, ffdecode.ErrNoFrame) {
			return nil, 0, 0, false, nil
		}
		return nil, 0, 0, false, decErr
	}
	if fr == nil {
		return nil, 0, 0, false, nil
	}
	return fr.RGB, fr.Width, fr.Height, true, nil
}
