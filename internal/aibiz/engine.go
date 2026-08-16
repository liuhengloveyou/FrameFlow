package aibiz

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/liuhengloveyou/FrameFlow/internal/api"
	"github.com/liuhengloveyou/FrameFlow/pkg/utils"
)

// DebugEvent 与前端 DebugEvent 对齐。
type DebugEvent = api.DebugEvent

type runContext struct {
	WorkflowID string
	RunID      string
	Input      map[string]any
	Vars       map[string]any // 上游输出按节点 id 存放
	ImageJPEG  []byte
	Events     []api.DebugEvent
	mu         sync.Mutex
}

func (c *runContext) emit(kind, nodeID, message string, payload any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Events = append(c.Events, api.DebugEvent{
		ID:      "ev-" + uuid.NewString()[:8],
		Kind:    kind,
		NodeID:  nodeID,
		Message: message,
		At:      utils.NowISO(),
		Payload: payload,
	})
}

type dagNode struct {
	ID   string
	Type string
	Data map[string]any
}

// validateAndOrder 校验图并返回拓扑序节点列表。
func validateAndOrder(nodes []api.WorkflowNode, edges []api.WorkflowEdge) ([]dagNode, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("工作流为空，请先在画布添加节点")
	}
	byID := make(map[string]dagNode, len(nodes))
	triggers := 0
	for _, n := range nodes {
		t := nodeTypeOf(n)
		if t == "" {
			return nil, fmt.Errorf("节点 %s 缺少类型", n.ID)
		}
		if t == "trigger_alarm" {
			triggers++
		}
		byID[n.ID] = dagNode{ID: n.ID, Type: t, Data: n.Data}
	}
	if triggers == 0 {
		return nil, fmt.Errorf("至少需要一个「报警触发」节点")
	}
	if triggers > 1 {
		return nil, fmt.Errorf("只能有一个触发节点")
	}

	indeg := make(map[string]int, len(nodes))
	adj := make(map[string][]string, len(nodes))
	for id := range byID {
		indeg[id] = 0
	}
	for _, e := range edges {
		if _, ok := byID[e.Source]; !ok {
			return nil, fmt.Errorf("边引用未知源节点 %s", e.Source)
		}
		if _, ok := byID[e.Target]; !ok {
			return nil, fmt.Errorf("边引用未知目标节点 %s", e.Target)
		}
		adj[e.Source] = append(adj[e.Source], e.Target)
		indeg[e.Target]++
	}

	var q []string
	for id, d := range indeg {
		if d == 0 {
			q = append(q, id)
		}
	}
	sort.Strings(q)
	order := make([]dagNode, 0, len(nodes))
	for len(q) > 0 {
		id := q[0]
		q = q[1:]
		order = append(order, byID[id])
		for _, to := range adj[id] {
			indeg[to]--
			if indeg[to] == 0 {
				q = append(q, to)
				sort.Strings(q)
			}
		}
	}
	if len(order) != len(nodes) {
		return nil, fmt.Errorf("工作流存在环，无法执行")
	}
	return order, nil
}

func nodeTypeOf(n api.WorkflowNode) string {
	if n.Type != "" && n.Type != "workflow" && n.Type != "default" {
		// React Flow 可能把 type 设为自定义组件名；优先 Data.type
	}
	if n.Data != nil {
		if t, ok := n.Data["type"].(string); ok && strings.TrimSpace(t) != "" {
			return strings.TrimSpace(t)
		}
	}
	return strings.TrimSpace(n.Type)
}

func nodeConfig(n dagNode) map[string]any {
	if n.Data == nil {
		return map[string]any{}
	}
	if c, ok := n.Data["config"].(map[string]any); ok && c != nil {
		return c
	}
	// 兼容旧图：配置字段直接挂在 data 上
	skip := map[string]bool{"type": true, "label": true, "description": true, "runState": true, "configInvalid": true}
	out := map[string]any{}
	for k, v := range n.Data {
		if skip[k] {
			continue
		}
		out[k] = v
	}
	return out
}

func nodeLabel(n dagNode) string {
	if n.Data != nil {
		if l, ok := n.Data["label"].(string); ok && l != "" {
			return l
		}
	}
	return n.Type
}

// predecessors 返回指向 id 的源节点。
func predecessors(id string, edges []api.WorkflowEdge) []string {
	var out []string
	for _, e := range edges {
		if e.Target == id {
			out = append(out, e.Source)
		}
	}
	return out
}

func (s *Service) runWorkflowGraph(workflowID string, nodes []api.WorkflowNode, edges []api.WorkflowEdge, input map[string]any) (string, []api.DebugEvent, error) {
	order, err := validateAndOrder(nodes, edges)
	if err != nil {
		return "", nil, err
	}
	order = filterReachableFromTrigger(order, edges)
	runID := "run-" + uuid.NewString()[:12]
	ctx := &runContext{
		WorkflowID: workflowID,
		RunID:      runID,
		Input:      input,
		Vars:       map[string]any{},
	}
	if input == nil {
		ctx.Input = map[string]any{}
	}
	ctx.ImageJPEG = loadInputImageJPEG(ctx.Input)
	ctx.emit("workflow_start", "", fmt.Sprintf("开始执行工作流 %s", workflowID), map[string]any{
		"run_id":     runID,
		"nodes":      len(order),
		"has_image":  len(ctx.ImageJPEG) > 0,
		"image_bytes": len(ctx.ImageJPEG),
	})

	ok := true
	for _, n := range order {
		label := nodeLabel(n)
		ctx.emit("node_start", n.ID, fmt.Sprintf("执行「%s」(%s)", label, n.Type), nil)
		out, err := s.execNode(ctx, n, predecessors(n.ID, edges))
		if err != nil {
			ok = false
			ctx.emit("node_error", n.ID, err.Error(), nil)
			ctx.emit("workflow_complete", "", "工作流失败", map[string]any{"ok": false})
			return runID, ctx.Events, err
		}
		ctx.Vars[n.ID] = out
		ctx.emit("node_output", n.ID, fmt.Sprintf("「%s」输出", label), out)
		ctx.emit("node_complete", n.ID, fmt.Sprintf("「%s」完成", label), nil)
	}
	ctx.emit("workflow_complete", "", "工作流完成", map[string]any{"ok": ok})
	return runID, ctx.Events, nil
}

