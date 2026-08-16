package aibiz

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/liuhengloveyou/FrameFlow/internal/api"
	"github.com/liuhengloveyou/FrameFlow/internal/app"
	"github.com/liuhengloveyou/FrameFlow/internal/vision"
	"gorm.io/gorm"
)

func detectRuleSchema() []api.AlgorithmRuleField {
	min0, max1, step := 0.0, 1.0, 0.01
	max3600, step1 := 3600.0, 1.0
	minDet, maxDet := 1.0, 500.0
	minSample, maxSample, stepSample := 50.0, 5000.0, 50.0
	minGop, maxGop, stepGop := 1.0, 64.0, 1.0
	return []api.AlgorithmRuleField{
		{
			Key: "conf", Label: "置信度阈值", Type: "number", Group: "检测",
			Description: "低于该值的检测框将被过滤（0～1）",
			Min:         &min0, Max: &max1, Step: &step, Required: true,
		},
		{
			Key: "iou", Label: "NMS IoU", Type: "number", Group: "检测",
			Description: "非极大值抑制交并比（0～1）",
			Min:         &min0, Max: &max1, Step: &step, Required: true,
		},
		{
			Key: "max_det", Label: "最大检测数", Type: "number", Group: "检测",
			Description: "单帧最多保留的检测框数量",
			Min:         &minDet, Max: &maxDet, Step: &step1, Required: true,
		},
		{
			Key: "pull_keyframe_only", Label: "仅关键帧取样", Type: "boolean", Group: "采样",
			Description: "开启后只拉 IDR；关闭则按间隔从 GOP 内取样（含 P 帧，叠框更密）",
		},
		{
			Key: "pull_sample_ms", Label: "检测间隔（毫秒）", Type: "number", Group: "采样",
			Description: "相邻两次取帧/检测的最小间隔（取帧节流与检测循环共用）",
			Min:         &minSample, Max: &maxSample, Step: &stepSample, Required: true,
		},
		{
			Key: "pull_max_gop_mb", Label: "GOP 缓冲上限（MB）", Type: "number", Group: "采样",
			Description: "interval 模式累积 Annex-B 上限，超出则丢弃直至下一 IDR",
			Min:         &minGop, Max: &maxGop, Step: &stepGop, Required: true,
		},
		{
			Key: "enable_track", Label: "启用跟踪", Type: "boolean", Group: "跟踪",
			Description: "跨帧保持同一目标身份（当前为配置项）",
		},
		{
			Key: "alarm_level", Label: "告警级别", Type: "select", Group: "告警", Required: true,
			Description: "检出后写入告警时的默认级别",
			Options: []map[string]string{
				{"label": "提示", "value": "info"},
				{"label": "警告", "value": "warn"},
				{"label": "严重", "value": "critical"},
			},
		},
		{
			Key: "cooldown_sec", Label: "告警冷却（秒）", Type: "number", Group: "告警",
			Description: "同一绑定在冷却期内不重复产生告警",
			Min:         &min0, Max: &max3600, Step: &step1, Required: true,
		},
	}
}

func defaultBindingRules() map[string]any {
	return vision.DefaultBindingRulesMap()
}

func parseRulesMap(s string) map[string]any {
	out := map[string]any{}
	s = strings.TrimSpace(s)
	if s == "" || s == "{}" {
		return out
	}
	_ = json.Unmarshal([]byte(s), &out)
	if out == nil {
		return map[string]any{}
	}
	return out
}

func parseRegions(s string) [][]map[string]any {
	out := [][]map[string]any{}
	s = strings.TrimSpace(s)
	if s == "" || s == "[]" {
		return out
	}
	_ = json.Unmarshal([]byte(s), &out)
	if out == nil {
		return [][]map[string]any{}
	}
	return out
}

func (s *Service) refreshAlgorithmBoundCount(algorithmID string) {
	n, err := s.store.countDistinctCamerasByAlgorithm(algorithmID)
	if err != nil {
		return
	}
	row, err := s.store.getAlgorithm(algorithmID)
	if err != nil {
		return
	}
	row.BoundCameraCount = int(n)
	_ = s.store.saveAlgorithm(&row)
}

func (s *Service) bindingToDTO(row BindingRow) (api.CameraAlgorithm, error) {
	alg, err := s.store.getAlgorithm(row.AlgorithmID)
	if err != nil {
		return api.CameraAlgorithm{}, fmt.Errorf("关联算法不存在")
	}
	return api.CameraAlgorithm{
		ID:          row.ID,
		AlgorithmID: row.AlgorithmID,
		Name:        alg.Name,
		Description: alg.Description,
		Status:      row.Status,
		ModelID:     alg.ModelID,
		ChannelID:   row.ChannelID,
		CameraID:    row.CameraID,
		Rules:       parseRulesMap(row.RulesJSON),
		RuleSchema:  detectRuleSchema(),
		Regions:     parseRegions(row.RegionsJSON),
	}, nil
}

