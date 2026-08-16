package api

import (
	"net/http"
	"sync"
)

var (
	webrtcHTTPMu      sync.RWMutex
	webrtcHTTPHandler http.HandlerFunc
)

// RegisterWebRTCHTTPHandler 由 internal/webrtc 在 Init 完成后注入；路由在 Init 中统一登记。
func RegisterWebRTCHTTPHandler(h http.HandlerFunc) {
	webrtcHTTPMu.Lock()
	defer webrtcHTTPMu.Unlock()
	webrtcHTTPHandler = h
}

func serveWebRTCHTTP(w http.ResponseWriter, r *http.Request) {
	webrtcHTTPMu.RLock()
	h := webrtcHTTPHandler
	webrtcHTTPMu.RUnlock()
	if h == nil {
		http.Error(w, "webrtc module not initialized", http.StatusServiceUnavailable)
		return
	}
	h(w, r)
}
