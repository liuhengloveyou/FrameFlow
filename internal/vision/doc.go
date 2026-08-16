// Package vision 从视频流拉关键帧并解析，通过 (*streams.Stream).BroadcastStreamData /
// streams.BroadcastStreamData 向该流实例上所有附属数据订阅者推送 JSON。
//
// 检测模型、标签、置信度等全部来自 aibiz 数据库（模型仓库上传 + 算法参数 + 通道绑定），
// 启动时不读 YAML vision 块，也没有全系统共用的固定模型。
// 流检测路径：stream → running 绑定 → 算法 → 该算法关联的 ONNX 文件。
// ONNX Runtime 动态库：环境变量 ONNXRUNTIME_SHARED_LIBRARY_PATH，或自动发现 lib/libonnxruntime.*。
//
// 采样默认见 init.go（DefaultStreamAIPolicy）；绑定 rules 可覆盖 pull_sample_ms / pull_*（每路独立）。
// debug 日志 [vision] stages：get=GetOrPatch；kf=拉帧；jpeg=H264/H265 的 libav 解码耗时（MJPEG 源为 FixJPEG）；det=推理；pub=Marshal+Broadcast。
// H264/H265 仅支持 CGO+libavcodec（internal/ffdecode）解码；解码与可选的 YUV→RGB 缩放由 libav/libswscale 在一条链内完成。无 CGO 或解码失败时返回错误，不再使用 FFmpeg 子进程转 JPEG。
// keyframe 模式下叠框滞后仍受编码端 GOP/IDR 间隔影响；interval 模式下主要受 pull_sample 与 poll 影响。
// VisionFramePayload 可选 rtp_ts（关键帧 RTP timestamp）；支持 Chromium 的页面可用 requestVideoFrameCallback(metadata.rtpTimestamp) 与叠框对齐到同一解码帧。
//
// 浏览器在 offer 前使用与 streams.VisionDataChannelLabel 相同的 label 创建 DataChannel；
// internal/webrtc 在 OnDataChannel 中调用 streams.RegisterStreamDataSubscriber，将广播数据写入 DC。
// 广播体为 VisionFramePayload（JSON）：含 unix_ms、keyframe_unix_ms（可选）、rtp_ts（可选，RTP 时间戳）、
// text（整帧说明）、boxes[]（VisionBBox：x,y,w,h 归一化，可选 label/score/text，text 建议在叠框外右下角下方展示）。
//
// 其他模块也可在解析完成后调用 streams.BroadcastStreamData(streamKey, jsonBytes)。
package vision
