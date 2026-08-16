package vision

// boxesFromKeyframeJPEG：无流绑定上下文时不可选模型。
// 线上叠框请走 boxesFromKeyframeJPEGStream（按 DB 绑定选算法模型）。
func boxesFromKeyframeJPEG(data []byte) ([]VisionBBox, string, error) {
	_ = data
	return []VisionBBox{}, "请通过视频 AI 绑定启动算法后再检测", nil
}

// boxesFromKeyframeRGB：同 [boxesFromKeyframeJPEG]。
func boxesFromKeyframeRGB(rgb []byte, w, h int) ([]VisionBBox, string, error) {
	_, _, _ = rgb, w, h
	return []VisionBBox{}, "请通过视频 AI 绑定启动算法后再检测", nil
}
