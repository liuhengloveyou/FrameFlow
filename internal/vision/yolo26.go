package vision

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	ort "github.com/yalue/onnxruntime_go"

	"github.com/liuhengloveyou/FrameFlow/pkg/fferrors"
)

// YOLO ONNX 后处理支持两类常见导出：
// 1) Ultralytics 检测头 float32 [B, 4+nc(+nm), A] 或 [B, A, 4+nc(+nm)]（如 [1,84,8400]），
//    前 4 维为 xywh（letterbox 输入像素空间），其后为各类别 logit；与 ops.non_max_suppression 一致用各类别 max(logit) 与 conf 比较。
// 2) 端到端（图内 NMS）float32 [..., N, 6] 或 [..., 6, N]，每框 x1,y1,x2,y2,conf,class_id（同上坐标空间）。
// 预处理：等比缩放 + 居中 pad、RGB、CHW、/255、pad 常数 114/255（与常见 Ultralytics 导出兼容）。

const (
	defaultYoloInputSize = 640
	defaultConf          = float32(0.25)
	defaultNMSIoU        = float32(0.45)
	defaultMaxDet        = 100
	maxRawCandidates     = 200_000 // 防止异常张量拖垮内存
)

var (
	yoloMu     sync.Mutex
	yoloByPath = map[string]*yoloDetector{} // 绝对路径 → session；多算法可同时常驻，按绑定选用
	ortInitMu  sync.Once
	ortInitErr error

	ortLibMu   sync.RWMutex
	ortLibPath string // ORT 动态库路径（环境变量 / 自动发现，非 YAML）
)

// SetORTLibPath 设置 ONNX Runtime 共享库路径；应在首次 ensureORT 之前调用（如 vision.Init）。
func SetORTLibPath(p string) {
	p = strings.TrimSpace(p)
	if p != "" && !filepath.IsAbs(p) {
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
	}
	ortLibMu.Lock()
	ortLibPath = p
	ortLibMu.Unlock()
}

func configuredORTLib() string {
	ortLibMu.RLock()
	defer ortLibMu.RUnlock()
	return ortLibPath
}

func resolveORTLib(sharedLib string) string {
	if p := strings.TrimSpace(sharedLib); p != "" {
		if !filepath.IsAbs(p) {
			if abs, err := filepath.Abs(p); err == nil {
				return abs
			}
		}
		return p
	}
	if p := configuredORTLib(); p != "" {
		return p
	}
	if e := strings.TrimSpace(os.Getenv("ONNXRUNTIME_SHARED_LIBRARY_PATH")); e != "" {
		if !filepath.IsAbs(e) {
			if abs, err := filepath.Abs(e); err == nil {
				return abs
			}
		}
		return e
	}
	return ""
}

func ensureORT(sharedLib string) error {
	ortInitMu.Do(func() {
		p := resolveORTLib(sharedLib)
		if p != "" {
			ort.SetSharedLibraryPath(p)
		}
		exists := p != "" && fileExists(p)
		log.Info().
			Str("ort_lib", p).
			Bool("file_exists", exists).
			Str("cwd", mustGetwd()).
			Msg("[vision] onnxruntime: initializing")
		ortInitErr = ort.InitializeEnvironment()
		if ortInitErr != nil {
			hint := p
			if hint == "" {
				hint = "(未配置，默认 onnxruntime.so)"
			}
			ortInitErr = fmt.Errorf(
				"加载 ONNX Runtime 失败（%s）：%w；请确认 vision.ort_lib 指向本机库（lib/libonnxruntime.{dylib|so}，先 fetch 再 ./scripts/prepare-dev-libs.sh）或设置 ONNXRUNTIME_SHARED_LIBRARY_PATH",
				hint, ortInitErr,
			)
			log.Error().Err(ortInitErr).Str("ort_lib", p).Msg("[vision] onnxruntime: init failed")
			return
		}
		log.Info().Str("ort_lib", p).Msg("[vision] onnxruntime: ready")
	})
	return ortInitErr
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return wd
}

type yoloDetector struct {
	session   *ort.DynamicAdvancedSession
	modelPath string
	labels    []string
	inName    string
	outName   string
	conf      float32
	nmsIoU    float32
	skipNMS   bool
	maxDet    int
	size      int
	scratch   letterboxScratch // 复用 letterbox 缓冲（对齐 MediaAI InferenceBuffer）
	runMu     sync.Mutex       // 保护 scratch + session.Run（ORT session 非线程安全）
}

func absModelPath(modelPath string) string {
	modelPath = strings.TrimSpace(modelPath)
	if modelPath == "" {
		return ""
	}
	if !filepath.IsAbs(modelPath) {
		if abs, err := filepath.Abs(modelPath); err == nil {
			return abs
		}
	}
	return modelPath
}

func absLabelsPath(labelsPath string) string {
	labelsPath = strings.TrimSpace(labelsPath)
	if labelsPath == "" {
		return ""
	}
	if !filepath.IsAbs(labelsPath) {
		if abs, err := filepath.Abs(labelsPath); err == nil {
			labelsPath = abs
		}
	}
	if IsForbiddenModelPath(labelsPath) {
		log.Warn().Str("path", labelsPath).Str("models_dir", ModelsDataDir()).Msg("[vision] yolo26: labels outside models_dir, ignored")
		return ""
	}
	return labelsPath
}

// EnsureORTReady 仅预加载 ONNX Runtime，不加载任何模型权重。
// 模型必须由算法绑定（DB）指定路径后再 LoadYOLOModel。
func EnsureORTReady() {
	if err := ensureORT(""); err != nil {
		log.Warn().Err(err).Msg("[vision] yolo26: onnxruntime preload")
	}
}

