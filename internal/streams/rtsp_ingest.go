package streams

// RTSP 推流接入与 RTMP 共用占位流表（streams 键与协议无关）。
// ANNOUNCE 路径 /<stream_key>，可选 ?push_key=<secret>（与 RTMP tcUrl 同名参数）。

// EnsureRTSPIngestSlot 确保 RTSP ANNOUNCE 占位流存在。
func EnsureRTSPIngestSlot(streamKey, pushSecret, pushSecretHashHex string) (string, error) {
	return EnsureRTMPIngestSlot(streamKey, pushSecret, pushSecretHashHex)
}

// CreateRTSPIngestSlot 创建 RTSP 推入占位流。
func CreateRTSPIngestSlot(streamKey, pushSecret string) (string, error) {
	return CreateRTMPIngestSlot(streamKey, pushSecret)
}

// DeleteRTSPIngestSlot 删除 RTSP 推流占位流与密钥登记。
func DeleteRTSPIngestSlot(streamKey string) {
	DeleteRTMPIngestSlot(streamKey)
}

// VerifyRTSPPushToken 校验 ANNOUNCE URL 上的推流密钥（未登记密钥则放行）。
func VerifyRTSPPushToken(streamKey, token string) error {
	return VerifyRTMPPushToken(streamKey, token)
}
