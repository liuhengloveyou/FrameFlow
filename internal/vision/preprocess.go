package vision

import (
	"image"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

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
// macOS 上默认追加 CoreML EP（Apple GPU/ANE）：yolo26l@640 单帧从 ~226ms(CPU 单线程)
// 降到数十毫秒。FRAMEFLOW_YOLO_EP=cpu|coreml|cuda|... 可强制指定或禁用。
// 注意：CoreML 首次加载模型有编译缓存开销（秒级），只在建 session 时发生一次。
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
	applyBestExecutionProvider(opts, modelPath)
	return ort.NewDynamicAdvancedSession(modelPath, []string{inName}, []string{outName}, opts)
}

// applyBestExecutionProvider 按平台追加最优推理加速后端；失败只警告并回退 CPU，
// 不影响主流程（CPU 路径已由调用方配好线程与图优化）。
//
//   - darwin（Apple Silicon / Intel）：CoreML（GPU/ANE）
//   - linux/windows 且检测到 NVIDIA GPU：CUDA（需 onnxruntime-gpu 版库，否则自动回退）
//   - 其他：CPU
//
// FRAMEFLOW_YOLO_EP 可强制指定：cpu（禁用加速）/ coreml / cuda / dml / openvino / tensorrt。
func applyBestExecutionProvider(opts *ort.SessionOptions, modelPath string) {
	if opts == nil {
		return
	}
	force := strings.ToLower(strings.TrimSpace(os.Getenv("FRAMEFLOW_YOLO_EP")))
	if force == "cpu" {
		return
	}
	name := force
	if name == "" {
		switch runtime.GOOS {
		case "darwin":
			name = "CoreMLExecutionProvider"
		default:
			if hasNvidiaGPU() {
				name = "CUDAExecutionProvider"
			}
		}
	}
	if name == "" {
		return // 无可用加速后端：纯 CPU
	}
	if err := opts.AppendExecutionProvider(name, nil); err != nil {
		log.Warn().Err(err).Str("ep", name).Str("model", filepath.Base(modelPath)).
			Msg("[vision] 推理加速后端追加失败，回退 CPU")
		return
	}
	log.Info().Str("ep", name).Str("model", filepath.Base(modelPath)).
		Bool("forced", force != "").Msg("[vision] 推理加速后端已启用")
}

// hasNvidiaGPU 粗判机器上是否有 NVIDIA GPU（nvidia-smi 或 /dev/nvidia0）。
// 只用于「要不要尝试 CUDA EP」的决策；库不带 CUDA 时追加会失败并自动回退 CPU。
func hasNvidiaGPU() bool {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		return false
	}
	if _, err := exec.LookPath("nvidia-smi"); err == nil {
		return true
	}
	if _, err := os.Stat("/dev/nvidia0"); err == nil {
		return true
	}
	return false
}
