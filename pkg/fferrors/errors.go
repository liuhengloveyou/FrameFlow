// Package fferrors 集中定义 FrameFlow 业务错误（参考 AINVR common/errors，带码统一用 go-errors）。
//
// 码段约定：
//   - 40xxx streams / app / rtmp
//   - 41xxx vision
//   - 42xxx gb28181
//   - 43xxx HTTP/API 通用（见 biz.go：ErrBadRequest / ErrUnauthorized 等）
package fferrors

import (
	"fmt"

	gerr "github.com/liuhengloveyou/go-errors"
)

// --- streams（go-errors，便于日志带 code）---

var (
	ErrStreamNotFound = gerr.NewError(40400, "stream not found")

	ErrStreamsNoProducers = gerr.NewError(40401, "streams: no producers (upstream not connected for this stream key; WebRTC src must match the streams key that is actually publishing, e.g. app_stream composite)")

	ErrStreamsNoMatchingMedia = gerr.NewError(40402, "streams: no matching producer media or codecs (upstream connected but no compatible video track yet)")

	ErrStreamsSourceNotSupported = gerr.NewError(40001, "streams: source not supported")

	ErrStreamsSourceEmpty = gerr.NewError(40002, "streams: source empty")

	ErrStreamsStreamKeyInvalid = gerr.NewError(40010, "streams: stream_key 长度须为 2–128，以字母或数字开头，仅含字母数字、._-（与 AINVR/go2rtc 流名规则一致）")

	ErrStreamsStreamKeyExists = gerr.NewError(40011, "streams: stream_key 已存在")

	ErrStreamsSourceInsecure = gerr.NewError(40012, "streams: source from insecure producer")

	ErrStreamsSourceSpacesInsecure = gerr.NewError(40013, "streams: source with spaces may be insecure")

	ErrStreamsPlayConsumerNotFound = gerr.NewError(40020, "can't find consumer")
)

// ErrStreamsUnsupportedURL 与历史 `streams: unsupported scheme: <url>` 一致。
func ErrStreamsUnsupportedURL(u string) error {
	return fmt.Errorf("streams: unsupported scheme: %s", u)
}

// StreamNotFoundKey 与历史 `stream not found: <key>` 行为一致。
func StreamNotFoundKey(key string) error {
	return fmt.Errorf("%s: %s", ErrStreamNotFound.Message, key)
}

// ErrStreamsStreamNotFoundName 与历史 `streams: stream not found: <name>` 一致。
func ErrStreamsStreamNotFoundName(name string) error {
	return fmt.Errorf("streams: stream not found: %s", name)
}

// --- vision ---

var (
	ErrVisionStreamNil = gerr.NewError(41101, "vision: stream is nil")

	ErrVisionNoKeyframe = gerr.NewError(41102, "vision: no keyframe")

	ErrVisionCodec = gerr.NewError(41103, "vision: unsupported video codec")

	ErrVisionLibavRequired = gerr.NewError(41104, "vision: H264/H265 requires libav (build with CGO_ENABLED=1 and libavcodec/libavutil/libswscale dev packages)")

	ErrVisionLibavDecode = gerr.NewError(41105, "vision: libav decode failed")

	ErrVisionLibavNoFrame = gerr.NewError(41106, "vision: libav produced no frame from sample")

	ErrVisionYoloSkipped = gerr.NewError(41200, "vision: yolo26 not configured")

	ErrVisionLabelsEmpty = gerr.NewError(41201, "vision: no class names in labels file")

	ErrVisionModelNoIO = gerr.NewError(41202, "vision: model has no inputs/outputs")

	ErrVisionYoloOutputType = gerr.NewError(41203, "vision: yolo output is not float32 tensor")

	// ErrVisionInferBusy 上一帧推理尚未完成，实时路径丢弃本帧以免积压。
	ErrVisionInferBusy = gerr.NewError(41204, "vision: detector busy, frame dropped")
)

// --- gb28181（设备/拉流就绪，42xxx；文案直接面向用户）---

var (
	// ErrGbDeviceOffline 设备尚未 SIP 注册或心跳超时离线。
	ErrGbDeviceOffline = gerr.NewError(42001, "设备离线")

	// ErrGbDeviceChannelNotFound 设备或通道不在注册表。
	ErrGbDeviceChannelNotFound = gerr.NewError(42002, "设备或通道不存在")

	// ErrGbInviteNotAllowed 待确认 / 已拒绝 / 已停用，不允许 INVITE。
	ErrGbInviteNotAllowed = gerr.NewError(42003, "设备尚未确认接入，请先在设备管理中确认")

	// ErrGbSipNotStarted 国标 SIP 客户端未启动。
	ErrGbSipNotStarted = gerr.NewError(42004, "国标服务未就绪")

	// ErrGbNoRtp INVITE 成功但未收到 RTP。
	ErrGbNoRtp = gerr.NewError(42005, "已接通但未收到码流，请稍后重试或检查媒体地址")

	// ErrGbDeviceTimeout SIP/设备侧响应超时。
	ErrGbDeviceTimeout = gerr.NewError(42006, "设备响应超时，可能尚未就绪，请稍后重试")

	// ErrGbPlaybackRangeInvalid 回放起止时间非法。
	ErrGbPlaybackRangeInvalid = gerr.NewError(42007, "回放时间范围无效")

	// ErrGbGatewayDisabled 网关未初始化 / listen 未配置。
	ErrGbGatewayDisabled = gerr.NewError(42008, "国标服务未启用")

	// ErrGbMediaPortExhausted 媒体端口池耗尽。
	ErrGbMediaPortExhausted = gerr.NewError(42009, "媒体端口不足，请稍后重试")

	// ErrStreamConnectTimeout 前端无首帧超时（码表对齐；后端一般不返回）。
	ErrStreamConnectTimeout = gerr.NewError(42010, "等待码流超时，摄像头可能尚未就绪，请稍后重试")

	// ErrGbDeviceBusy 设备对 INVITE 返回 486 Busy Here（并发能力不足或设备侧拒接）。
	ErrGbDeviceBusy = gerr.NewError(42011, "设备忙，暂时无法接受拉流，请稍后重试")
)

// --- app / rtmp ---

var (
	ErrConfigDisabled = gerr.NewError(40030, "config file disabled")

	ErrRtmpPushSecretInvalid = gerr.NewError(40040, "rtmp: 推流密钥无效或 tcUrl 未携带 push_key")
)

// ErrRtmpUnknownCommand 动态 intent 文案。
func ErrRtmpUnknownCommand(intent string) error {
	return fmt.Errorf("rtmp: unknown command: %s", intent)
}