func (s *Service) ListCameraAlgorithms(cameraID, channelID string) ([]api.CameraAlgorithm, error) {
	cameraID = strings.TrimSpace(cameraID)
	channelID = strings.TrimSpace(channelID)
	if cameraID == "" || channelID == "" {
		return nil, fmt.Errorf("缺少 cameraId / channelId")
	}
	rows, err := s.store.listBindings(cameraID, channelID)
	if err != nil {
		return nil, err
	}
	out := make([]api.CameraAlgorithm, 0, len(rows))
	for _, r := range rows {
		dto, err := s.bindingToDTO(r)
		if err != nil {
			continue
		}
		out = append(out, dto)
	}
	return out, nil
}

func (s *Service) BindCameraAlgorithm(cameraID, channelID, algorithmID string) (api.CameraAlgorithm, error) {
	cameraID = strings.TrimSpace(cameraID)
	channelID = strings.TrimSpace(channelID)
	algorithmID = strings.TrimSpace(algorithmID)
	if cameraID == "" || channelID == "" || algorithmID == "" {
		return api.CameraAlgorithm{}, fmt.Errorf("请选择算法")
	}
	alg, err := s.store.getAlgorithm(algorithmID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return api.CameraAlgorithm{}, fmt.Errorf("算法不存在")
		}
		return api.CameraAlgorithm{}, err
	}
	if alg.Status == "disabled" {
		return api.CameraAlgorithm{}, fmt.Errorf("算法已停用，无法绑定")
	}
	existing, err := s.store.listBindings(cameraID, channelID)
	if err != nil {
		return api.CameraAlgorithm{}, err
	}
	for _, e := range existing {
		if e.AlgorithmID == algorithmID {
			return api.CameraAlgorithm{}, fmt.Errorf("该算法已绑定到此码流")
		}
	}
	row := &BindingRow{
		ID:          newID("bind-"),
		CameraID:    cameraID,
		ChannelID:   channelID,
		AlgorithmID: algorithmID,
		Status:      "uninitialized",
		RulesJSON:   mustJSON(defaultBindingRules()),
		RegionsJSON: "[]",
	}
	if err := s.store.insertBinding(row); err != nil {
		return api.CameraAlgorithm{}, err
	}
	s.refreshAlgorithmBoundCount(algorithmID)
	return s.bindingToDTO(*row)
}

func (s *Service) UnbindCameraAlgorithm(cameraID, channelID, bindID string) error {
	row, err := s.store.getBinding(bindID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("算法绑定不存在")
		}
		return err
	}
	if row.CameraID != cameraID || row.ChannelID != channelID {
		return fmt.Errorf("算法绑定不存在")
	}
	if row.Status == "running" {
		return fmt.Errorf("请先停止算法再解绑")
	}
	algID := row.AlgorithmID
	if err := s.store.deleteBinding(bindID); err != nil {
		return err
	}
	s.refreshAlgorithmBoundCount(algID)
	return nil
}

func (s *Service) SaveCameraAlgorithm(cameraID, channelID, bindID string, patch api.CameraAlgorithmPatch) (api.CameraAlgorithm, error) {
	row, err := s.store.getBinding(bindID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return api.CameraAlgorithm{}, fmt.Errorf("算法绑定不存在")
		}
		return api.CameraAlgorithm{}, err
	}
	if row.CameraID != cameraID || row.ChannelID != channelID {
		return api.CameraAlgorithm{}, fmt.Errorf("算法绑定不存在")
	}
	if patch.Rules != nil {
		row.RulesJSON = mustJSON(patch.Rules)
	}
	if patch.Regions != nil {
		row.RegionsJSON = mustJSON(patch.Regions)
	}
	if err := s.store.saveBinding(&row); err != nil {
		return api.CameraAlgorithm{}, err
	}
	return s.bindingToDTO(row)
}

