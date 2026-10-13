package vision

import (
	"sync"
	"time"
)

// DetectionEvent 某路流一次有效检出（规则过滤后非空框）。
//
// 证据画面二选一：JPEG 为「源本身就是 JPEG」（MJPEG 直通），RGB+Width+Height 为解码后的原始帧。
// 检测热路径不再同步编码 JPEG——告警有冷却，绝大多数拍不会落库，编码放到真正落库前按需做。
type DetectionEvent struct {
	StreamKey string
	Boxes     []VisionBBox
	JPEG      []byte // 源即 JPEG 时直接带上（优先使用）
	RGB       []byte // 解码后的 RGB888；与 JPEG 二选一
	Width     int
	Height    int
	Policy    StreamAIPolicy
	At        time.Time
}

// EvidenceJPEG 返回告警证据 JPEG：优先复用源 JPEG，否则把 RGB 按需编码（1080p 约 50ms）。
// 无可用画面时返回 nil；调用方应在「确定要落告警」之后再调用，避免白白编码。
func (ev DetectionEvent) EvidenceJPEG() []byte {
	if len(ev.JPEG) > 0 {
		return ev.JPEG
	}
	if len(ev.RGB) == 0 || ev.Width <= 0 || ev.Height <= 0 {
		return nil
	}
	b, err := EncodeJPEG(ev.RGB, ev.Width, ev.Height)
	if err != nil {
		log.Warn().Err(err).Str("stream", ev.StreamKey).Msg("[vision] evidence jpeg encode")
		return nil
	}
	return b
}

// DetectionHook 由业务层（aibiz）注册：检出 → 告警 / 联动。
type DetectionHook func(ev DetectionEvent)

var (
	detectionHookMu sync.RWMutex
	detectionHook   DetectionHook
)

// SetDetectionHook 注册检出回调；传 nil 清除。回调宜异步，勿阻塞 vision 循环。
func SetDetectionHook(fn DetectionHook) {
	detectionHookMu.Lock()
	defer detectionHookMu.Unlock()
	detectionHook = fn
}

// NotifyDetection 在非空框时触发业务 hook（异步）。
func NotifyDetection(ev DetectionEvent) {
	if len(ev.Boxes) == 0 {
		return
	}
	detectionHookMu.RLock()
	fn := detectionHook
	detectionHookMu.RUnlock()
	if fn == nil {
		return
	}
	if ev.At.IsZero() {
		ev.At = time.Now()
	}
	go fn(ev)
}
