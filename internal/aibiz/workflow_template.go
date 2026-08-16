package aibiz

import (
	"encoding/json"
	"errors"

	"github.com/liuhengloveyou/FrameFlow/internal/api"
	"github.com/liuhengloveyou/FrameFlow/internal/app"
	"gorm.io/gorm"
)

type workflowPreset struct {
	ID          string
	Name        string
	Description string
	Nodes       []api.WorkflowNode
	Edges       []api.WorkflowEdge
}

// defaultWorkflowGraph 新建工作流时的开箱模板（与「报警通知归档」预设同结构）。
func defaultWorkflowGraph() (nodes []api.WorkflowNode, edges []api.WorkflowEdge) {
	return graphNotifyFile()
}

func wfNode(id, typ, label, desc string, x, y float64, cfg map[string]any) api.WorkflowNode {
	if cfg == nil {
		cfg = map[string]any{}
	}
	return api.WorkflowNode{
		ID:       id,
		Type:     "workflow",
		Position: map[string]any{"x": x, "y": y},
		Data: map[string]any{
			"type":        typ,
			"label":       label,
			"description": desc,
			"config":      cfg,
		},
	}
}

func graphNotifyFile() ([]api.WorkflowNode, []api.WorkflowEdge) {
	t, n, s := "n-trigger", "n-notify", "n-save"
	return []api.WorkflowNode{
			wfNode(t, "trigger_alarm", "报警触发", "由报警事件或调试输入触发", 80, 160, nil),
			wfNode(n, "notify", "通知输出", "写入通知文件", 360, 160, map[string]any{
				"channel": "file",
				"message": "{{title}} · {{cameraName}} · {{message}}",
			}),
			wfNode(s, "save_file", "保存结果", "写入 data/workflow-results/", 640, 160, nil),
		}, []api.WorkflowEdge{
			{ID: "e-trigger-notify", Source: t, Target: n},
			{ID: "e-notify-save", Source: n, Target: s},
		}
}

func graphToast() ([]api.WorkflowNode, []api.WorkflowEdge) {
	t, n := "n-trigger", "n-notify"
	return []api.WorkflowNode{
			wfNode(t, "trigger_alarm", "报警触发", "由报警事件或调试输入触发", 120, 160, nil),
			wfNode(n, "notify", "页面提示", "前端弹出提示（toast）", 420, 160, map[string]any{
				"channel": "toast",
				"message": "【{{level}}】{{title}} · {{cameraName}}",
			}),
		}, []api.WorkflowEdge{
			{ID: "e-trigger-notify", Source: t, Target: n},
		}
}

func graphWebhook() ([]api.WorkflowNode, []api.WorkflowEdge) {
	t, n, s := "n-trigger", "n-notify", "n-save"
	return []api.WorkflowNode{
			wfNode(t, "trigger_alarm", "报警触发", "由报警事件或调试输入触发", 80, 160, nil),
			wfNode(n, "notify", "Webhook", "POST JSON 到外部系统（请改 URL）", 360, 160, map[string]any{
				"channel":     "webhook",
				"webhook_url": "https://httpbin.org/post",
				"message":     "{{title}} · {{cameraName}} · {{message}}",
			}),
			wfNode(s, "save_file", "保存结果", "本地留存执行上下文", 640, 160, nil),
		}, []api.WorkflowEdge{
			{ID: "e-trigger-notify", Source: t, Target: n},
			{ID: "e-notify-save", Source: n, Target: s},
		}
}

func graphFilterToast() ([]api.WorkflowNode, []api.WorkflowEdge) {
	t, f, n, s := "n-trigger", "n-filter", "n-notify", "n-save"
	return []api.WorkflowNode{
			wfNode(t, "trigger_alarm", "报警触发", "由报警事件或调试输入触发", 60, 160, nil),
			wfNode(f, "filter_zone", "区域过滤", "置信度≥0.4；可再配类别/ROI", 280, 160, map[string]any{
				"min_conf": 0.4,
				"labels":   "person",
			}),
			wfNode(n, "notify", "页面提示", "过滤后仍有目标再提示", 500, 160, map[string]any{
				"channel": "toast",
				"message": "降噪后告警：{{title}} · {{cameraName}}",
			}),
			wfNode(s, "save_file", "保存结果", "写入过滤后上下文", 720, 160, nil),
		}, []api.WorkflowEdge{
			{ID: "e-trigger-filter", Source: t, Target: f},
			{ID: "e-filter-notify", Source: f, Target: n},
			{ID: "e-notify-save", Source: n, Target: s},
		}
}

func graphArchiveOnly() ([]api.WorkflowNode, []api.WorkflowEdge) {
	t, s := "n-trigger", "n-save"
	return []api.WorkflowNode{
			wfNode(t, "trigger_alarm", "报警触发", "由报警事件或调试输入触发", 160, 160, nil),
			wfNode(s, "save_file", "保存结果", "仅归档，不发通知", 440, 160, nil),
		}, []api.WorkflowEdge{
			{ID: "e-trigger-save", Source: t, Target: s},
		}
}

func presetWorkflows() []workflowPreset {
	n1, e1 := graphNotifyFile()
	n2, e2 := graphToast()
	n3, e3 := graphWebhook()
	n4, e4 := graphFilterToast()
	n5, e5 := graphArchiveOnly()
	return []workflowPreset{
		{
			ID:          "wf-preset-notify-file",
			Name:        "报警通知归档",
			Description: "常用：写通知文件 + 保存结果。适合联调与留痕。",
			Nodes:       n1, Edges: e1,
		},
		{
			ID:          "wf-preset-toast",
			Name:        "页面提示",
			Description: "报警后前端弹出提示。适合快速验证联动。",
			Nodes:       n2, Edges: e2,
		},
		{
			ID:          "wf-preset-webhook",
			Name:        "Webhook 推送",
			Description: "POST 到外部 URL（默认 httpbin，请改成你的地址）+ 本地归档。",
			Nodes:       n3, Edges: e3,
		},
		{
			ID:          "wf-preset-filter-toast",
			Name:        "降噪后提示",
			Description: "置信度/类别过滤后再 toast + 归档。适合减少误报刷屏测试。",
			Nodes:       n4, Edges: e4,
		},
		{
			ID:          "wf-preset-archive",
			Name:        "仅归档",
			Description: "只保存结果、不通知。适合压测投递链路。",
			Nodes:       n5, Edges: e5,
		},
	}
}

// seedPresetWorkflows 幂等写入常用预设（按固定 ID；已存在则跳过，不覆盖用户改动）。
func (s *Service) seedPresetWorkflows() {
	created := 0
	for _, p := range presetWorkflows() {
		if _, err := s.store.getWorkflow(p.ID); err == nil {
			continue
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			app.Logger.Warn().Err(err).Str("id", p.ID).Msg("[workflows] seed lookup")
			continue
		}
		nodesJSON, err := json.Marshal(p.Nodes)
		if err != nil {
			continue
		}
		edgesJSON, err := json.Marshal(p.Edges)
		if err != nil {
			continue
		}
		row := &WorkflowRow{
			ID:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			Status:      "stopped",
			NodesJSON:   string(nodesJSON),
			EdgesJSON:   string(edgesJSON),
		}
		if err := s.store.insertWorkflow(row); err != nil {
			app.Logger.Warn().Err(err).Str("id", p.ID).Msg("[workflows] seed insert")
			continue
		}
		created++
	}
	if created > 0 {
		app.Logger.Info().Int("created", created).Msg("[workflows] preset workflows seeded")
	}
}