func (s *Service) CameraAlgorithmAction(cameraID, channelID, bindID, action string) (api.CameraAlgorithm, error) {
	row, err := s.store.getBinding(bindID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return api.CameraAlgorithm{}, fmt.Errorf("算法绑定不存在")
		}
		return api.CameraAlgorithm{}, err
	}
	if row.CameraID != cameraID || row.ChannelID != channelID {
		return api.CameraAlgorithm{}, fmt.Errorf("算法绑定不存在")
	}
	switch action {
	case "init":
		row.Status = "stopped"
	case "start":
		// 未初始化时静默就绪，用户无需单独点「初始化」
		if row.Status == "uninitialized" {
			row.Status = "stopped"
		}
		if err := s.ensureBindingModelReady(row.AlgorithmID); err != nil {
			return api.CameraAlgorithm{}, err
		}
		// 同路叠框只展示一个 running 算法（产品规则）；不同摄像头可同时跑不同上传模型。
		if err := s.stopOtherRunningBindings(cameraID, channelID, bindID); err != nil {
			return api.CameraAlgorithm{}, err
		}
		row.Status = "running"
	case "stop":
		row.Status = "stopped"
	case "release":
		row.Status = "uninitialized"
	default:
		return api.CameraAlgorithm{}, fmt.Errorf("未知操作：%s", action)
	}
	if err := s.store.saveBinding(&row); err != nil {
		return api.CameraAlgorithm{}, err
	}
	return s.bindingToDTO(row)
}

// ensureBindingModelReady 启动前按算法在 DB 中关联的上传 ONNX 预热进 vision 缓存。
func (s *Service) ensureBindingModelReady(algorithmID string) error {
	alg, err := s.store.getAlgorithm(algorithmID)
	if err != nil {
		return fmt.Errorf("关联算法不存在")
	}
	if alg.Status != "ready" {
		return fmt.Errorf("算法未启用，无法启动")
	}
	m, err := s.store.getModel(alg.ModelID)
	if err != nil {
		return fmt.Errorf("关联模型不存在")
	}
	path := resolveModelPath(m.LocalPath)
	if path == "" || !fileExists(path) || vision.IsForbiddenModelPath(path) {
		return fmt.Errorf("模型本地文件不可用，请先在模型仓库上传 ONNX")
	}
	labels := resolveModelPath(m.LabelsPath)
	if err := vision.ValidateModelFile(path, labels, 0); err != nil {
		return fmt.Errorf("模型校验失败：%w", err)
	}
	// 预热 session；置信度/NMS/maxDet 以绑定 rules 为准，在流检测时按 policy 刷新
	d := vision.DefaultStreamAIPolicy()
	if err := vision.LoadYOLOModel(path, labels, "", d.Conf, 0, d.NMSIoU, d.MaxDet); err != nil {
		return fmt.Errorf("模型加载失败：%w", err)
	}
	if !vision.HasYOLOModel(path) {
		return fmt.Errorf("模型加载失败，请检查 ONNX Runtime 与模型文件")
	}
	return nil
}

// stopOtherRunningBindings 将同摄像头同通道上除 keepID 外的 running 绑定停掉。
func (s *Service) stopOtherRunningBindings(cameraID, channelID, keepID string) error {
	rows, err := s.store.listBindings(cameraID, channelID)
	if err != nil {
		return err
	}
	for i := range rows {
		r := &rows[i]
		if r.ID == keepID || r.Status != "running" {
			continue
		}
		r.Status = "stopped"
		if err := s.store.saveBinding(r); err != nil {
			return err
		}
		app.Logger.Info().
			Str("stopped_bind", r.ID).
			Str("keep_bind", keepID).
			Str("camera", cameraID).
			Str("channel", channelID).
			Msg("[aibiz] stopped other running binding on same channel")
	}
	return nil
}

// reloadRunningBindingModels 进程重启后为仍标记 running 的绑定预热各自算法模型（可多模型并存缓存）。
func (s *Service) reloadRunningBindingModels() {
	var rows []BindingRow
	if err := s.store.db.Where("status = ?", "running").Find(&rows).Error; err != nil {
		return
	}
	// 同 camera+channel 只保留一条 running（叠框产品规则）。
	keep := map[string]string{} // camera|channel -> bind id
	for i := range rows {
		r := &rows[i]
		key := r.CameraID + "|" + r.ChannelID
		if prev, ok := keep[key]; ok && prev != r.ID {
			r.Status = "stopped"
			_ = s.store.saveBinding(r)
			app.Logger.Info().
				Str("stopped_bind", r.ID).
				Str("keep_bind", prev).
				Msg("[aibiz] reload: duplicate running binding stopped")
			continue
		}
		if _, ok := keep[key]; !ok {
			keep[key] = r.ID
		}
	}
	seen := map[string]bool{}
	for _, r := range rows {
		if r.Status != "running" {
			continue
		}
		if seen[r.AlgorithmID] {
			continue
		}
		seen[r.AlgorithmID] = true
		if err := s.ensureBindingModelReady(r.AlgorithmID); err != nil {
			app.Logger.Warn().Err(err).Str("bind", r.ID).Str("algorithm", r.AlgorithmID).
				Msg("[aibiz] reload running binding model failed")
		}
	}
}

