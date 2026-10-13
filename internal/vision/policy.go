package vision

import (
	"sync"
	"time"
)

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
	CooldownSec    float64       // 同类告警冷却秒数；≤0 用业务默认
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

// policyCacheTTL 策略解析结果缓存时长。
// 解析要查 bindings/算法/模型/摄像头（4~6 条 SQL），而检测循环每拍会调 3~4 次；
// 没有缓存时每帧要打 20~30 条 SQL。1s 表示改配置最多 1s 生效，对告警配置足够。
const policyCacheTTL = time.Second

type policyCacheEntry struct {
	policy   StreamAIPolicy
	ok       bool
	expireAt time.Time
}

var policyCache sync.Map // streamKey -> policyCacheEntry

// SetStreamAIPolicyResolver 由 aibiz.Init 注册；传 nil 清除。
func SetStreamAIPolicyResolver(fn StreamAIPolicyResolver) {
	policyMu.Lock()
	policyResolver = fn
	policyMu.Unlock()
	InvalidateStreamAIPolicies()
}

// LookupStreamAIPolicy 查询当前流的算法策略（带 TTL 缓存，见 policyCacheTTL）。
// hasResolver=false 表示尚未注册业务解析器。
func LookupStreamAIPolicy(streamKey string) (p StreamAIPolicy, resolved, hasResolver bool) {
	policyMu.RLock()
	fn := policyResolver
	policyMu.RUnlock()
	if fn == nil {
		return StreamAIPolicy{}, false, false
	}
	now := time.Now()
	if v, hit := policyCache.Load(streamKey); hit {
		if e, okEntry := v.(policyCacheEntry); okEntry && now.Before(e.expireAt) {
			return e.policy, e.ok, true
		}
	}
	p, ok := fn(streamKey)
	policyCache.Store(streamKey, policyCacheEntry{policy: p, ok: ok, expireAt: now.Add(policyCacheTTL)})
	return p, ok, true
}

// InvalidateStreamAIPolicies 立即清空策略缓存。绑定 / 算法 / 模型变更后调用，
// 免得要等 TTL 过期才生效。
func InvalidateStreamAIPolicies() {
	policyCache.Range(func(k, _ any) bool {
		policyCache.Delete(k)
		return true
	})
}
