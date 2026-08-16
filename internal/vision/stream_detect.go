package vision

import (
	"errors"
	"fmt"
	"strings"

	"github.com/liuhengloveyou/FrameFlow/pkg/fferrors"
)

// detectForStream 按流的 DB 绑定选算法与模型：
//
//	流 → running 绑定 → 算法 → 上传的 ONNX（policy.ModelPath）+ 算法/规则 conf
//
// 无 running 绑定则不检测；有则加载该模型进缓存并推理，再按 ROI 过滤。
// 置信度只使用绑定这一套，不再与任何 YAML/全局 detector_conf 二次过滤。
func detectForStream(streamKey string, run func(modelPath string, conf float32) ([]VisionBBox, error)) ([]VisionBBox, string, error) {
	policy, resolved, hasResolver := LookupStreamAIPolicy(streamKey)

	conf := float32(0)
	modelPath := ""
	if hasResolver && resolved && policy.Enabled {
		conf = policy.Conf
		modelPath = strings.TrimSpace(policy.ModelPath)
		if modelPath != "" {
			if err := LoadYOLOModel(modelPath, policy.LabelsPath, "", conf, 0, policy.NMSIoU, policy.MaxDet); err != nil {
				logAIPipeline(streamKey, "1_model_load_fail", true, map[string]any{
					"model": modelPath,
					"err":   err.Error(),
				})
				return []VisionBBox{}, fmt.Sprintf("算法模型加载失败：%v", err), nil
			}
		}
	}

	_, detSize, detLabels, detReady := YOLOModelInfo(modelPath)

	logAIPipeline(streamKey, "1_policy", false, map[string]any{
		"has_resolver":   hasResolver,
		"resolved":       resolved,
		"enabled":        policy.Enabled,
		"algorithm":      policy.AlgorithmName,
		"bind_id":        policy.BindID,
		"conf":           conf,
		"model":          modelPath,
		"policy_regions": len(policy.Regions),
		"detector_ready": detReady,
		"detector_in":    detSize,
		"labels":         detLabels,
	})

	if hasResolver {
		if !resolved {
			logAIPipeline(streamKey, "1_policy_skip", true, map[string]any{
				"reason": "stream_not_mappable",
			})
			return []VisionBBox{}, "该流暂不支持算法绑定检测", nil
		}
		if !policy.Enabled {
			name := policy.AlgorithmName
			txt := "未启动算法（请在视频 AI 配置中绑定并启动）"
			if name != "" {
				txt = fmt.Sprintf("算法「%s」未运行", name)
			}
			logAIPipeline(streamKey, "1_policy_skip", true, map[string]any{
				"reason":    "algorithm_not_running",
				"algorithm": name,
			})
			return []VisionBBox{}, txt, nil
		}
		if modelPath == "" {
			logAIPipeline(streamKey, "1_policy_skip", true, map[string]any{
				"reason": "binding_model_missing",
			})
			return []VisionBBox{}, "绑定算法未关联可用模型文件", nil
		}
		if conf >= 0.99 {
			logAIPipeline(streamKey, "1_policy_warn_conf", true, map[string]any{
				"conf": conf,
				"hint": "置信度≈1 会滤掉几乎所有框（分数通常 <1）；建议 0.25～0.5",
			})
		}
	}

	if !detReady {
		logAIPipeline(streamKey, "2_yolo_skip", true, map[string]any{"reason": "detector_not_ready"})
	}

	boxes, err := run(modelPath, conf)
	if err == nil {
		top := summarizeBoxesTop(boxes, 5)
		logAIPipeline(streamKey, "2_yolo_done", len(boxes) > 0, map[string]any{
			"boxes_raw": len(boxes),
			"conf":      conf,
			"model":     modelPath,
			"top":       top,
		})
		if hasResolver && policy.Enabled {
			before := len(boxes)
			boxes = filterBoxesByRegions(boxes, policy.Regions)
			droppedROI := before - len(boxes)
			logAIPipeline(streamKey, "3_policy_filter", before > 0 || len(boxes) > 0, map[string]any{
				"before":      before,
				"after_roi":   len(boxes),
				"dropped_roi": droppedROI,
				"conf":        conf,
				"regions":     len(policy.Regions),
				"top_after":   summarizeBoxesTop(boxes, 5),
			})
			txt := ""
			if len(boxes) > 0 {
				txt = fmt.Sprintf("%s · 检测 %d 个目标", policy.AlgorithmName, len(boxes))
				if before != len(boxes) {
					txt += fmt.Sprintf("（区域过滤前 %d）", before)
				}
			} else if before > 0 {
				txt = fmt.Sprintf("%s · 目标均在区域外", policy.AlgorithmName)
			} else {
				txt = fmt.Sprintf("%s · 未检出目标", policy.AlgorithmName)
			}
			return boxes, txt, nil
		}
		if len(boxes) > 0 {
			return boxes, fmt.Sprintf("检测 %d 个目标", len(boxes)), nil
		}
		return boxes, "未检出目标（当前帧无框超过置信度阈值）", nil
	}
	if errors.Is(err, fferrors.ErrVisionInferBusy) {
		logAIPipeline(streamKey, "2_yolo_busy", false, map[string]any{"err": err.Error()})
		return nil, "", err
	}
	if errors.Is(err, fferrors.ErrVisionYoloSkipped) {
		logAIPipeline(streamKey, "2_yolo_skip", true, map[string]any{"reason": "no_model"})
		return []VisionBBox{}, "未加载绑定算法的模型（请确认模型仓库已上传 ONNX）", nil
	}
	logAIPipeline(streamKey, "2_yolo_error", true, map[string]any{"err": err.Error()})
	return nil, "", err
}

func summarizeBoxesTop(boxes []VisionBBox, n int) string {
	if len(boxes) == 0 || n <= 0 {
		return ""
	}
	if n > len(boxes) {
		n = len(boxes)
	}
	s := ""
	for i := 0; i < n; i++ {
		b := boxes[i]
		if i > 0 {
			s += "; "
		}
		lbl := b.Label
		if lbl == "" {
			lbl = "?"
		}
		s += fmt.Sprintf("%s:%.3f", lbl, b.Score)
	}
	if len(boxes) > n {
		s += fmt.Sprintf(" …+%d", len(boxes)-n)
	}
	return s
}

// boxesFromKeyframeJPEGStream 带流策略的 JPEG 检测。
func boxesFromKeyframeJPEGStream(streamKey string, data []byte) ([]VisionBBox, string, error) {
	return detectForStream(streamKey, func(modelPath string, conf float32) ([]VisionBBox, error) {
		return detectBoxesJPEG(modelPath, data, conf)
	})
}

// boxesFromKeyframeRGBStream 带流策略的 RGB 检测。
func boxesFromKeyframeRGBStream(streamKey string, rgb []byte, w, h int) ([]VisionBBox, string, error) {
	return detectForStream(streamKey, func(modelPath string, conf float32) ([]VisionBBox, error) {
		return detectBoxesRGB(modelPath, rgb, w, h, conf)
	})
}