// InferTest 对上传图片或视频做真实 YOLO 推理（模型本地文件就绪时）。
// 视频会先抽一帧再检测；返回 previewJpegBase64 供前端叠框预览。
func (s *Service) InferTest(id string, imageName string, conf float64, mediaBytes []byte) (map[string]any, error) {
	log := app.Logger
	log.Info().
		Str("algorithm_id", id).
		Str("image", imageName).
		Float64("conf", conf).
		Int("image_bytes", len(mediaBytes)).
		Msg("[aibiz] infer-test begin")

	row, err := s.store.getAlgorithm(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Warn().Str("algorithm_id", id).Msg("[aibiz] infer-test: algorithm not found")
			return nil, fmt.Errorf("算法不存在")
		}
		log.Error().Err(err).Str("algorithm_id", id).Msg("[aibiz] infer-test: load algorithm")
		return nil, err
	}
	if row.Status != "ready" {
		log.Warn().Str("algorithm_id", id).Str("status", row.Status).Msg("[aibiz] infer-test: algorithm not ready")
		return nil, fmt.Errorf("算法未启用，无法推理")
	}
	if conf <= 0 {
		conf = float64(vision.DefaultConf)
	}
	if conf < 0 || conf > 1 {
		return nil, fmt.Errorf("置信度须在 0～1")
	}
	if len(mediaBytes) == 0 {
		log.Warn().Str("algorithm_id", id).Msg("[aibiz] infer-test: empty media")
		return nil, fmt.Errorf("请上传测试图片或视频")
	}
	if strings.TrimSpace(imageName) == "" {
		imageName = "upload.bin"
	}

	jpegBytes, kind, err := vision.SampleToJPEG(mediaBytes, imageName)
	if err != nil {
		log.Warn().Err(err).Str("algorithm_id", id).Str("file", imageName).
			Msg("[aibiz] infer-test: sample convert failed")
		return nil, err
	}

	m, err := s.store.getModel(row.ModelID)
	if err != nil {
		log.Warn().Err(err).Str("algorithm_id", id).Str("model_id", row.ModelID).
			Msg("[aibiz] infer-test: model missing")
		return nil, fmt.Errorf("关联模型不存在")
	}
	modelPath := resolveModelPath(m.LocalPath)
	labelsPath := resolveModelPath(m.LabelsPath)
	log.Info().
		Str("algorithm_id", id).
		Str("algorithm", row.Name).
		Str("model_id", m.ID).
		Str("model", m.Name).
		Str("model_path", modelPath).
		Str("labels_path", labelsPath).
		Str("model_status", m.ModelStatus).
		Str("sample_kind", string(kind)).
		Int("jpeg_bytes", len(jpegBytes)).
		Bool("model_file_ok", modelPath != "" && fileExists(modelPath)).
		Msg("[aibiz] infer-test: resolved model")

	if modelPath == "" || !fileExists(modelPath) {
		return nil, fmt.Errorf("模型本地文件不可用，请先在模型仓库接入/下载 ONNX")
	}

	t0 := time.Now()
	d := vision.DefaultStreamAIPolicy()
	boxes, err := vision.DetectJPEGOnce(
		modelPath,
		labelsPath,
		jpegBytes,
		float32(conf),
		d.NMSIoU,
		d.MaxDet,
		0,
	)
	latency := int(time.Since(t0).Milliseconds())
	if err != nil {
		log.Error().
			Err(err).
			Str("algorithm_id", id).
			Str("model_path", modelPath).
			Int("latency_ms", latency).
			Msg("[aibiz] infer-test: detect failed")
		return nil, fmt.Errorf("推理失败：%w", err)
	}

	const canvasW, canvasH = 800, 520
	dets := make([]map[string]any, 0, len(boxes))
	for _, b := range boxes {
		dets = append(dets, map[string]any{
			"cls":  b.Label,
			"conf": b.Score,
			"bbox": []int{
				int(b.X * canvasW),
				int(b.Y * canvasH),
				int((b.X + b.W) * canvasW),
				int((b.Y + b.H) * canvasH),
			},
		})
	}
	previewB64 := base64.StdEncoding.EncodeToString(jpegBytes)
	kindLabel := "图片"
	if kind == vision.SampleKindVideo {
		kindLabel = "视频帧"
	}
	log.Info().
		Str("algorithm_id", id).
		Int("detections", len(dets)).
		Int("latency_ms", latency).
		Str("sample_kind", string(kind)).
		Msg("[aibiz] infer-test ok")
	return map[string]any{
		"ok":                true,
		"latencyMs":         latency,
		"message":           fmt.Sprintf("基于「%s」完成推理 · %s（%s）· conf≥%.2f", m.Name, imageName, kindLabel, conf),
		"detections":        dets,
		"params":            map[string]any{"confThreshold": conf, "iouThreshold": float64(d.NMSIoU), "maxDetections": d.MaxDet},
		"sampleKind":        string(kind),
		"previewJpegBase64": previewB64,
	}, nil
}