// LoadYOLOModel 按绝对/相对路径加载（或刷新参数）一个上传的 ONNX，并入进程缓存。
// 不同算法可各自 Load，互不覆盖；流检测按绑定的 ModelPath 取用对应 session。
// inputSize≤0：优先读 ONNX 张量，否则默认 640。nmsIoU≥1 关闭 NMS。
func LoadYOLOModel(modelPath, labelsPath, ortLib string, conf float32, inputSize int, nmsIoU float32, maxDet int) error {
	if p := strings.TrimSpace(ortLib); p != "" {
		SetORTLibPath(p)
	}
	modelPath = absModelPath(modelPath)
	if modelPath == "" {
		EnsureORTReady()
		return fmt.Errorf("模型路径为空")
	}
	if IsForbiddenModelPath(modelPath) {
		log.Error().Str("path", modelPath).Str("required_root", ModelsDataDir()).
			Msg("[vision] yolo26: model path must be under models_dir")
		return fmt.Errorf("模型路径必须在 models_dir 下")
	}
	if !fileExists(modelPath) {
		log.Info().Str("path", modelPath).Msg("[vision] yolo26: model file missing")
		return fmt.Errorf("模型文件不存在：%s", modelPath)
	}
	labelsPath = absLabelsPath(labelsPath)
	if conf <= 0 {
		conf = defaultConf
	}
	skipNMS := nmsIoU >= 1
	if nmsIoU <= 0 {
		nmsIoU = defaultNMSIoU
	}
	if maxDet <= 0 {
		maxDet = defaultMaxDet
	}

	// 已缓存：只刷新参数，避免每帧读 ONNX 元数据并误报 configured=640
	yoloMu.Lock()
	if d := yoloByPath[modelPath]; d != nil && d.session != nil {
		d.conf = conf
		d.nmsIoU = nmsIoU
		d.skipNMS = skipNMS
		d.maxDet = maxDet
		yoloMu.Unlock()
		return nil
	}
	yoloMu.Unlock()

	if err := ensureORT(ortLib); err != nil {
		log.Error().Err(err).Msg("[vision] yolo26: onnxruntime init")
		return err
	}
	// inputSize≤0 = 未指定；由 ONNX 决定，勿先填 640 再告警
	inputSize = resolveYoloInputSize(modelPath, inputSize)

	var labels []string
	if lp := labelsPath; lp != "" {
		var err error
		labels, err = loadLabels(lp)
		if err != nil {
			log.Warn().Err(err).Str("path", lp).Msg("[vision] yolo26: labels load failed, continuing with empty class names")
			labels = nil
		}
	} else {
		log.Warn().Str("model", modelPath).Msg("[vision] yolo26: no labels file, class names will be empty")
	}
	inName, outName, err := pickYoloIONames(modelPath)
	if err != nil {
		log.Error().Err(err).Msg("[vision] yolo26: io names")
		return err
	}
	sess, err := newYOLOSession(modelPath, inName, outName)
	if err != nil {
		log.Error().Err(err).Str("model", modelPath).Msg("[vision] yolo26: session")
		return err
	}
	det := &yoloDetector{
		session:   sess,
		modelPath: modelPath,
		labels:    labels,
		inName:    inName,
		outName:   outName,
		conf:      conf,
		nmsIoU:    nmsIoU,
		skipNMS:   skipNMS,
		maxDet:    maxDet,
		size:      inputSize,
	}
	yoloMu.Lock()
	if existing := yoloByPath[modelPath]; existing != nil && existing.session != nil {
		existing.conf = conf
		existing.nmsIoU = nmsIoU
		existing.skipNMS = skipNMS
		existing.maxDet = maxDet
		if inputSize > 0 {
			existing.size = inputSize
		}
		yoloMu.Unlock()
		det.runMu.Lock()
		if det.session != nil {
			_ = det.session.Destroy()
			det.session = nil
		}
		det.runMu.Unlock()
		return nil
	}
	nCached := len(yoloByPath) + 1
	yoloByPath[modelPath] = det
	yoloMu.Unlock()
	log.Info().
		Str("model", modelPath).
		Int("labels", len(labels)).
		Int("input", inputSize).
		Float64("conf", float64(conf)).
		Bool("nms_off", skipNMS).
		Float64("nms_iou", float64(nmsIoU)).
		Int("max_det", maxDet).
		Int("cached_models", nCached).
		Msg("[vision] yolo26: ready (bound model cache)")
	return nil
}

// InitYOLO26 兼容旧调用：等价于 LoadYOLOModel（写入缓存，不卸载其它模型）。
func InitYOLO26(modelPath, labelsPath, ortLib string, conf float32, inputSize int, nmsIoU float32, maxDet int) {
	_ = LoadYOLOModel(modelPath, labelsPath, ortLib, conf, inputSize, nmsIoU, maxDet)
}

// UnloadYOLOModel 从缓存卸载指定 ONNX（本地文件删除后调用）。
func UnloadYOLOModel(modelPath string) {
	modelPath = absModelPath(modelPath)
	if modelPath == "" {
		return
	}
	yoloMu.Lock()
	old := yoloByPath[modelPath]
	delete(yoloByPath, modelPath)
	yoloMu.Unlock()
	if old == nil {
		return
	}
	old.runMu.Lock()
	if old.session != nil {
		_ = old.session.Destroy()
		old.session = nil
	}
	old.runMu.Unlock()
	log.Info().Str("model", modelPath).Msg("[vision] yolo26: unloaded")
}

// HasYOLOModel 指定上传模型是否已在缓存中。
func HasYOLOModel(modelPath string) bool {
	modelPath = absModelPath(modelPath)
	if modelPath == "" {
		return false
	}
	yoloMu.Lock()
	defer yoloMu.Unlock()
	d := yoloByPath[modelPath]
	return d != nil && d.session != nil
}

// HasYOLO 是否已有任意绑定模型在缓存中（非“系统默认模型”）。
func HasYOLO() bool {
	yoloMu.Lock()
	defer yoloMu.Unlock()
	return len(yoloByPath) > 0
}

