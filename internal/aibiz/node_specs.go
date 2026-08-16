package aibiz

import "github.com/liuhengloveyou/FrameFlow/internal/api"

func f64(v float64) *float64 { return &v }

// DefaultNodeSpecs 编辑器可用节点类型。
func DefaultNodeSpecs() []api.NodeSpec {
	return []api.NodeSpec{
		{
			Type: "trigger_alarm", Label: "报警触发", Category: "触发",
			Description: "工作流入口：由报警事件或调试输入触发", IsTrigger: true,
			Inputs:  []api.NodeFieldSpec{},
			Outputs: []api.NodePortSpec{{Key: "alarm", Label: "报警对象", Type: "json"}},
		},
		{
			Type: "camera_read", Label: "摄像头读取", Category: "采集",
			Description: "从在线流拉取 JPEG 关键帧；若已有报警证据图则优先复用",
			Inputs: []api.NodeFieldSpec{
				{Key: "camera_info", Label: "摄像头", Type: "camera_info", Required: true},
				{
					Key: "read_scheme", Label: "读取方案", Type: "select", Required: true,
					Options: []map[string]string{
						{"label": "关键帧", "value": "keyframe"},
						{"label": "实时帧", "value": "live"},
						{"label": "截图", "value": "snapshot"},
					},
				},
			},
			Outputs: []api.NodePortSpec{{Key: "frame", Label: "图像帧", Type: "json"}},
		},
		{
			Type: "infer_detect", Label: "目标检测", Category: "推理",
			Description: "对输入图像执行 YOLO/ONNX 检测（需模型本地可用）",
			Inputs: []api.NodeFieldSpec{
				{Key: "model_id", Label: "模型 ID", Type: "string", Required: true},
				{Key: "conf", Label: "置信度", Type: "number", Min: f64(0), Max: f64(1), Step: f64(0.01)},
				{Key: "extra", Label: "扩展参数", Type: "json"},
				{Key: "enable_nms", Label: "启用 NMS", Type: "boolean"},
			},
			Outputs: []api.NodePortSpec{{Key: "detections", Label: "检测结果", Type: "json"}},
		},
		{
			Type: "filter_zone", Label: "区域过滤", Category: "处理",
			Description: "按置信度 / 类别白名单 / 归一化矩形 ROI 过滤检测框",
			Inputs: []api.NodeFieldSpec{
				{Key: "min_conf", Label: "最低置信度", Type: "number", Min: f64(0), Max: f64(1), Step: f64(0.01)},
				{Key: "labels", Label: "类别白名单", Type: "string", Description: "逗号分隔，空=全部；如 person,car"},
				{Key: "zone", Label: "ROI 矩形", Type: "json", Description: "{\"x\":0.1,\"y\":0.1,\"w\":0.5,\"h\":0.5} 归一化坐标，空=不限区域"},
			},
			Outputs: []api.NodePortSpec{{Key: "detections", Label: "过滤后检测", Type: "json"}},
		},
		{
			Type: "notify", Label: "通知输出", Category: "动作",
			Description: "发送通知（toast / webhook / 文件）；消息支持 {{title}} {{cameraName}} 等占位符",
			Inputs: []api.NodeFieldSpec{
				{
					Key: "channel", Label: "通道", Type: "select", Required: true,
					Options: []map[string]string{
						{"label": "页面提示", "value": "toast"},
						{"label": "Webhook", "value": "webhook"},
						{"label": "文件", "value": "file"},
					},
				},
				{Key: "message", Label: "消息模板", Type: "string", Description: "可用 {{title}} {{level}} {{message}} {{cameraName}} {{algorithmName}}"},
				{Key: "webhook_url", Label: "Webhook URL", Type: "string"},
			},
			Outputs: []api.NodePortSpec{{Key: "result", Label: "发送结果", Type: "json"}},
		},
		{
			Type: "save_file", Label: "保存结果", Category: "动作",
			Description: "将上游输出写入 data/workflow-results/",
			Inputs:  []api.NodeFieldSpec{},
			Outputs: []api.NodePortSpec{{Key: "path", Label: "文件路径", Type: "string"}},
		},
	}
}
