package vision

import (
	"sync"
	"time"
)

// aiPipeline 节流：默认 log.level=info 时每流约 2s 打一条主链路摘要，避免刷屏；细节仍走 Debug。
var (
	aiPipeMu     sync.Mutex
	aiPipeLast   = map[string]time.Time{}
	aiPipePeriod = 2 * time.Second
)

func aiPipelineDue(streamKey string) bool {
	aiPipeMu.Lock()
	defer aiPipeMu.Unlock()
	now := time.Now()
	if t, ok := aiPipeLast[streamKey]; ok && now.Sub(t) < aiPipePeriod {
		return false
	}
	aiPipeLast[streamKey] = now
	return true
}

// logAIPipeline 记录 AI 主链路阶段。forceInfo 或节流到期时用 Info，否则 Debug。
func logAIPipeline(streamKey, stage string, forceInfo bool, fields map[string]any) {
	info := forceInfo || aiPipelineDue(streamKey+"|"+stage) || aiPipelineDue(streamKey)
	ev := log.Debug()
	if info {
		ev = log.Info()
	}
	ev = ev.Str("stream", streamKey).Str("stage", stage)
	for k, v := range fields {
		switch t := v.(type) {
		case string:
			ev = ev.Str(k, t)
		case int:
			ev = ev.Int(k, t)
		case int64:
			ev = ev.Int64(k, t)
		case bool:
			ev = ev.Bool(k, t)
		case float32:
			ev = ev.Float32(k, t)
		case float64:
			ev = ev.Float64(k, t)
		case time.Duration:
			ev = ev.Dur(k, t)
		case error:
			if t != nil {
				ev = ev.Err(t)
			}
		default:
			ev = ev.Interface(k, v)
		}
	}
	ev.Msg("[vision] ai-pipeline")
}
