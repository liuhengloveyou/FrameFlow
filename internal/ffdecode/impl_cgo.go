//go:build cgo

package ffdecode

/*
#cgo darwin pkg-config: libavcodec libavutil libswscale libswresample

#cgo linux,amd64 CFLAGS: -I${SRCDIR}/../../lib/ffmpeg/linux/amd64/include
#cgo linux,amd64 LDFLAGS: -L${SRCDIR}/../../lib/ffmpeg/linux/amd64/lib -Wl,-rpath,\$ORIGIN/lib -lavcodec -lavutil -lswscale -lswresample
#cgo linux,arm64 CFLAGS: -I${SRCDIR}/../../lib/ffmpeg/linux/arm64/include
#cgo linux,arm64 LDFLAGS: -L${SRCDIR}/../../lib/ffmpeg/linux/arm64/lib -Wl,-rpath,\$ORIGIN/lib -lavcodec -lavutil -lswscale -lswresample

#include <libavutil/mem.h>
#include <libswresample/swresample.h>
#include <stdlib.h>
#include "ffmpeg_decode.h"

// 引用符号，避免 --as-needed 丢掉 -lswresample。
// libavcodec 对它的传递依赖不会走可执行文件的 RUNPATH。
__attribute__((used)) static void *const ffdecode_keep_swresample = (void *)swr_alloc;
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"
)

type cgoDecoder struct {
	p *C.ffdecode_ctx
}

func newDecoderImpl(codec Codec, opt Options) (decoderImpl, error) {
	var p *C.ffdecode_ctx
	maxw := C.int(opt.MaxOutWidth)
	if maxw < 0 {
		maxw = 0
	}
	stream := opt.Streaming
	switch codec {
	case CodecH264:
		if stream {
			p = C.ffdecode_open_h264_stream(maxw)
		} else {
			p = C.ffdecode_open_h264_maxw(maxw)
		}
	case CodecH265:
		if stream {
			p = C.ffdecode_open_hevc_stream(maxw)
		} else {
			p = C.ffdecode_open_hevc_maxw(maxw)
		}
	default:
		return nil, fmt.Errorf("ffdecode: unknown codec %v", codec)
	}
	if p == nil {
		return nil, fmt.Errorf("%w", ErrOpenFailed)
	}
	return &cgoDecoder{p: p}, nil
}

func (d *cgoDecoder) close() {
	if d.p != nil {
		C.ffdecode_close(d.p)
		d.p = nil
	}
}

func (d *cgoDecoder) decode(data []byte) (*Frame, error) {
	if d.p == nil {
		return nil, errors.New("ffdecode: decoder closed")
	}
	if len(data) == 0 {
		return nil, ErrNoFrame
	}
	var out *C.uint8_t
	var w, h C.int
	base := (*C.uint8_t)(unsafe.Pointer(&data[0]))
	ret := C.ffdecode_feed(d.p, base, C.size_t(len(data)), &out, &w, &h)
	if ret < 0 {
		return nil, fmt.Errorf("ffdecode: libavcodec error %d", int(ret))
	}
	return frameFromC(out, w, h, ret == 0)
}

func (d *cgoDecoder) decodeStream(data []byte, wantFrame bool) (*Frame, error) {
	if d.p == nil {
		return nil, errors.New("ffdecode: decoder closed")
	}
	if len(data) == 0 {
		return nil, ErrNoFrame
	}
	var out *C.uint8_t
	var w, h C.int
	want := C.int(0)
	if wantFrame {
		want = 1
	}
	base := (*C.uint8_t)(unsafe.Pointer(&data[0]))
	ret := C.ffdecode_feed_stream(d.p, base, C.size_t(len(data)), want, &out, &w, &h)
	if ret < 0 {
		return nil, fmt.Errorf("ffdecode: libavcodec error %d", int(ret))
	}
	if !wantFrame {
		if out != nil {
			C.av_free(unsafe.Pointer(out))
		}
		return nil, nil
	}
	return frameFromC(out, w, h, ret == 0)
}

// frameFromC 把 C 侧 RGB 缓冲转换为 [Frame] 并释放 C 缓冲；未取到帧时返回 [ErrNoFrame]。
func frameFromC(out *C.uint8_t, w, h C.int, gotFrame bool) (*Frame, error) {
	if !gotFrame || out == nil || w <= 0 || h <= 0 {
		if out != nil {
			C.av_free(unsafe.Pointer(out))
		}
		return nil, ErrNoFrame
	}
	n := int(w) * int(h) * 3
	rgb := C.GoBytes(unsafe.Pointer(out), C.int(n))
	C.av_free(unsafe.Pointer(out))
	return &Frame{Width: int(w), Height: int(h), RGB: rgb}, nil
}
