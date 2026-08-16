package streams

// VisionDataChannelLabel 与浏览器 createDataChannel(label) 一致（WebRTC 侧协商用）。
const VisionDataChannelLabel = "frameflow-vision"

var (
	onFirstStreamDataSubscriber  func(streamKey string)
	onLastStreamDataUnsubscriber func(streamKey string)
)

// SetStreamDataHooks 由 vision 等模块在 Init 时注册：当某流首次出现 / 最后一个
// 流附属数据订阅者离开时回调（用于启停后台解析任务）。仅在进程启动阶段调用。
func SetStreamDataHooks(onFirst, onLast func(streamKey string)) {
	onFirstStreamDataSubscriber = onFirst
	onLastStreamDataUnsubscriber = onLast
}

// RegisterStreamDataSubscriber 按流名解析 *Stream 后注册附属数据订阅（多别名指向同一 Stream 时共享订阅表）。
func RegisterStreamDataSubscriber(streamKey string, send func([]byte)) (cancel func()) {
	s := Get(streamKey)
	if s == nil {
		return func() {}
	}
	return s.RegisterStreamDataSubscriber(streamKey, send)
}

// BroadcastStreamData 按流名向该 Stream 上所有附属数据订阅者推送 payload。
func BroadcastStreamData(streamKey string, payload []byte) {
	s := Get(streamKey)
	if s == nil {
		return
	}
	s.BroadcastStreamData(payload)
}
