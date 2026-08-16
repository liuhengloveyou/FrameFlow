package aibiz

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/liuhengloveyou/FrameFlow/internal/app"
	"github.com/liuhengloveyou/FrameFlow/internal/streams"
	"github.com/liuhengloveyou/FrameFlow/internal/vision"
	"github.com/liuhengloveyou/FrameFlow/pkg/core"
	"github.com/liuhengloveyou/FrameFlow/pkg/magic"
	"github.com/liuhengloveyou/FrameFlow/pkg/mjpeg"
	"github.com/liuhengloveyou/FrameFlow/pkg/utils"
)

func execTriggerAlarm(ctx *runContext) (map[string]any, error) {
	alarm := map[string]any{}
	if a, ok := ctx.Input["alarm"].(map[string]any); ok {
		alarm = a
	} else {
		for k, v := range ctx.Input {
			alarm[k] = v
		}
		if len(alarm) == 0 {
			alarm = map[string]any{
				"title":      "调试报警",
				"level":      "warn",
				"message":    "workflow debug trigger",
				"cameraId":   "",
				"occurredAt": utils.NowISO(),
			}
		}
	}
	return map[string]any{"alarm": alarm}, nil
}

func execCameraRead(ctx *runContext, cfg map[string]any) (map[string]any, error) {
	cam, _ := cfg["camera_info"].(map[string]any)
	if cam == nil {
		return nil, fmt.Errorf("请配置摄像头")
	}
	id, _ := cam["id"].(string)
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("摄像头无效")
	}
	scheme, _ := cfg["read_scheme"].(string)
	if scheme == "" {
		scheme = "keyframe"
	}
	name, _ := cam["name"].(string)

	note := "使用已有输入图像"
	got := len(ctx.ImageJPEG) > 0
	if !got {
		if jpeg, srcNote, err := pullCameraJPEG(id, scheme); err == nil && len(jpeg) > 0 {
			ctx.ImageJPEG = jpeg
			got = true
			note = srcNote
		} else if err != nil {
			note = "取帧失败：" + err.Error()
		} else {
			note = "未取到帧（流未在线或非 JPEG 源）；可用报警证据图或调试注入"
		}
	}

	return map[string]any{
		"frame": map[string]any{
			"cameraId":   id,
			"name":       name,
			"scheme":     scheme,
			"at":         utils.NowISO(),
			"hasImage":   got,
			"imageBytes": len(ctx.ImageJPEG),
			"note":       note,
		},
		"cameraId": id,
		"hasImage": got,
	}, nil
}

// pullCameraJPEG 尽力从在线流拉一帧 JPEG（短超时）。H264/H265 暂不在此解码。
func pullCameraJPEG(cameraID, scheme string) ([]byte, string, error) {
	_ = scheme
	cameraID = strings.TrimSpace(cameraID)
	if cameraID == "" {
		return nil, "", fmt.Errorf("空摄像头 ID")
	}
	names := streams.GetAllNames()
	var candidates []string
	for _, n := range names {
		if strings.Contains(n, cameraID) {
			candidates = append(candidates, n)
		}
	}
	if len(candidates) == 0 {
		return nil, "", fmt.Errorf("无匹配在线流")
	}
	for _, streamKey := range candidates {
		stream := streams.Get(streamKey)
		if stream == nil {
			continue
		}
		kf := magic.NewVideoKeyframeConsumer()
		if err := stream.AddConsumer(kf); err != nil {
			continue
		}
		type result struct {
			b []byte
		}
		ch := make(chan result, 1)
		go func() {
			once := &core.OnceBuffer{}
			_, _ = kf.WriteTo(once)
			ch <- result{b: once.Buffer()}
		}()
		var raw []byte
		select {
		case r := <-ch:
			raw = r.b
		case <-time.After(4 * time.Second):
			stream.RemoveConsumer(kf)
			continue
		}
		stream.RemoveConsumer(kf)
		if len(raw) == 0 {
			continue
		}
		codec := kf.CodecName()
		switch codec {
		case core.CodecJPEG, "":
			// 空 codec 时若像 JPEG 则尝试
			fixed := mjpeg.FixJPEG(raw)
			if len(fixed) > 2 && fixed[0] == 0xff && fixed[1] == 0xd8 {
				return fixed, "从流 " + streamKey + " 拉取 JPEG", nil
			}
			if codec == core.CodecJPEG {
				return fixed, "从流 " + streamKey + " 拉取 JPEG", nil
			}
			continue
		default:
			continue
		}
	}
	return nil, "", fmt.Errorf("候选流均未拿到 JPEG 帧")
}

