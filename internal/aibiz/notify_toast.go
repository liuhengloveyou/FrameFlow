package aibiz

import "sync"

var (
	toastMu     sync.Mutex
	toastSeq    int64
	toastLatest map[string]any
)

func pushWorkflowToast(payload map[string]any) int64 {
	toastMu.Lock()
	defer toastMu.Unlock()
	toastSeq++
	cp := map[string]any{}
	for k, v := range payload {
		cp[k] = v
	}
	cp["seq"] = toastSeq
	toastLatest = cp
	return toastSeq
}

// LatestWorkflowToast 返回最近一次 toast 通知（供前端轮询）。
func (s *Service) LatestWorkflowToast() (item map[string]any, seq int64) {
	toastMu.Lock()
	defer toastMu.Unlock()
	if toastLatest == nil {
		return nil, toastSeq
	}
	cp := map[string]any{}
	for k, v := range toastLatest {
		cp[k] = v
	}
	return cp, toastSeq
}