func (s *Service) execNode(ctx *runContext, n dagNode, preds []string) (map[string]any, error) {
	cfg := nodeConfig(n)
	switch n.Type {
	case "trigger_alarm":
		return execTriggerAlarm(ctx)
	case "camera_read":
		return execCameraRead(ctx, cfg)
	case "infer_detect":
		return s.execInferDetect(ctx, cfg, preds)
	case "filter_zone":
		return execFilterZone(ctx, cfg, preds)
	case "notify":
		return execNotify(cfg, preds, ctx)
	case "save_file":
		return execSaveFile(workflowResultDir(), ctx, preds)
	default:
		return nil, fmt.Errorf("未知节点类型：%s", n.Type)
	}
}

// filterReachableFromTrigger 只保留从 trigger_alarm 沿边可达的节点（拓扑序内）。
func filterReachableFromTrigger(order []dagNode, edges []api.WorkflowEdge) []dagNode {
	triggerID := ""
	for _, n := range order {
		if n.Type == "trigger_alarm" {
			triggerID = n.ID
			break
		}
	}
	if triggerID == "" {
		return order
	}
	adj := make(map[string][]string, len(edges))
	for _, e := range edges {
		adj[e.Source] = append(adj[e.Source], e.Target)
	}
	seen := map[string]bool{triggerID: true}
	q := []string{triggerID}
	for len(q) > 0 {
		id := q[0]
		q = q[1:]
		for _, to := range adj[id] {
			if seen[to] {
				continue
			}
			seen[to] = true
			q = append(q, to)
		}
	}
	out := make([]dagNode, 0, len(order))
	for _, n := range order {
		if seen[n.ID] {
			out = append(out, n)
		}
	}
	return out
}

// loadInputImageJPEG 从报警证据路径或 input.image_jpeg 注入运行时图像。
func loadInputImageJPEG(input map[string]any) []byte {
	if input == nil {
		return nil
	}
	if raw, ok := input["image_jpeg"].([]byte); ok && len(raw) > 0 {
		return raw
	}
	alarm, _ := input["alarm"].(map[string]any)
	if alarm == nil {
		return nil
	}
	candidates := []string{}
	if p, ok := alarm["evidencePath"].(string); ok && strings.TrimSpace(p) != "" {
		candidates = append(candidates, strings.TrimSpace(p))
	}
	if arr, ok := alarm["evidenceFileIds"].([]string); ok {
		candidates = append(candidates, arr...)
	}
	if arr, ok := alarm["evidenceFileIds"].([]any); ok {
		for _, v := range arr {
			if s, ok := v.(string); ok && s != "" {
				candidates = append(candidates, s)
			}
		}
	}
	for _, p := range candidates {
		if b := readEvidenceJPEGFile(p); len(b) > 0 {
			return b
		}
	}
	return nil
}

func readEvidenceJPEGFile(rel string) []byte {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return nil
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	slash := filepath.ToSlash(clean)
	if !strings.HasPrefix(slash, "data/alarm-evidence/") || strings.Contains(slash, "..") {
		return nil
	}
	b, err := os.ReadFile(clean)
	if err != nil || len(b) == 0 {
		return nil
	}
	return b
}

func mergePredOutputs(ctx *runContext, preds []string) map[string]any {
	out := map[string]any{}
	for _, p := range preds {
		if v, ok := ctx.Vars[p].(map[string]any); ok {
			for k, val := range v {
				out[k] = val
			}
		}
	}
	return out
}

// —— 调试运行缓冲（供 SSE）——

type debugRun struct {
	Events []api.DebugEvent
	Done   bool
	Err    string
	ch     chan api.DebugEvent
}

var (
	debugRuns   = map[string]*debugRun{}
	debugRunsMu sync.Mutex
)

func storeDebugRun(runID string, events []api.DebugEvent) {
	debugRunsMu.Lock()
	defer debugRunsMu.Unlock()
	ch := make(chan api.DebugEvent, len(events)+4)
	dr := &debugRun{Events: append([]api.DebugEvent{}, events...), Done: true, ch: ch}
	for _, ev := range events {
		ch <- ev
	}
	close(ch)
	debugRuns[runID] = dr
	// 清理过期
	go func() {
		time.Sleep(5 * time.Minute)
		debugRunsMu.Lock()
		delete(debugRuns, runID)
		debugRunsMu.Unlock()
	}()
}

func getDebugRun(runID string) *debugRun {
	debugRunsMu.Lock()
	defer debugRunsMu.Unlock()
	return debugRuns[runID]
}

// DebugStream 将已缓冲的调试事件回放给 SSE。
func (s *Service) DebugStream(runID string, send func(api.DebugEvent) error) error {
	dr := getDebugRun(runID)
	if dr == nil {
		return fmt.Errorf("run_id 不存在或已过期")
	}
	for _, ev := range dr.Events {
		if err := send(ev); err != nil {
			return err
		}
	}
	return nil
}