func (s *Service) execInferDetect(ctx *runContext, cfg map[string]any, preds []string) (map[string]any, error) {
	modelID, _ := cfg["model_id"].(string)
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return nil, fmt.Errorf("请填写模型 ID")
	}
	conf := 0.4
	if v, ok := cfg["conf"].(float64); ok && v > 0 {
		conf = v
	}
	m, err := s.store.getModel(modelID)
	if err != nil {
		return nil, fmt.Errorf("模型不存在：%s", modelID)
	}
	path := resolveModelPath(m.LocalPath)
	if path == "" || !fileExists(path) {
		return nil, fmt.Errorf("模型本地文件不可用：%s", modelID)
	}
	jpeg := ctx.ImageJPEG
	if len(jpeg) == 0 {
		return map[string]any{
			"detections": []any{},
			"modelId":    modelID,
			"modelName":  m.Name,
			"note":       "无输入图像，跳过 ONNX（由报警证据或摄像头读取提供）",
		}, nil
	}
	labels := resolveModelPath(m.LabelsPath)
	t0 := time.Now()
	boxes, err := vision.DetectJPEGOnce(path, labels, jpeg, float32(conf), 0.45, 100, 0)
	if err != nil {
		return nil, fmt.Errorf("检测失败：%w", err)
	}
	dets := make([]map[string]any, 0, len(boxes))
	for _, b := range boxes {
		dets = append(dets, map[string]any{
			"cls": b.Label, "conf": b.Score,
			"box": map[string]any{"x": b.X, "y": b.Y, "w": b.W, "h": b.H},
		})
	}
	_ = preds
	return map[string]any{
		"detections": dets,
		"modelId":    modelID,
		"modelName":  m.Name,
		"latencyMs":  time.Since(t0).Milliseconds(),
		"count":      len(dets),
	}, nil
}

func execFilterZone(ctx *runContext, cfg map[string]any, preds []string) (map[string]any, error) {
	_ = ctx
	upstream := mergePredOutputs(ctx, preds)
	dets, _ := upstream["detections"].([]any)
	if dets == nil {
		if arr, ok := upstream["detections"].([]map[string]any); ok {
			for _, d := range arr {
				dets = append(dets, d)
			}
		}
	}
	minConf := 0.0
	if v, ok := cfg["min_conf"].(float64); ok {
		minConf = v
	}
	allowLabels := parseLabelAllowlist(cfg["labels"])
	zone := parseNormRect(cfg["zone"]) // x,y,w,h 归一化；nil 表示不按区域

	filtered := make([]any, 0, len(dets))
	for _, raw := range dets {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		c, _ := m["conf"].(float64)
		if c < minConf {
			continue
		}
		cls, _ := m["cls"].(string)
		if len(allowLabels) > 0 && !allowLabels[strings.ToLower(strings.TrimSpace(cls))] {
			continue
		}
		if zone != nil {
			cx, cy, okCenter := detectionCenter(m)
			if !okCenter || !zone.contains(cx, cy) {
				continue
			}
		}
		filtered = append(filtered, m)
	}
	note := "已按置信度过滤"
	if len(allowLabels) > 0 {
		note += " · 类别白名单"
	}
	if zone != nil {
		note += " · 矩形 ROI"
	}
	return map[string]any{
		"detections": filtered,
		"count":      len(filtered),
		"note":       note,
	}, nil
}

type normRect struct{ x, y, w, h float64 }

func (r *normRect) contains(px, py float64) bool {
	return px >= r.x && px <= r.x+r.w && py >= r.y && py <= r.y+r.h
}

func parseNormRect(v any) *normRect {
	m, ok := v.(map[string]any)
	if !ok || m == nil {
		return nil
	}
	x, okX := asFloat(m["x"])
	y, okY := asFloat(m["y"])
	w, okW := asFloat(m["w"])
	h, okH := asFloat(m["h"])
	if !okX || !okY || !okW || !okH || w <= 0 || h <= 0 {
		return nil
	}
	return &normRect{x: x, y: y, w: w, h: h}
}

func asFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func parseLabelAllowlist(v any) map[string]bool {
	out := map[string]bool{}
	add := func(s string) {
		for _, part := range strings.Split(s, ",") {
			p := strings.ToLower(strings.TrimSpace(part))
			if p != "" {
				out[p] = true
			}
		}
	}
	switch t := v.(type) {
	case string:
		add(t)
	case []any:
		for _, item := range t {
			if s, ok := item.(string); ok {
				add(s)
			}
		}
	case []string:
		for _, s := range t {
			add(s)
		}
	}
	return out
}