// YOLOModelInfo 返回指定模型缓存的运行时参数。
func YOLOModelInfo(modelPath string) (conf float32, inputSize, labelCount int, ready bool) {
	modelPath = absModelPath(modelPath)
	yoloMu.Lock()
	defer yoloMu.Unlock()
	d := yoloByPath[modelPath]
	if d == nil || d.session == nil {
		return 0, 0, 0, false
	}
	return d.conf, d.size, len(d.labels), true
}

// YOLORuntimeInfo 兼容旧日志：返回缓存中任意一个模型的参数（无则未就绪）。
// 流检测请用 YOLOModelInfo(绑定 ModelPath)。
func YOLORuntimeInfo() (conf float32, inputSize, labelCount int, ready bool) {
	yoloMu.Lock()
	defer yoloMu.Unlock()
	for _, d := range yoloByPath {
		if d != nil && d.session != nil {
			return d.conf, d.size, len(d.labels), true
		}
	}
	return 0, 0, 0, false
}

// EnsureYOLO 若该路径尚未缓存则 Load；已缓存则 noop（不碰其它模型）。
func EnsureYOLO(modelPath, labelsPath, ortLib string, conf float32, inputSize int, nmsIoU float32, maxDet int) {
	if HasYOLOModel(modelPath) {
		return
	}
	_ = LoadYOLOModel(modelPath, labelsPath, ortLib, conf, inputSize, nmsIoU, maxDet)
}

// DetectJPEGOnce 对上传样张做一次完整检测（试跑）：优先复用绑定模型缓存，否则临时 session。
//
//  1. 解码图片  2) 解析模型输入尺寸  3) letterbox 预处理
//  4. ONNX 推理  5) 置信度过滤 + NMS 后处理
func DetectJPEGOnce(modelPath, labelsPath string, imageBytes []byte, conf, nmsIoU float32, maxDet, inputSize int) ([]VisionBBox, error) {
	modelPath = strings.TrimSpace(modelPath)
	if modelPath == "" {
		return nil, fferrors.ErrVisionYoloSkipped
	}
	if len(imageBytes) == 0 {
		return nil, fmt.Errorf("请上传测试图片")
	}
	if conf <= 0 {
		conf = defaultConf
	}
	if nmsIoU <= 0 {
		nmsIoU = defaultNMSIoU
	}
	if maxDet <= 0 {
		maxDet = defaultMaxDet
	}

	t0 := time.Now()

	// 1) 先处理图片：解码并转为 RGBA，确认可推理
	img, format, err := decodeImageBytes(imageBytes)
	if err != nil {
		log.Error().Err(err).Int("bytes", len(imageBytes)).Msg("[vision] detect: decode image")
		return nil, fmt.Errorf("图片解码失败：%w", err)
	}
	bounds := img.Bounds()
	ow, oh := bounds.Dx(), bounds.Dy()
	if ow < 1 || oh < 1 {
		return nil, fmt.Errorf("图片尺寸无效：%dx%d", ow, oh)
	}
	rgba := imageToRGBA(img)
	log.Info().
		Str("format", format).
		Int("w", ow).Int("h", oh).
		Int("bytes", len(imageBytes)).
		Dur("decode", time.Since(t0)).
		Msg("[vision] detect: image ready")

	// 2) 使用算法关联的上传模型：优先进缓存复用（与流检测同一套）
	tModel := time.Now()
	if err := LoadYOLOModel(modelPath, labelsPath, "", conf, inputSize, nmsIoU, maxDet); err != nil {
		return nil, err
	}
	yoloMu.Lock()
	d := yoloByPath[absModelPath(modelPath)]
	yoloMu.Unlock()
	if d == nil || d.session == nil {
		return nil, fferrors.ErrVisionYoloSkipped
	}
	log.Info().
		Str("model", absModelPath(modelPath)).
		Int("input", d.size).
		Str("in", d.inName).Str("out", d.outName).
		Int("labels", len(d.labels)).
		Float32("conf", conf).
		Dur("model_prep", time.Since(tModel)).
		Msg("[vision] detect: model ready")

	// 3–5) letterbox → 推理 → 后处理
	boxes, err := d.inferFromRGBA(rgba, ow, oh, conf)
	if err != nil {
		log.Error().Err(err).Msg("[vision] detect: infer")
		return nil, err
	}
	log.Info().
		Int("detections", len(boxes)).
		Int("src_w", ow).Int("src_h", oh).
		Int("model_in", d.size).
		Dur("total", time.Since(t0)).
		Msg("[vision] detect: ok")
	return boxes, nil
}

func decodeImageBytes(imageBytes []byte) (image.Image, string, error) {
	img, format, err := image.Decode(bytes.NewReader(imageBytes))
	if err == nil {
		return img, format, nil
	}
	// 部分 JPEG 仅 jpeg.Decode 可解析
	img, jerr := jpeg.Decode(bytes.NewReader(imageBytes))
	if jerr != nil {
		return nil, "", err
	}
	return img, "jpeg", nil
}

func imageToRGBA(img image.Image) *image.RGBA {
	bounds := img.Bounds()
	if rgba, ok := img.(*image.RGBA); ok && rgba.Rect.Min.Eq(bounds.Min) && rgba.Rect.Max.Eq(bounds.Max) {
		return rgba
	}
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)
	return rgba
}

func loadLabels(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	var out []string
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		out = append(out, ln)
	}
	if len(out) < 1 {
		return nil, fferrors.ErrVisionLabelsEmpty
	}
	return out, nil
}

