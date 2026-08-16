package vision

import "sync"

// NormPoint 归一化坐标（相对画面 0～1）。
type NormPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// StreamAIPolicy 某路视频流的算法策略（由 aibiz 根据 DB 中 running 绑定解析）。
// ModelPath/LabelsPath/Conf/采样参数等均来自「绑定 → 算法 → 模型/规则」表，非 YAML。
type StreamAIPolicy struct {
	Enabled        bool
	BindID         string
	AlgorithmName  string
	ModelPath      string
	LabelsPath     string
	Conf           float32 // 置信度阈值；≤0 表示用检测器默认
	NMSIoU         float32
	MaxDet         int
	Regions        [][]NormPoint // 空 = 不按区域过滤
	AlarmLevel     string        // info|warn|critical
	CooldownSec     float64       // 同类告警冷却秒数；≤0 用业务默认
	SkipIfNoPolicy bool          // 解析器未注册时：false=跳过检测

	// 取帧 / 检测节拍（绑定规则可配；≤0 / 未设时用代码默认）
	PullKeyframeOnly bool // true=只拉 IDR；false=interval 取样（含 P 帧）
	PullSampleMS     int  // 检测间隔毫秒：既管取帧节流，也管检测循环最小间隔
	PullMaxGOPBytes  int  // interval GOP 累积上限（字节）
}

// StreamAIPolicyResolver 根据 streams 键（如 gb28181://dev/ch?stream=sub）返回策略。
// ok=false 表示无法解析该流（非国标等）；此时 vision 跳过检测。
type StreamAIPolicyResolver func(streamKey string) (StreamAIPolicy, bool)

var (
	policyMu       sync.RWMutex
	policyResolver StreamAIPolicyResolver
)

// SetStreamAIPolicyResolver 由 aibiz.Init 注册；传 nil 清除。
func SetStreamAIPolicyResolver(fn StreamAIPolicyResolver) {
	policyMu.Lock()
	defer policyMu.Unlock()
	policyResolver = fn
}

// LookupStreamAIPolicy 查询当前流的算法策略。
// hasResolver=false 表示尚未注册业务解析器。
func LookupStreamAIPolicy(streamKey string) (p StreamAIPolicy, resolved, hasResolver bool) {
	policyMu.RLock()
	fn := policyResolver
	policyMu.RUnlock()
	if fn == nil {
		return StreamAIPolicy{}, false, false
	}
	p, ok := fn(streamKey)
	return p, ok, true
}
