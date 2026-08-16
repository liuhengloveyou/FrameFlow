package vision

import (
	"sync"
	"time"
)

// DetectionEvent 某路流一次有效检出（规则过滤后非空框）。
type DetectionEvent struct {
	StreamKey string
	Boxes     []VisionBBox
	JPEG      []byte // 可空；有则可用于告警抓拍
	Policy    StreamAIPolicy
	At        time.Time
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