func pickYoloIONames(modelPath string) (inName, outName string, err error) {
	ins, outs, e := ort.GetInputOutputInfo(modelPath)
	if e != nil {
		return "", "", e
	}
	if len(ins) == 0 || len(outs) == 0 {
		return "", "", fferrors.ErrVisionModelNoIO
	}
	inName = ins[0].Name
	// 优先端到端 [..., 6, N] / [..., N, 6]
	for _, o := range outs {
		if o.OrtValueType != ort.ONNXTypeTensor {
			continue
		}
		d := o.Dimensions
		if len(d) < 2 {
			continue
		}
		d1, d2 := d[len(d)-2], d[len(d)-1]
		has6 := d1 == 6 || d2 == 6
		other := int64(0)
		if d1 == 6 {
			other = d2
		} else if d2 == 6 {
			other = d1
		}
		if has6 && (other > 0 || other == -1) {
			return inName, o.Name, nil
		}
	}
	// Ultralytics 风格 [B, C, A] / [B, A, C]
	for _, o := range outs {
		if o.OrtValueType != ort.ONNXTypeTensor {
			continue
		}
		if _, _, _, _, ok := yoloDet3DLayout(o.Dimensions); ok {
			return inName, o.Name, nil
		}
	}
	return inName, outs[0].Name, nil
}

// yoloInputSizeFromModel 从 ONNX 输入张量读取空间尺寸（NCHW [N,C,H,W] 或 NHWC [N,H,W,C]）。
// 动态维（≤0）时返回 0。
func yoloInputSizeFromModel(modelPath string) int {
	ins, _, err := ort.GetInputOutputInfo(modelPath)
	if err != nil || len(ins) == 0 {
		return 0
	}
	d := ins[0].Dimensions
	if len(d) < 4 {
		return 0
	}
	var h, w int64
	switch {
	case d[1] == 1 || d[1] == 3: // NCHW
		h, w = d[2], d[3]
	case d[3] == 1 || d[3] == 3: // NHWC
		h, w = d[1], d[2]
	default:
		h, w = d[2], d[3]
	}
	if h <= 0 || w <= 0 {
		return 0
	}
	if h == w {
		return int(h)
	}
	// 非正方形少见；letterbox 侧取较大边
	if h > w {
		return int(h)
	}
	return int(w)
}

// resolveYoloInputSize 优先用模型图内尺寸，其次调用方指定，最后默认 640。
func resolveYoloInputSize(modelPath string, configured int) int {
	if sz := yoloInputSizeFromModel(modelPath); sz > 0 {
		if configured > 0 && configured != sz {
			log.Warn().
				Int("configured", configured).
				Int("model", sz).
				Str("path", modelPath).
				Msg("[vision] yolo input size: prefer ONNX tensor over config")
		}
		return sz
	}
	if configured > 0 {
		return configured
	}
	return defaultYoloInputSize
}

// detectBoxesJPEG 使用绑定指定的 YOLO ONNX；modelPath 空或未加载时返回 ErrVisionYoloSkipped。
// conf≤0 时使用该检测器缓存的默认阈值。
func detectBoxesJPEG(modelPath string, jpeg []byte, conf float32) ([]VisionBBox, error) {
	modelPath = absModelPath(modelPath)
	yoloMu.Lock()
	d := yoloByPath[modelPath]
	yoloMu.Unlock()
	if d == nil || d.session == nil {
		return nil, fferrors.ErrVisionYoloSkipped
	}
	return d.infer(jpeg, conf)
}

// detectBoxesRGB 视频快路径：RGB888 → letterbox（复用缓冲）→ 推理；忙则丢帧。
// conf≤0 时使用该检测器缓存的默认阈值。
func detectBoxesRGB(modelPath string, rgb []byte, w, h int, conf float32) ([]VisionBBox, error) {
	modelPath = absModelPath(modelPath)
	yoloMu.Lock()
	d := yoloByPath[modelPath]
	yoloMu.Unlock()
	if d == nil || d.session == nil {
		return nil, fferrors.ErrVisionYoloSkipped
	}
	if w < 1 || h < 1 || len(rgb) < w*h*3 {
		return nil, fmt.Errorf("vision: invalid rgb frame %dx%d len=%d", w, h, len(rgb))
	}
	return d.inferFromRGB888(rgb, w, h, true, conf)
}

func (d *yoloDetector) infer(jpegBytes []byte, conf float32) ([]VisionBBox, error) {
	img, _, err := decodeImageBytes(jpegBytes)
	if err != nil {
		log.Warn().Err(err).Int("jpeg_bytes", len(jpegBytes)).Msg("[vision] yolo26: image decode failed")
		return nil, err
	}
	return d.inferFromImage(img, conf)
}

func (d *yoloDetector) inferFromImage(img image.Image, conf float32) ([]VisionBBox, error) {
	bounds := img.Bounds()
	ow, oh := bounds.Dx(), bounds.Dy()
	if ow < 1 || oh < 1 {
		log.Info().Int("w", ow).Int("h", oh).Msg("[vision] yolo26: empty frame, skip")
		return []VisionBBox{}, nil
	}
	return d.inferFromRGBA(imageToRGBA(img), ow, oh, conf)
}

// inferFromRGB888：视频帧完整链路（复用缓冲；dropIfBusy 时检测器忙则丢帧）。
// conf≤0 时使用 d.conf。
func (d *yoloDetector) inferFromRGB888(rgb []byte, ow, oh int, dropIfBusy bool, conf float32) ([]VisionBBox, error) {
	if d.size < 1 {
		return nil, fmt.Errorf("模型输入尺寸无效：%d", d.size)
	}
	if dropIfBusy {
		if !d.runMu.TryLock() {
			return nil, fferrors.ErrVisionInferBusy
		}
	} else {
		d.runMu.Lock()
	}
	defer d.runMu.Unlock()

	tPre := time.Now()
	blob, padX, padY, scale := letterboxRGB888Into(rgb, ow, oh, d.size, &d.scratch)
	return d.runBlobLocked(blob, ow, oh, padX, padY, scale, time.Since(tPre), conf)
}

