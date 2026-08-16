package vision

import (
	"image"
	"math"
	"runtime"

	ort "github.com/yalue/onnxruntime_go"
	xdraw "golang.org/x/image/draw"
)

// letterboxScratch 复用预处理缓冲，避免实时视频每帧分配（对齐 MediaAI InferenceBuffer）。
type letterboxScratch struct {
	blob    []float32
	srcRGBA *image.RGBA
	resized *image.RGBA
}

func (s *letterboxScratch) ensureBlob(n int) []float32 {
	if cap(s.blob) < n {
		s.blob = make([]float32, n)
	} else {
		s.blob = s.blob[:n]
	}
	return s.blob
}

func ensureExactRGBA(dst **image.RGBA, w, h int) *image.RGBA {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	if *dst == nil || (*dst).Bounds().Dx() != w || (*dst).Bounds().Dy() != h {
		*dst = image.NewRGBA(image.Rect(0, 0, w, h))
	}
	return *dst
}

func fillRGBAFromRGB888(dst *image.RGBA, rgb []byte, w, h int) {
	pix := dst.Pix
	stride := dst.Stride
	for y := 0; y < h; y++ {
		di := y * stride
		si := y * w * 3
		for x := 0; x < w; x++ {
			pix[di+0] = rgb[si+0]
			pix[di+1] = rgb[si+1]
			pix[di+2] = rgb[si+2]
			pix[di+3] = 255
			di += 4
			si += 3
		}
	}
}

// letterboxInto 等比缩放 + 居中 pad(114) → NCHW float32 /255，写入复用缓冲。
// 与 Ultralytics / MediaAI letterBoxToBlob 对齐一致。
func letterboxInto(src *image.RGBA, dst int, s *letterboxScratch) (blob []float32, padX, padY, scale float32) {
	ow := src.Rect.Dx()
	oh := src.Rect.Dy()
	dstW, dstH := dst, dst
	scale = float32(math.Min(float64(dstH)/float64(oh), float64(dstW)/float64(ow)))
	newW := int(math.Round(float64(ow) * float64(scale)))
	newH := int(math.Round(float64(oh) * float64(scale)))
	newW = clampInt(newW, 1, dstW)
	newH = clampInt(newH, 1, dstH)

	padLeft := (dstW - newW) / 2
	padTop := (dstH - newH) / 2
	padX = float32(padLeft)
	padY = float32(padTop)

	resized := ensureExactRGBA(&s.resized, newW, newH)
	xdraw.ApproxBiLinear.Scale(resized, resized.Bounds(), src, src.Bounds(), xdraw.Src, nil)

	const padNorm = float32(114.0 / 255.0)
	nPix := dstH * dstW * 3
	blob = s.ensureBlob(nPix)
	for i := range blob {
		blob[i] = padNorm
	}
	rCh := blob[0 : dstH*dstW]
	gCh := blob[dstH*dstW : 2*dstH*dstW]
	bCh := blob[2*dstH*dstW : 3*dstH*dstW]
	const inv255 = 1.0 / 255.0
	rp := resized.Pix
	rs := resized.Stride
	for y := 0; y < newH; y++ {
		dy := y + padTop
		row := y * rs
		for x := 0; x < newW; x++ {
			dx := x + padLeft
			idx := dy*dstW + dx
			si := row + x*4
			rCh[idx] = float32(rp[si+0]) * inv255
			gCh[idx] = float32(rp[si+1]) * inv255
			bCh[idx] = float32(rp[si+2]) * inv255
		}
	}
	return blob, padX, padY, scale
}

// letterboxRGB888Into 视频帧快路径：RGB888 平面 → letterbox blob，无每帧新建 image。
func letterboxRGB888Into(rgb []byte, ow, oh, dst int, s *letterboxScratch) (blob []float32, padX, padY, scale float32) {
	src := ensureExactRGBA(&s.srcRGBA, ow, oh)
	fillRGBAFromRGB888(src, rgb, ow, oh)
	return letterboxInto(src, dst, s)
}

// newYOLOSession 创建带图优化与线程限制的 ORT session（参考 MediaAI：低延迟优先）。
func newYOLOSession(modelPath, inName, outName string) (*ort.DynamicAdvancedSession, error) {
	opts, err := ort.NewSessionOptions()
	if err != nil {
		return ort.NewDynamicAdvancedSession(modelPath, []string{inName}, []string{outName}, nil)
	}
	defer func() { _ = opts.Destroy() }()

	_ = opts.SetIntraOpNumThreads(1)
	_ = opts.SetInterOpNumThreads(1)
	if runtime.GOOS == "darwin" {
		// MediaAI：Apple 上 BASIC + 关 MemPattern 更稳
		_ = opts.SetGraphOptimizationLevel(ort.GraphOptimizationLevelEnableBasic)
		_ = opts.SetMemPattern(false)
	} else {
		_ = opts.SetGraphOptimizationLevel(ort.GraphOptimizationLevelEnableAll)
	}
	return ort.NewDynamicAdvancedSession(modelPath, []string{inName}, []string{outName}, opts)
}