func detectionCenter(m map[string]any) (cx, cy float64, ok bool) {
	box, _ := m["box"].(map[string]any)
	if box == nil {
		return 0, 0, false
	}
	x, okX := asFloat(box["x"])
	y, okY := asFloat(box["y"])
	w, okW := asFloat(box["w"])
	h, okH := asFloat(box["h"])
	if !okX || !okY || !okW || !okH {
		return 0, 0, false
	}
	return x + w/2, y + h/2, true
}

func execNotify(cfg map[string]any, preds []string, ctx *runContext) (map[string]any, error) {
	channel, _ := cfg["channel"].(string)
	if channel == "" {
		channel = "toast"
	}
	msgTpl, _ := cfg["message"].(string)
	if msgTpl == "" {
		msgTpl = "工作流通知"
	}
	webhookURL, _ := cfg["webhook_url"].(string)
	upstream := mergePredOutputs(ctx, preds)
	var alarm map[string]any
	if a, ok := ctx.Input["alarm"].(map[string]any); ok {
		alarm = a
	}
	msg := renderNotifyMessage(msgTpl, alarm, upstream)

	payload := map[string]any{
		"channel":  channel,
		"message":  msg,
		"at":       utils.NowISO(),
		"workflow": ctx.WorkflowID,
		"run_id":   ctx.RunID,
		"context":  upstream,
	}
	if alarm != nil {
		payload["alarm"] = alarm
	}

	detail := ""
	switch channel {
	case "webhook":
		u := strings.TrimSpace(webhookURL)
		if u == "" {
			return nil, fmt.Errorf("webhook 通道需配置 webhook_url")
		}
		if _, err := url.ParseRequestURI(u); err != nil {
			return nil, fmt.Errorf("webhook_url 无效：%w", err)
		}
		body, _ := json.Marshal(payload)
		req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{Timeout: 8 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("webhook 请求失败：%w", err)
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		if resp.StatusCode >= 300 {
			return nil, fmt.Errorf("webhook HTTP %d", resp.StatusCode)
		}
		detail = fmt.Sprintf("POST %s → %d", u, resp.StatusCode)
	case "file":
		dir := filepath.Join("data", "workflow-notify")
		_ = os.MkdirAll(dir, 0o755)
		name := fmt.Sprintf("%s_%s.json", ctx.WorkflowID, time.Now().Format("20060102_150405"))
		path := filepath.Join(dir, name)
		b, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, b, 0o644); err != nil {
			return nil, err
		}
		detail = path
		payload["path"] = path
	default: // toast
		seq := pushWorkflowToast(payload)
		payload["seq"] = seq
		dir := filepath.Join("data", "workflow-notify")
		_ = os.MkdirAll(dir, 0o755)
		path := filepath.Join(dir, "toast_latest.json")
		b, _ := json.MarshalIndent(payload, "", "  ")
		_ = os.WriteFile(path, b, 0o644)
		detail = fmt.Sprintf("toast#%d → %s", seq, path)
	}

	app.Logger.Info().
		Str("workflow", ctx.WorkflowID).
		Str("channel", channel).
		Str("message", msg).
		Str("detail", detail).
		Msg("[workflows] notify")
	return map[string]any{"result": payload, "ok": true, "detail": detail}, nil
}

func renderNotifyMessage(tpl string, alarm, upstream map[string]any) string {
	out := tpl
	get := func(m map[string]any, key string) string {
		if m == nil {
			return ""
		}
		if v, ok := m[key]; ok && v != nil {
			return fmt.Sprint(v)
		}
		return ""
	}
	repl := map[string]string{
		"{{title}}":         get(alarm, "title"),
		"{{level}}":         get(alarm, "level"),
		"{{message}}":       get(alarm, "message"),
		"{{cameraName}}":    get(alarm, "cameraName"),
		"{{cameraId}}":      get(alarm, "cameraId"),
		"{{algorithmName}}": get(alarm, "algorithmName"),
		"{{count}}":         get(upstream, "count"),
	}
	for k, v := range repl {
		out = strings.ReplaceAll(out, k, v)
	}
	return out
}

func workflowResultDir() string {
	dir := filepath.Join("data", "workflow-results")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

func execSaveFile(dir string, ctx *runContext, preds []string) (map[string]any, error) {
	upstream := mergePredOutputs(ctx, preds)
	name := fmt.Sprintf("%s_%s.json", ctx.WorkflowID, time.Now().Format("20060102_150405"))
	path := filepath.Join(dir, name)
	b, err := json.MarshalIndent(map[string]any{
		"run_id":   ctx.RunID,
		"workflow": ctx.WorkflowID,
		"at":       utils.NowISO(),
		"data":     upstream,
	}, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return nil, err
	}
	return map[string]any{"path": path, "ok": true}, nil
}