// inferFromRGBA：样张/JPEG 路径；letterbox 复用缓冲 → 推理 → 后处理。
// conf≤0 时使用 d.conf。
func (d *yoloDetector) inferFromRGBA(rgba *image.RGBA, ow, oh int, conf float32) ([]VisionBBox, error) {
	if rgba == nil || ow < 1 || oh < 1 {
		return []VisionBBox{}, nil
	}
	if d.size < 1 {
		return nil, fmt.Errorf("模型输入尺寸无效：%d", d.size)
	}
	d.runMu.Lock()
	defer d.runMu.Unlock()

	tPre := time.Now()
	blob, padX, padY, scale := letterboxInto(rgba, d.size, &d.scratch)
	return d.runBlobLocked(blob, ow, oh, padX, padY, scale, time.Since(tPre), conf)
}

// runBlobLocked 调用方须已持有 d.runMu。conf≤0 时使用 d.conf。
func (d *yoloDetector) runBlobLocked(blob []float32, ow, oh int, padX, padY, scale float32, preprocessDur time.Duration, conf float32) ([]VisionBBox, error) {
	if d.session == nil {
		return nil, fferrors.ErrVisionYoloSkipped
	}
	if conf <= 0 {
		conf = d.conf
	}
	log.Debug().
		Int("src_w", ow).Int("src_h", oh).
		Int("model_in", d.size).
		Float32("scale", scale).
		Float32("pad_x", padX).Float32("pad_y", padY).
		Float32("conf", conf).
		Dur("preprocess", preprocessDur).
		Msg("[vision] detect: preprocess (letterbox)")

	sh := ort.NewShape(1, 3, int64(d.size), int64(d.size))
	inTensor, err := ort.NewTensor(sh, blob)
	if err != nil {
		return nil, fmt.Errorf("构造输入张量失败：%w", err)
	}
	defer inTensor.Destroy()

	tRun := time.Now()
	outs := []ort.Value{nil}
	if err := d.session.Run([]ort.Value{inTensor}, outs); err != nil {
		return nil, fmt.Errorf("推理失败（输入 %dx%d→%d）：%w", ow, oh, d.size, err)
	}
	if outs[0] == nil {
		log.Warn().Msg("[vision] yolo26: session.Run returned nil output tensor")
		return []VisionBBox{}, nil
	}
	defer func() { _ = outs[0].Destroy() }()

	tf, ok := outs[0].(*ort.Tensor[float32])
	if !ok {
		return nil, fferrors.ErrVisionYoloOutputType
	}
	raw := tf.GetData()
	shape := tf.GetShape()
	runDur := time.Since(tRun)

	tPost := time.Now()
	var boxes []VisionBBox
	var postStats postprocessStats
	if b, ok := postprocessUltralyticsIfBCN(raw, shape, ow, oh, padX, padY, scale, conf, d.nmsIoU, d.skipNMS, d.maxDet, d.labels); ok {
		boxes = b
	} else {
		boxes, postStats = postprocessDetections(raw, shape, ow, oh, padX, padY, scale, conf, d.nmsIoU, d.skipNMS, d.maxDet, d.labels)
	}
	postDur := time.Since(tPost)
	logYoloInferSummary(ow, oh, d.size, shape, boxes, raw, postStats, conf, preprocessDur, runDur, postDur)
	logAIPipeline("_yolo", "2_yolo_post", true, map[string]any{
		"frame_w":        ow,
		"frame_h":        oh,
		"model_in":       d.size,
		"out_shape":      fmt.Sprintf("%v", shape),
		"boxes":          len(boxes),
		"conf":           conf,
		"max_slot_conf":  postStats.MaxRaw,
		"max_valid_conf": postStats.MaxValid,
		"pass_conf":      postStats.PassConf,
		"fail_geom":      postStats.FailGeom,
		"top":            summarizeBoxesTop(boxes, 3),
		"dur_pre_ms":     preprocessDur.Milliseconds(),
		"dur_run_ms":     runDur.Milliseconds(),
		"dur_post_ms":    postDur.Milliseconds(),
	})
	if conf >= 0.99 {
		logAIPipeline("_yolo", "2_yolo_warn_conf", true, map[string]any{
			"conf": conf,
			"hint": "置信度≈1 时后处理会丢掉几乎所有候选框",
		})
	}
	if len(boxes) == 0 && log.Debug().Enabled() {
		debugLogAllRawDetectionSlots(raw, shape, conf, d.labels)
	}
	return boxes, nil
}

