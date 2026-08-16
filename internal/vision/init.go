package vision

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/liuhengloveyou/FrameFlow/internal/app"
	"github.com/liuhengloveyou/FrameFlow/internal/streams"
	"github.com/rs/zerolog"
)

var log zerolog.Logger

// visionLibavDecode：本二进制以 CGO+libavcodec 构建时为 true。
var visionLibavDecode bool

// 绑定检测 / 取帧参数的唯一默认值来源。
// aibiz 新建绑定 rules、策略解析兜底、无 running 绑定时的取帧，均由此派生，勿再抄一份字面量。
const (
	DefaultConf             = float32(0.4)
	DefaultNMSIoU           = float32(0.45)
	DefaultMaxDet           = 100
	DefaultAlarmLevel       = "warn"
	DefaultCooldownSec       = 30.0
	DefaultPullKeyframeOnly = false
	DefaultPullSampleMS     = 100
	DefaultPullMaxGOPBytes  = 8 << 20
)

// DefaultStreamAIPolicy 返回未覆盖绑定 rules 时的策略字段默认值（不含 Enabled/BindID/模型路径）。
func DefaultStreamAIPolicy() StreamAIPolicy {
	return StreamAIPolicy{
		Conf:             DefaultConf,
		NMSIoU:           DefaultNMSIoU,
		MaxDet:           DefaultMaxDet,
		AlarmLevel:       DefaultAlarmLevel,
		CooldownSec:      DefaultCooldownSec,
		PullKeyframeOnly: DefaultPullKeyframeOnly,
		PullSampleMS:     DefaultPullSampleMS,
		PullMaxGOPBytes:  DefaultPullMaxGOPBytes,
	}
}

// DefaultBindingRulesMap 新建绑定时写入 DB 的 rules 默认值（与 DefaultStreamAIPolicy 一致）。
func DefaultBindingRulesMap() map[string]any {
	d := DefaultStreamAIPolicy()
	return map[string]any{
		"conf":               float64(d.Conf),
		"iou":                float64(d.NMSIoU),
		"max_det":            d.MaxDet,
		"pull_keyframe_only": d.PullKeyframeOnly,
		"pull_sample_ms":     d.PullSampleMS,
		"pull_max_gop_mb":    d.PullMaxGOPBytes >> 20,
		"enable_track":       true,
		"alarm_level":        d.AlarmLevel,
		"cooldown_sec":       d.CooldownSec,
	}
}

// Init 初始化采样/解码钩子与 ORT 路径。
// 模型权重、标签、置信度、取帧/轮询参数由 aibiz 模型仓库与算法绑定提供，不再读 YAML。
func Init() {
	log = app.GetLogger("vision")
	SetModelsDataDir("") // data/models
	if p := discoverORTLib(); p != "" {
		SetORTLibPath(p)
	}

	visionLibavDecode = libavDecodeSupported()
	d := DefaultStreamAIPolicy()

	log.Info().
		Str("models_dir", ModelsDataDir()).
		Str("models_dir_abs", ModelsRootAbs()).
		Str("ort_lib", configuredORTLib()).
		Bool("default_pull_keyframe_only", d.PullKeyframeOnly).
		Int("default_pull_sample_ms", d.PullSampleMS).
		Int("default_pull_max_gop_mb", d.PullMaxGOPBytes>>20).
		Bool("libav_decode", visionLibavDecode).
		Msg("[vision] init (models/pull params from DB binding)")

	// 不在此加载 ONNX：模型在 DB；由绑定启动 / 流检测按算法关联路径 LoadYOLOModel。
	streams.SetStreamDataHooks(startVisionStreamWorker, stopVisionStreamWorker)
}

// discoverORTLib 在常见相对路径与环境变量中查找 ONNX Runtime 动态库。
func discoverORTLib() string {
	if e := os.Getenv("ONNXRUNTIME_SHARED_LIBRARY_PATH"); e != "" {
		return e
	}
	var names []string
	switch runtime.GOOS {
	case "darwin":
		names = []string{"lib/libonnxruntime.dylib", "lib/onnxruntime.dylib"}
	case "windows":
		names = []string{"lib/onnxruntime.dll", "lib/libonnxruntime.dll"}
	default:
		names = []string{"lib/libonnxruntime.so", "lib/onnxruntime.so"}
	}
	for _, n := range names {
		if st, err := os.Stat(n); err == nil && !st.IsDir() {
			if abs, err := filepath.Abs(n); err == nil {
				return abs
			}
			return n
		}
	}
	return ""
}
