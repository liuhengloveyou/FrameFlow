package vision

// VisionFramePayload 经 WebRTC DataChannel（label=streams.VisionDataChannelLabel）推送的单帧结果。
type VisionFramePayload struct {
	Type           string       `json:"type"`                       // 与 DataChannel label 一致
	V              int          `json:"v"`                          // 协议版本
	Stream         string       `json:"stream"`                     // 对应 streams 键
	Event          string       `json:"event"`                      // 如 frame
	UnixMS         int64        `json:"unix_ms"`                    // 即将写入 DataChannel 前（检测与序列化完成之后）
	KeyframeUnixMS int64        `json:"keyframe_unix_ms,omitempty"` // KeyframeUnixMS 为拿到本帧 JPEG 字节后、调用检测之前的服务器时刻，用于与 UnixMS 相减估算服务端处理耗时。
	RtpTimestamp   *int64       `json:"rtp_ts,omitempty"`           // 检测所用关键帧的 RTP timestamp；与浏览器解码帧 metadata.rtpTimestamp 对齐叠框（无指针=未知）。
	Text           string       `json:"text,omitempty"`             // 本帧整体文字说明（业务/调试）
	Boxes          []VisionBBox `json:"boxes"`
}

// VisionBBox 归一化矩形（相对整帧宽高 0..1），用于浏览器叠框。
type VisionBBox struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	W     float64 `json:"w"`
	H     float64 `json:"h"`
	Label string  `json:"label,omitempty"`
	Score float64 `json:"score,omitempty"`
	// Text 画在框外右下角下方；空时前端可用 Label 兜底。
	Text string `json:"text,omitempty"`
	// TrackID 航迹 ID（BoxTracker 分配，同一目标跨帧稳定）。0 = 未跟踪（透传的原始检测）。
	// 前端据此做稳定 key 与插值，避免框逐帧重建造成闪烁。
	TrackID uint64 `json:"track_id,omitempty"`
	// Miss 连续丢失拍数（>0 表示该框是航迹保持中的外推位置，非本拍真实检测）。
	Miss int `json:"miss,omitempty"`
}