// debugLogAllRawDetectionSlots 在无最终框且 log.level=debug 时，打印原始张量中每个槽位（含未过 vision.conf 的）。
// 端到端 [...,N,6]/[...,6,N]：一行 TSV 头 + 分块输出；Ultralytics BCN [B,C,A]：按 anchor 打 best 类概率（sigmoid）。
func debugLogAllRawDetectionSlots(raw []float32, shape []int64, confTh float32, labels []string) {
	n, interleaved, ok := parseDetectionLayout(shape, len(raw))
	if ok && interleaved && len(raw) >= n*6 {
		const chunk = 64
		log.Debug().Int("slots", n).Float32("conf_th", confTh).Msg("[vision] yolo26: [N,6] slots TSV columns: slot conf cls label x1 y1 x2 y2 pass_conf")
		for start := 0; start < n; start += chunk {
			end := min(start+chunk, n)
			var sb strings.Builder
			for i := start; i < end; i++ {
				o := i * 6
				x1, y1, x2, y2 := raw[o], raw[o+1], raw[o+2], raw[o+3]
				conf := raw[o+4]
				clsF := raw[o+5]
				cid := int(clsF)
				lbl := ""
				if cid >= 0 && cid < len(labels) {
					lbl = labels[cid]
				}
				pass := conf > confTh
				fmt.Fprintf(&sb, "%d\t%g\t%d\t%s\t%g\t%g\t%g\t%g\t%v\n", i, conf, cid, lbl, x1, y1, x2, y2, pass)
			}
			log.Debug().Int("slot_from", start).Int("slot_to", end-1).Str("tsv", sb.String()).Send()
		}
		return
	}
	if ok && !interleaved && len(raw) >= 6*n {
		const chunk = 64
		log.Debug().Int("slots", n).Float32("conf_th", confTh).Msg("[vision] yolo26: [6,N] slots TSV columns: slot conf cls label x1 y1 x2 y2 pass_conf")
		for start := 0; start < n; start += chunk {
			end := min(start+chunk, n)
			var sb strings.Builder
			for i := start; i < end; i++ {
				x1, y1, x2, y2 := raw[0*n+i], raw[1*n+i], raw[2*n+i], raw[3*n+i]
				conf := raw[4*n+i]
				clsF := raw[5*n+i]
				cid := int(clsF)
				lbl := ""
				if cid >= 0 && cid < len(labels) {
					lbl = labels[cid]
				}
				pass := conf > confTh
				fmt.Fprintf(&sb, "%d\t%g\t%d\t%s\t%g\t%g\t%g\t%g\t%v\n", i, conf, cid, lbl, x1, y1, x2, y2, pass)
			}
			log.Debug().Int("slot_from", start).Int("slot_to", end-1).Str("tsv", sb.String()).Send()
		}
		return
	}
	_, C, A, channelMajor, okBCN := yoloDet3DLayout(shape)
	if okBCN && len(raw) >= int(C*A) {
		nc := classCountForBCN(int(C), labels)
		log.Debug().
			Int("anchors", int(A)).
			Int("channels", int(C)).
			Int("nc", nc).
			Bool("channel_major", channelMajor).
			Float32("conf_th", confTh).
			Msg("[vision] yolo26: BCN layout anchors (best_cls_sigmoid per slot)")
		const chunk = 80
		Ci, Ai := int(C), int(A)
		for start := 0; start < Ai; start += chunk {
			end := min(start+chunk, Ai)
			var sb strings.Builder
			for anchor := start; anchor < end; anchor++ {
				bestLogit := float32(-1e9)
				bestCls := 0
				for k := 0; k < nc; k++ {
					ik := rawIndexBCN(channelMajor, Ci, Ai, 4+k, anchor)
					v := raw[ik]
					if v > bestLogit {
						bestLogit = v
						bestCls = k
					}
				}
				p := float32(sigmoid64(float64(bestLogit)))
				i0 := rawIndexBCN(channelMajor, Ci, Ai, 0, anchor)
				i1 := rawIndexBCN(channelMajor, Ci, Ai, 1, anchor)
				i2 := rawIndexBCN(channelMajor, Ci, Ai, 2, anchor)
				i3 := rawIndexBCN(channelMajor, Ci, Ai, 3, anchor)
				cx, cy, bw, bh := raw[i0], raw[i1], raw[i2], raw[i3]
				lbl := ""
				if bestCls >= 0 && bestCls < len(labels) {
					lbl = labels[bestCls]
				}
				pass := p > confTh
				fmt.Fprintf(&sb, "%d\t%g\t%d\t%s\tcx=%g cy=%g w=%g h=%g\t%v\n", anchor, p, bestCls, lbl, cx, cy, bw, bh, pass)
			}
			log.Debug().Int("anchor_from", start).Int("anchor_to", end-1).Str("tsv", sb.String()).Send()
		}
		return
	}
	log.Debug().Str("out_shape", fmt.Sprintf("%v", shape)).Int("raw_len", len(raw)).Msg("[vision] yolo26: no slot dump for this output layout")
}

// postprocessStats 端到端 [N,6] 后处理计数，用于区分「置信度不够」与「高分但框退化」。
type postprocessStats struct {
	PassConf  int
	FailGeom  int
	MaxValid  float64 // 通过几何校验的槽位最大 conf；无则 0
	MaxRaw    float64 // 原始槽位最大 conf（可能对应零面积框）
}

// maxInterleavedSlotConf 对 [...,N,6] 端到端布局取各槽位 conf（每 6 元组第 5 个）的最大值，便于对照 vision.conf。
func maxInterleavedSlotConf(raw []float32, shape []int64) float64 {
	n, interleaved, ok := parseDetectionLayout(shape, len(raw))
	if !ok || !interleaved || n <= 0 || len(raw) < n*6 {
		return 0
	}
	var m float32
	for i := 0; i < n; i++ {
		c := raw[i*6+4]
		if c > m {
			m = c
		}
	}
	return float64(m)
}

// logYoloInferSummary 每次推理打 Info（含无框），便于对照实时链路。
func logYoloInferSummary(origW, origH, modelIn int, outShape []int64, boxes []VisionBBox, raw []float32, st postprocessStats, confTh float32, preprocess, run, post time.Duration) {
	const maxShow = 6
	if st.MaxRaw == 0 {
		st.MaxRaw = maxInterleavedSlotConf(raw, outShape)
	}
	evt := log.Info().
		Int("frame_w", origW).Int("frame_h", origH).
		Int("model_in", modelIn).
		Str("out_shape", fmt.Sprintf("%v", outShape)).
		Int("boxes", len(boxes)).
		Float64("max_slot_conf", st.MaxRaw).
		Float64("max_valid_conf", st.MaxValid).
		Int("pass_conf", st.PassConf).
		Int("fail_geom", st.FailGeom).
		Float32("conf_th", confTh).
		Dur("preprocess", preprocess).
		Dur("run", run).
		Dur("postprocess", post).
		Dur("total", preprocess+run+post)
	if len(boxes) == 0 {
		evt.Msg("[vision] yolo26: infer done (no boxes)")
		return
	}
	var sb strings.Builder
	for i := 0; i < len(boxes) && i < maxShow; i++ {
		b := boxes[i]
		if i > 0 {
			sb.WriteString("; ")
		}
		lbl := b.Label
		if lbl == "" {
			lbl = "?"
		}
		fmt.Fprintf(&sb, "%s:%.2f", lbl, b.Score)
	}
	if len(boxes) > maxShow {
		fmt.Fprintf(&sb, " …+%d", len(boxes)-maxShow)
	}
	evt.Str("sample", sb.String()).Msg("[vision] yolo26: infer done")
}

// letterboxBlobRGB 兼容入口：单次分配缓冲（测试/非热路径）。实时请用 letterboxInto + scratch。
func letterboxBlobRGB(src *image.RGBA, dst int) (blob []float32, padX, padY, scale float32) {
	var s letterboxScratch
	return letterboxInto(src, dst, &s)
}

type detCand struct {
	ix1, iy1, ix2, iy2 int
	score              float64
	classID            int
	label              string
}

// yoloDet3DLayout 识别 rank-3 检测张量：channels = min(d1,d2)，anchors = max(d1,d2)，且 anchors >> channels。
func yoloDet3DLayout(d []int64) (batch, channels, anchors int64, channelMajor bool, ok bool) {
	if len(d) != 3 {
		return 0, 0, 0, false, false
	}
	batch = d[0]
	if batch <= 0 {
		batch = 1
	}
	d1, d2 := d[1], d[2]
	if d1 <= 0 || d2 <= 0 {
		return 0, 0, 0, false, false
	}
	// 与 parseDetectionLayout 的 [...,6,N]/[...,N,6] 端到端布局互斥，避免 [1,6,300] 误判为 BCN(nc=2)。
	if d1 == 6 || d2 == 6 {
		return 0, 0, 0, false, false
	}
	lo, hi := d1, d2
	if d2 < d1 {
		lo, hi = d2, d1
	}
	if lo < 5 || lo > 4+1024 {
		return 0, 0, 0, false, false
	}
	if hi > int64(maxRawCandidates) || hi <= lo*2 {
		return 0, 0, 0, false, false
	}
	channels, anchors = lo, hi
	channelMajor = (d1 == channels && d2 == anchors)
	return batch, channels, anchors, channelMajor, true
}

func classCountForBCN(channels int, labels []string) int {
	nc := channels - 4
	if nc < 1 {
		return 0
	}
	if len(labels) > 0 && len(labels) < nc {
		return len(labels)
	}
	return nc
}

func rawIndexBCN(channelMajor bool, C, A int, ch, anchor int) int {
	if channelMajor {
		return ch*A + anchor
	}
	return anchor*C + ch
}

func sigmoid64(x float64) float64 {
	if x >= 35 {
		return 1
	}
	if x <= -35 {
		return 0
	}
	return 1 / (1 + math.Exp(-x))
}

// postprocessUltralyticsIfBCN 处理 [B,C,A] / [B,A,C]；否则返回 ok=false 走 parseDetectionLayout 的 6 维路径。
func postprocessUltralyticsIfBCN(raw []float32, shape []int64, origW, origH int,
	padX, padY, scale, confTh, nmsIoU float32, skipNMS bool, maxDet int, labels []string) ([]VisionBBox, bool) {
	_, C, A, channelMajor, ok := yoloDet3DLayout(shape)
	if !ok {
		return nil, false
	}
	nc := classCountForBCN(int(C), labels)
	if nc < 1 {
		return []VisionBBox{}, true
	}
	expect := int(C * A)
	if len(raw) < expect {
		return []VisionBBox{}, true
	}
	invScale := 1.0 / float64(scale)
	Ci, Ai := int(C), int(A)
	cands := make([]detCand, 0, min(Ai, 4096))
	for anchor := 0; anchor < Ai; anchor++ {
		i0 := rawIndexBCN(channelMajor, Ci, Ai, 0, anchor)
		i1 := rawIndexBCN(channelMajor, Ci, Ai, 1, anchor)
		i2 := rawIndexBCN(channelMajor, Ci, Ai, 2, anchor)
		i3 := rawIndexBCN(channelMajor, Ci, Ai, 3, anchor)
		cx := float64(raw[i0])
		cy := float64(raw[i1])
		bw := float64(raw[i2])
		bh := float64(raw[i3])
		if bw <= 0 || bh <= 0 || bw != bw || bh != bh {
			continue
		}
		bestLogit := float32(-1e9)
		bestCls := 0
		for k := 0; k < nc; k++ {
			ik := rawIndexBCN(channelMajor, Ci, Ai, 4+k, anchor)
			v := raw[ik]
			if v > bestLogit {
				bestLogit = v
				bestCls = k
			}
		}
		bestProb := sigmoid64(float64(bestLogit))
		if bestProb <= float64(confTh) {
			continue
		}
		x1 := cx - bw*0.5
		y1 := cy - bh*0.5
		x2 := cx + bw*0.5
		y2 := cy + bh*0.5
		sx1 := (x1 - float64(padX)) * invScale
		sy1 := (y1 - float64(padY)) * invScale
		sx2 := (x2 - float64(padX)) * invScale
		sy2 := (y2 - float64(padY)) * invScale
		ix1 := clampInt(int(math.Round(sx1)), 0, origW-1)
		iy1 := clampInt(int(math.Round(sy1)), 0, origH-1)
		ix2 := clampInt(int(math.Round(sx2)), 0, origW-1)
		iy2 := clampInt(int(math.Round(sy2)), 0, origH-1)
		if ix2 <= ix1 || iy2 <= iy1 {
			continue
		}
		lbl := ""
		if bestCls >= 0 && bestCls < len(labels) {
			lbl = labels[bestCls]
		}
		cands = append(cands, detCand{
			ix1: ix1, iy1: iy1, ix2: ix2, iy2: iy2,
			score: bestProb, classID: bestCls, label: lbl,
		})
	}
	if len(cands) == 0 {
		return []VisionBBox{}, true
	}
	var kept []detCand
	if skipNMS {
		sort.Slice(cands, func(i, j int) bool { return cands[i].score > cands[j].score })
		if len(cands) > maxDet {
			kept = cands[:maxDet]
		} else {
			kept = cands
		}
	} else {
		kept = nmsClassAware(cands, float64(nmsIoU), maxDet)
	}
	out := make([]VisionBBox, 0, len(kept))
	fw, fh := float64(origW), float64(origH)
	for _, k := range kept {
		lbl := k.label
		out = append(out, VisionBBox{
			X:     float64(k.ix1) / fw,
			Y:     float64(k.iy1) / fh,
			W:     float64(k.ix2-k.ix1) / fw,
			H:     float64(k.iy2-k.iy1) / fh,
			Label: lbl,
			Score: k.score,
			Text:  lbl,
		})
	}
	return out, true
}

func postprocessDetections(raw []float32, shape []int64, origW, origH int,
	padX, padY, scale, confTh, nmsIoU float32, skipNMS bool, maxDet int, labels []string) ([]VisionBBox, postprocessStats) {
	var st postprocessStats
	n, interleaved, ok := parseDetectionLayout(shape, len(raw))
	if !ok || n == 0 {
		return []VisionBBox{}, st
	}
	if n > maxRawCandidates {
		return []VisionBBox{}, st
	}
	if interleaved {
		if len(raw) < n*6 {
			return []VisionBBox{}, st
		}
	} else if len(raw) < 6*n {
		return []VisionBBox{}, st
	}

	invScale := 1.0 / float64(scale)
	cands := make([]detCand, 0, min(n, 256))
	for i := 0; i < n; i++ {
		var x1, y1, x2, y2, confidence float32
		var classID int
		if interleaved {
			o := i * 6
			x1, y1, x2, y2 = raw[o], raw[o+1], raw[o+2], raw[o+3]
			confidence = raw[o+4]
			classID = int(raw[o+5])
		} else {
			x1 = raw[0*n+i]
			y1 = raw[1*n+i]
			x2 = raw[2*n+i]
			y2 = raw[3*n+i]
			confidence = raw[4*n+i]
			classID = int(raw[5*n+i])
		}
		if confidence > float32(st.MaxRaw) {
			st.MaxRaw = float64(confidence)
		}
		if confidence <= confTh || confidence != confidence { // 跳过 NaN
			continue
		}
		st.PassConf++
		sx1 := float64(x1-padX) * invScale
		sy1 := float64(y1-padY) * invScale
		sx2 := float64(x2-padX) * invScale
		sy2 := float64(y2-padY) * invScale

		ix1 := clampInt(int(math.Round(sx1)), 0, origW-1)
		iy1 := clampInt(int(math.Round(sy1)), 0, origH-1)
		ix2 := clampInt(int(math.Round(sx2)), 0, origW-1)
		iy2 := clampInt(int(math.Round(sy2)), 0, origH-1)
		if ix2 <= ix1 || iy2 <= iy1 {
			st.FailGeom++
			continue
		}
		if float64(confidence) > st.MaxValid {
			st.MaxValid = float64(confidence)
		}
		label := ""
		if classID >= 0 && classID < len(labels) {
			label = labels[classID]
		}
		cands = append(cands, detCand{
			ix1: ix1, iy1: iy1, ix2: ix2, iy2: iy2,
			score: float64(confidence), classID: classID, label: label,
		})
	}
	if len(cands) == 0 {
		return []VisionBBox{}, st
	}
	var kept []detCand
	if skipNMS {
		sort.Slice(cands, func(i, j int) bool { return cands[i].score > cands[j].score })
		if len(cands) > maxDet {
			kept = cands[:maxDet]
		} else {
			kept = cands
		}
	} else {
		kept = nmsClassAware(cands, float64(nmsIoU), maxDet)
	}
	out := make([]VisionBBox, 0, len(kept))
	fw, fh := float64(origW), float64(origH)
	for _, k := range kept {
		lbl := k.label
		out = append(out, VisionBBox{
			X:     float64(k.ix1) / fw,
			Y:     float64(k.iy1) / fh,
			W:     float64(k.ix2-k.ix1) / fw,
			H:     float64(k.iy2-k.iy1) / fh,
			Label: lbl,
			Score: k.score,
			Text:  lbl,
		})
	}
	return out, st
}

// parseDetectionLayout 解析 [... , N, 6] 或 [... , 6, N]；动态维 -1 时由 raw 长度推断 N。
func parseDetectionLayout(shape []int64, rawLen int) (n int, interleaved bool, ok bool) {
	if len(shape) < 2 || rawLen < 6 {
		return 0, false, false
	}
	rank := len(shape)
	d1, d2 := shape[rank-2], shape[rank-1]
	switch {
	case d2 == 6 && d1 > 0:
		return int(d1), true, true
	case d1 == 6 && d2 > 0:
		return int(d2), false, true
	case d2 == 6 && d1 == -1:
		if rawLen%6 != 0 {
			return 0, false, false
		}
		return rawLen / 6, true, true
	case d1 == 6 && d2 == -1:
		if rawLen%6 != 0 {
			return 0, false, false
		}
		return rawLen / 6, false, true
	default:
		return 0, false, false
	}
}

func nmsClassAware(cands []detCand, iouTh float64, maxDet int) []detCand {
	sort.Slice(cands, func(i, j int) bool { return cands[i].score > cands[j].score })
	out := make([]detCand, 0, min(len(cands), maxDet))
	for _, c := range cands {
		sup := false
		for _, k := range out {
			if k.classID != c.classID {
				continue
			}
			if boxIoU(c, k) >= iouTh {
				sup = true
				break
			}
		}
		if !sup {
			out = append(out, c)
			if len(out) >= maxDet {
				break
			}
		}
	}
	return out
}

func boxIoU(a, b detCand) float64 {
	x1 := max(a.ix1, b.ix1)
	y1 := max(a.iy1, b.iy1)
	x2 := min(a.ix2, b.ix2)
	y2 := min(a.iy2, b.iy2)
	interW := x2 - x1
	interH := y2 - y1
	if interW <= 0 || interH <= 0 {
		return 0
	}
	inter := float64(interW * interH)
	areaA := float64((a.ix2 - a.ix1) * (a.iy2 - a.iy1))
	areaB := float64((b.ix2 - b.ix1) * (b.iy2 - b.iy1))
	union := areaA + areaB - inter
	if union <= 0 {
		return 0
	}
	return inter / union
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
