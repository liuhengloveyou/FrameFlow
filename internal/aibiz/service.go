package aibiz

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/liuhengloveyou/FrameFlow/internal/api"
	"github.com/liuhengloveyou/FrameFlow/internal/app"
	"github.com/liuhengloveyou/FrameFlow/internal/vision"
	"github.com/liuhengloveyou/FrameFlow/pkg/utils"
	"gorm.io/gorm"
)

// Service AI 管理面业务。
type Service struct {
	store *Store
}

func (s *Service) modelToDTO(row ModelRow) api.ModelItem {
	return api.ModelItem{
		ID:             row.ID,
		Name:           row.Name,
		Description:    row.Description,
		Tags:           parseStringSlice(row.TagsJSON),
		Formats:        parseStringSlice(row.FormatsJSON),
		LicenseStatus:  row.LicenseStatus,
		ModelStatus:    row.ModelStatus,
		Version:        row.Version,
		PublishedAt:    row.PublishedAt,
		LocalVersion:   row.LocalVersion,
		LocalUpdatedAt: row.LocalUpdatedAt,
		RuntimeScale:   row.RuntimeScale,
		FileSizeBytes:  row.FileSizeBytes,
		FileName:       row.FileName,
		LocalPath:      row.LocalPath,
		LabelsPath:     row.LabelsPath,
	}
}

func (s *Service) ListModels() (items []api.ModelItem, total int, err error) {
	rows, err := s.store.listModels()
	if err != nil {
		return nil, 0, err
	}
	items = make([]api.ModelItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, s.modelToDTO(r))
	}
	return items, len(items), nil
}

func (s *Service) RegisterModel(in api.ModelRegisterInput) (api.ModelItem, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return api.ModelItem{}, fmt.Errorf("请填写模型名称")
	}
	if len(in.Formats) == 0 {
		return api.ModelItem{}, fmt.Errorf("请至少选择一种格式")
	}
	ver := strings.TrimSpace(in.Version)
	if ver == "" {
		ver = "1.0.0"
	}
	localPath := strings.TrimSpace(in.LocalPath)
	if localPath == "" && strings.TrimSpace(in.FileName) != "" {
		localPath = filepath.Join(vision.ModelsDataDir(), filepath.Base(in.FileName))
	}
	localPath = resolveModelPath(localPath)
	labels := resolveModelPath(strings.TrimSpace(in.LabelsPath))
	if vision.IsForbiddenModelPath(localPath) || (labels != "" && vision.IsForbiddenModelPath(labels)) {
		return api.ModelItem{}, fmt.Errorf("模型文件必须位于 %s/ 目录下，请通过上传接入", vision.ModelsDataDir())
	}

	now := utils.NowISO()
	var localVer *string
	var localAt *string
	status := "remote"
	if fileExists(localPath) {
		if err := vision.ValidateModelFile(localPath, labels, in.FileSizeBytes); err != nil {
			return api.ModelItem{}, fmt.Errorf("模型校验失败：%w", err)
		}
		localVer = &ver
		localAt = &now
		status = "ready"
		if in.FileSizeBytes <= 0 {
			if st, err := os.Stat(localPath); err == nil {
				in.FileSizeBytes = st.Size()
			}
		}
	}

	row := &ModelRow{
		ID:             newID("m-"),
		Name:           name,
		Description:    strings.TrimSpace(in.Description),
		TagsJSON:       mustJSON(in.Tags),
		FormatsJSON:    mustJSON(in.Formats),
		LicenseStatus:  "authorized",
		ModelStatus:    status,
		Version:        ver,
		PublishedAt:    now,
		LocalVersion:   localVer,
		LocalUpdatedAt: localAt,
		RuntimeScale:   1,
		FileName:       strings.TrimSpace(in.FileName),
		FileSizeBytes:  in.FileSizeBytes,
		LocalPath:      localPath,
		LabelsPath:     labels,
	}
	if row.Description == "" {
		row.Description = "本地接入模型"
	}
	if err := s.store.insertModel(row); err != nil {
		return api.ModelItem{}, err
	}
	_ = s.store.insertUpdate(&ModelUpdateRow{
		ID:             newID("u-"),
		ModelName:      row.Name,
		CurrentVersion: "-",
		TargetVersion:  ver,
		OperatedAt:     now,
		Status:         "success",
		Message:        fmt.Sprintf("已登记模型 · %s", row.FileName),
	})
	return s.modelToDTO(*row), nil
}

func (s *Service) DownloadModel(id string) (api.ModelItem, error) {
	row, err := s.store.getModel(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return api.ModelItem{}, fmt.Errorf("模型不存在")
		}
		return api.ModelItem{}, err
	}
	if row.LicenseStatus == "expired" || row.LicenseStatus == "disabled" {
		if row.LicenseStatus == "expired" {
			return api.ModelItem{}, fmt.Errorf("授权已到期，无法下载")
		}
		return api.ModelItem{}, fmt.Errorf("授权已关闭，无法下载")
	}
	path := resolveModelPath(row.LocalPath)
	if !fileExists(path) {
		return api.ModelItem{}, fmt.Errorf("本地文件不存在：%s（请先放置 ONNX 或登记正确路径）", row.LocalPath)
	}
	if vision.IsForbiddenModelPath(path) {
		return api.ModelItem{}, fmt.Errorf("模型文件必须位于 %s/ 目录下，请通过上传接入", vision.ModelsDataDir())
	}
	if err := vision.ValidateModelFile(path, resolveModelPath(row.LabelsPath), 0); err != nil {
		return api.ModelItem{}, fmt.Errorf("模型校验失败：%w", err)
	}
	now := utils.NowISO()
	ver := row.Version
	prev := "-"
	if row.LocalVersion != nil {
		prev = *row.LocalVersion
	}
	row.LocalVersion = &ver
	row.LocalUpdatedAt = &now
	row.ModelStatus = "ready"
	row.LocalPath = path
	if row.FileSizeBytes <= 0 {
		if st, e := os.Stat(path); e == nil {
			row.FileSizeBytes = st.Size()
		}
	}
	if err := s.store.saveModel(&row); err != nil {
		return api.ModelItem{}, err
	}
	_ = s.store.insertUpdate(&ModelUpdateRow{
		ID:             newID("u-"),
		ModelName:      row.Name,
		CurrentVersion: prev,
		TargetVersion:  ver,
		OperatedAt:     now,
		Status:         "success",
		Message:        "本地文件校验完成",
	})
	s.ensureORTReady()
	return s.modelToDTO(row), nil
}

func (s *Service) RemoveLocalModel(id string) (api.ModelItem, error) {
	row, err := s.store.getModel(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return api.ModelItem{}, fmt.Errorf("模型不存在")
		}
		return api.ModelItem{}, err
	}
	if row.LocalVersion == nil {
		return api.ModelItem{}, fmt.Errorf("本地无缓存可清除")
	}
	prev := *row.LocalVersion
	removeManagedModelFiles(row.LocalPath)
	vision.UnloadYOLOModel(resolveModelPath(row.LocalPath))
	now := utils.NowISO()
	row.LocalVersion = nil
	row.LocalUpdatedAt = nil
	row.ModelStatus = "remote"
	row.FileSizeBytes = 0
	// 托管目录已删；保留原路径字段便于重新下载提示
	if err := s.store.saveModel(&row); err != nil {
		return api.ModelItem{}, err
	}
	_ = s.store.insertUpdate(&ModelUpdateRow{
		ID:             newID("u-"),
		ModelName:      row.Name,
		CurrentVersion: prev,
		TargetVersion:  "-",
		OperatedAt:     now,
		Status:         "success",
		Message:        "已清除本地模型文件/标记",
	})
	return s.modelToDTO(row), nil
}

func (s *Service) CheckModelHealth(id string) (api.ModelHealthResult, error) {
	row, err := s.store.getModel(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return api.ModelHealthResult{}, fmt.Errorf("模型不存在")
		}
		return api.ModelHealthResult{}, err
	}
	t0 := time.Now()
	if row.LicenseStatus != "authorized" {
		return api.ModelHealthResult{OK: false, Message: "授权异常，无法加载", LatencyMs: 0}, nil
	}
	if row.LocalVersion == nil {
		return api.ModelHealthResult{OK: false, Message: "本地未下载，请先下载模型", LatencyMs: 0}, nil
	}
	path := resolveModelPath(row.LocalPath)
	if !fileExists(path) {
		return api.ModelHealthResult{
			OK:        false,
			Message:   fmt.Sprintf("文件缺失：%s", row.LocalPath),
			LatencyMs: int(time.Since(t0).Milliseconds()),
		}, nil
	}
	ms := int(time.Since(t0).Milliseconds())
	if ms < 1 {
		ms = 1
	}
	return api.ModelHealthResult{
		OK:        true,
		Message:   fmt.Sprintf("本地权重可用 · %s", path),
		LatencyMs: ms,
	}, nil
}

func (s *Service) ScaleModel(id string, scale float64) (api.ModelItem, error) {
	row, err := s.store.getModel(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return api.ModelItem{}, fmt.Errorf("模型不存在")
		}
		return api.ModelItem{}, err
	}
	if row.LocalVersion == nil {
		return api.ModelItem{}, fmt.Errorf("本地未下载，无法调整缩放")
	}
	if scale < 0.25 || scale > 4 {
		return api.ModelItem{}, fmt.Errorf("缩放范围须在 0.25～4")
	}
	now := utils.NowISO()
	row.RuntimeScale = float64(int(scale*100+0.5)) / 100
	row.LocalUpdatedAt = &now
	if err := s.store.saveModel(&row); err != nil {
		return api.ModelItem{}, err
	}
	_ = s.store.insertUpdate(&ModelUpdateRow{
		ID:             newID("u-"),
		ModelName:      row.Name,
		CurrentVersion: *row.LocalVersion,
		TargetVersion:  *row.LocalVersion,
		OperatedAt:     now,
		Status:         "success",
		Message:        fmt.Sprintf("运行缩放调整为 %.2gx", row.RuntimeScale),
	})
	return s.modelToDTO(row), nil
}

func (s *Service) DeleteModel(id string) error {
	n, err := s.store.countAlgorithmsByModel(id)
	if err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("仍有 %d 个算法引用该模型，无法删除", n)
	}
	row, err := s.store.getModel(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("模型不存在")
		}
		return err
	}
	removeManagedModelFiles(row.LocalPath)
	if err := s.store.deleteModel(id); err != nil {
		return err
	}
	cur := row.Version
	if row.LocalVersion != nil {
		cur = *row.LocalVersion
	}
	_ = s.store.insertUpdate(&ModelUpdateRow{
		ID:             newID("u-"),
		ModelName:      row.Name,
		CurrentVersion: cur,
		TargetVersion:  "-",
		OperatedAt:     utils.NowISO(),
		Status:         "success",
		Message:        "已从模型仓库删除",
	})
	return nil
}

func (s *Service) ListModelUpdates() ([]api.ModelUpdateRecord, error) {
	rows, err := s.store.listUpdates(100)
	if err != nil {
		return nil, err
	}
	out := make([]api.ModelUpdateRecord, 0, len(rows))
	for _, r := range rows {
		out = append(out, api.ModelUpdateRecord{
			ID:             r.ID,
			ModelName:      r.ModelName,
			CurrentVersion: r.CurrentVersion,
			TargetVersion:  r.TargetVersion,
			OperatedAt:     r.OperatedAt,
			Status:         r.Status,
			Message:        r.Message,
		})
	}
	return out, nil
}

func (s *Service) algoToDTO(row AlgorithmRow, modelName string) api.AlgorithmCapability {
	bound := row.BoundCameraCount
	if n, err := s.store.countDistinctCamerasByAlgorithm(row.ID); err == nil {
		bound = int(n)
	}
	return api.AlgorithmCapability{
		ID:               row.ID,
		Name:             row.Name,
		Description:      row.Description,
		Category:         row.Category,
		ModelID:          row.ModelID,
		ModelName:        modelName,
		Status:           row.Status,
		BoundCameraCount: bound,
		Tags:             parseStringSlice(row.TagsJSON),
		Params:           parseAlgorithmParams(row.ParamsJSON),
		UpdatedAt:        utils.FormatISO(row.UpdatedAt),
	}
}

func (s *Service) modelName(id string) string {
	row, err := s.store.getModel(id)
	if err != nil {
		return id
	}
	return row.Name
}

func (s *Service) ListAlgorithms(q, category, status string) (api.AlgorithmPage, error) {
	rows, err := s.store.listAlgorithms()
	if err != nil {
		return api.AlgorithmPage{}, err
	}
	q = strings.ToLower(strings.TrimSpace(q))
	items := make([]api.AlgorithmCapability, 0, len(rows))
	for _, r := range rows {
		if category != "" && r.Category != category {
			continue
		}
		if status != "" && r.Status != status {
			continue
		}
		dto := s.algoToDTO(r, s.modelName(r.ModelID))
		if q != "" {
			blob := strings.ToLower(dto.Name + " " + dto.Description + " " + strings.Join(dto.Tags, " "))
			if !strings.Contains(blob, q) {
				continue
			}
		}
		items = append(items, dto)
	}
	return api.AlgorithmPage{Items: items, Total: len(items), Page: 1, PageSize: len(items)}, nil
}

func (s *Service) CreateAlgorithm(in api.AlgorithmInput) (api.AlgorithmCapability, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return api.AlgorithmCapability{}, fmt.Errorf("请填写算法名称")
	}
	modelID := strings.TrimSpace(in.ModelID)
	if modelID == "" {
		return api.AlgorithmCapability{}, fmt.Errorf("请选择关联模型")
	}
	m, err := s.store.getModel(modelID)
	if err != nil {
		return api.AlgorithmCapability{}, fmt.Errorf("关联模型不存在")
	}
	st := strings.TrimSpace(in.Status)
	if st == "" {
		st = "draft"
	}
	switch st {
	case "ready", "draft", "disabled":
	default:
		return api.AlgorithmCapability{}, fmt.Errorf("无效状态")
	}
	params := normalizeAlgorithmParams(in.Params)
	row := &AlgorithmRow{
		ID:               newID("alg-"),
		Name:             name,
		Description:      strings.TrimSpace(in.Description),
		Category:         strings.TrimSpace(in.Category),
		ModelID:          modelID,
		Status:           st,
		BoundCameraCount: 0,
		TagsJSON:         mustJSON(in.Tags),
		ParamsJSON:       mustJSON(params),
	}
	if err := s.store.insertAlgorithm(row); err != nil {
		return api.AlgorithmCapability{}, err
	}
	return s.algoToDTO(*row, m.Name), nil
}

func (s *Service) UpdateAlgorithm(id string, in api.AlgorithmInput) (api.AlgorithmCapability, error) {
	row, err := s.store.getAlgorithm(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return api.AlgorithmCapability{}, fmt.Errorf("算法不存在")
		}
		return api.AlgorithmCapability{}, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return api.AlgorithmCapability{}, fmt.Errorf("请填写算法名称")
	}
	modelID := strings.TrimSpace(in.ModelID)
	if modelID == "" {
		return api.AlgorithmCapability{}, fmt.Errorf("请选择关联模型")
	}
	m, err := s.store.getModel(modelID)
	if err != nil {
		return api.AlgorithmCapability{}, fmt.Errorf("关联模型不存在")
	}
	st := strings.TrimSpace(in.Status)
	if st == "" {
		st = row.Status
	}
	switch st {
	case "ready", "draft", "disabled":
	default:
		return api.AlgorithmCapability{}, fmt.Errorf("无效状态")
	}
	row.Name = name
	row.Description = strings.TrimSpace(in.Description)
	row.Category = strings.TrimSpace(in.Category)
	row.ModelID = modelID
	row.Status = st
	row.TagsJSON = mustJSON(in.Tags)
	row.ParamsJSON = mustJSON(normalizeAlgorithmParams(in.Params))
	if err := s.store.saveAlgorithm(&row); err != nil {
		return api.AlgorithmCapability{}, err
	}
	return s.algoToDTO(row, m.Name), nil
}

func (s *Service) UpdateAlgorithmStatus(id, status string) (api.AlgorithmCapability, error) {
	row, err := s.store.getAlgorithm(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return api.AlgorithmCapability{}, fmt.Errorf("算法不存在")
		}
		return api.AlgorithmCapability{}, err
	}
	switch status {
	case "ready", "draft", "disabled":
	default:
		return api.AlgorithmCapability{}, fmt.Errorf("无效状态")
	}
	row.Status = status
	if err := s.store.saveAlgorithm(&row); err != nil {
		return api.AlgorithmCapability{}, err
	}
	return s.algoToDTO(row, s.modelName(row.ModelID)), nil
}

func (s *Service) DeleteAlgorithm(id string) error {
	n, err := s.store.countBindingsByAlgorithm(id)
	if err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("仍有 %d 路通道绑定该算法，请先解绑后再删除", n)
	}
	if _, err := s.store.getAlgorithm(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("算法不存在")
		}
		return err
	}
	return s.store.deleteAlgorithm(id)
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func (s *Service) wfSummary(row WorkflowRow) api.WorkflowSummary {
	return api.WorkflowSummary{
		ID:           row.ID,
		Name:         row.Name,
		Description:  row.Description,
		Status:       row.Status,
		NodeCount:    row.NodeCount,
		EdgeCount:    row.EdgeCount,
		CallCount:    row.CallCount,
		SuccessCount: row.SuccessCount,
		SuccessRate:  row.SuccessRate,
		LastRunAt:    row.LastRunAt,
		UpdatedAt:    utils.FormatISO(row.UpdatedAt),
	}
}

func (s *Service) wfDetail(row WorkflowRow) (api.WorkflowDetail, error) {
	var nodes []api.WorkflowNode
	var edges []api.WorkflowEdge
	if row.NodesJSON != "" {
		_ = json.Unmarshal([]byte(row.NodesJSON), &nodes)
	}
	if row.EdgesJSON != "" {
		_ = json.Unmarshal([]byte(row.EdgesJSON), &edges)
	}
	if nodes == nil {
		nodes = []api.WorkflowNode{}
	}
	if edges == nil {
		edges = []api.WorkflowEdge{}
	}
	return api.WorkflowDetail{
		WorkflowSummary: s.wfSummary(row),
		Nodes:           nodes,
		Edges:           edges,
	}, nil
}

func (s *Service) ListWorkflows() ([]api.WorkflowSummary, error) {
	rows, err := s.store.listWorkflows()
	if err != nil {
		return nil, err
	}
	out := make([]api.WorkflowSummary, 0, len(rows))
	for _, r := range rows {
		out = append(out, s.wfSummary(r))
	}
	return out, nil
}

func (s *Service) GetWorkflow(id string) (api.WorkflowDetail, error) {
	row, err := s.store.getWorkflow(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return api.WorkflowDetail{}, fmt.Errorf("工作流不存在")
		}
		return api.WorkflowDetail{}, err
	}
	return s.wfDetail(row)
}

func (s *Service) CreateWorkflow(in api.WorkflowCreateInput) (api.WorkflowSummary, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return api.WorkflowSummary{}, fmt.Errorf("请填写工作流名称")
	}
	nodes, edges := defaultWorkflowGraph()
	nodesJSON, err := json.Marshal(nodes)
	if err != nil {
		return api.WorkflowSummary{}, err
	}
	edgesJSON, err := json.Marshal(edges)
	if err != nil {
		return api.WorkflowSummary{}, err
	}
	row := &WorkflowRow{
		ID:          newID("wf-"),
		Name:        name,
		Description: strings.TrimSpace(in.Description),
		Status:      "stopped",
		NodesJSON:   string(nodesJSON),
		EdgesJSON:   string(edgesJSON),
	}
	if err := s.store.insertWorkflow(row); err != nil {
		return api.WorkflowSummary{}, err
	}
	return s.wfSummary(*row), nil
}

func (s *Service) UpdateWorkflow(id string, name, desc *string, nodes []api.WorkflowNode, edges []api.WorkflowEdge, hasNodes, hasEdges bool) (api.WorkflowDetail, error) {
	row, err := s.store.getWorkflow(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return api.WorkflowDetail{}, fmt.Errorf("工作流不存在")
		}
		return api.WorkflowDetail{}, err
	}
	if name != nil {
		n := strings.TrimSpace(*name)
		if n == "" {
			return api.WorkflowDetail{}, fmt.Errorf("名称不能为空")
		}
		row.Name = n
	}
	if desc != nil {
		row.Description = strings.TrimSpace(*desc)
	}
	if hasNodes {
		if nodes == nil {
			nodes = []api.WorkflowNode{}
		}
		row.NodesJSON = mustJSON(nodes)
		row.NodeCount = len(nodes)
	}
	if hasEdges {
		if edges == nil {
			edges = []api.WorkflowEdge{}
		}
		row.EdgesJSON = mustJSON(edges)
		row.EdgeCount = len(edges)
	}
	if err := s.store.saveWorkflow(&row); err != nil {
		return api.WorkflowDetail{}, err
	}
	return s.wfDetail(row)
}

func (s *Service) DeleteWorkflow(id string) error {
	row, err := s.store.getWorkflow(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("工作流不存在")
		}
		return err
	}
	if row.Status == "running" {
		return fmt.Errorf("请先停止工作流再删除")
	}
	return s.store.deleteWorkflow(id)
}

func (s *Service) StartWorkflow(id string) (api.WorkflowSummary, error) {
	row, err := s.store.getWorkflow(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return api.WorkflowSummary{}, fmt.Errorf("工作流不存在")
		}
		return api.WorkflowSummary{}, err
	}
	detail, err := s.wfDetail(row)
	if err != nil {
		return api.WorkflowSummary{}, err
	}
	if _, err := validateAndOrder(detail.Nodes, detail.Edges); err != nil {
		return api.WorkflowSummary{}, err
	}
	now := utils.NowISO()
	row.Status = "running"
	row.LastRunAt = &now
	if err := s.store.saveWorkflow(&row); err != nil {
		return api.WorkflowSummary{}, err
	}
	s.runtimeSet(id, true)
	return s.wfSummary(row), nil
}

func (s *Service) ListNodeSpecs() []api.NodeSpec {
	return DefaultNodeSpecs()
}

func (s *Service) StopWorkflow(id string) (api.WorkflowSummary, error) {
	row, err := s.store.getWorkflow(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return api.WorkflowSummary{}, fmt.Errorf("工作流不存在")
		}
		return api.WorkflowSummary{}, err
	}
	row.Status = "stopped"
	if err := s.store.saveWorkflow(&row); err != nil {
		return api.WorkflowSummary{}, err
	}
	s.runtimeSet(id, false)
	return s.wfSummary(row), nil
}

func (s *Service) DebugWorkflow(id string, in api.WorkflowDebugInput) (string, []api.DebugEvent, error) {
	row, err := s.store.getWorkflow(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil, fmt.Errorf("工作流不存在")
		}
		return "", nil, err
	}
	detail, err := s.wfDetail(row)
	if err != nil {
		return "", nil, err
	}
	input := map[string]any{}
	switch in.InputMode {
	case "custom":
		for k, v := range in.CustomInput {
			input[k] = v
		}
		input["alarm"] = map[string]any{
			"title":   "自定义调试",
			"level":   "info",
			"message": "custom debug input",
			"extra":   in.CustomInput,
		}
	default: // latest_alarm
		input["alarm"] = s.latestAlarmInput()
	}
	runID, events, err := s.runWorkflowGraph(id, detail.Nodes, detail.Edges, input)
	if err != nil && len(events) == 0 {
		return runID, events, err
	}
	// 更新调用统计
	row.CallCount++
	if err == nil {
		row.SuccessCount++
	}
	if row.CallCount > 0 {
		row.SuccessRate = float64(row.SuccessCount) / float64(row.CallCount)
	}
	now := utils.NowISO()
	row.LastRunAt = &now
	_ = s.store.saveWorkflow(&row)

	storeDebugRun(runID, events)
	if err != nil {
		// 仍返回事件便于前端高亮错误节点
		return runID, events, nil
	}
	return runID, events, nil
}

func (s *Service) latestAlarmInput() map[string]any {
	rows, _, err := s.store.listAlarms(api.AlarmQuery{Page: 1, PageSize: 1})
	if err != nil || len(rows) == 0 {
		return map[string]any{
			"title":      "无历史报警（调试占位）",
			"level":      "warn",
			"message":    "debug: latest_alarm — 库中暂无报警",
			"cameraId":   "",
			"occurredAt": utils.NowISO(),
		}
	}
	return alarmRecordToMap(alarmToDTO(rows[0]))
}

// runtime：已启动工作流集合（报警投递用）。
var (
	runtimeMu sync.Mutex
	runtimeOn = map[string]bool{}
)

func (s *Service) runtimeSet(id string, on bool) {
	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	if on {
		runtimeOn[id] = true
	} else {
		delete(runtimeOn, id)
	}
}

// DispatchAlarm 向 running 工作流投递报警（异步）。
// 有 workflowId：仅投递该工作流（须 running）；无联动目标则跳过（调试请用工作流 Debug）。
func (s *Service) DispatchAlarm(alarm map[string]any) {
	targetID := ""
	if alarm != nil {
		if v, ok := alarm["workflowId"].(string); ok {
			targetID = strings.TrimSpace(v)
		}
	}
	if targetID == "" {
		app.Logger.Debug().Msg("[workflows] alarm skipped: no linked workflowId")
		return
	}

	runtimeMu.Lock()
	ok := runtimeOn[targetID]
	runtimeMu.Unlock()
	if !ok {
		app.Logger.Info().
			Str("workflow", targetID).
			Msg("[workflows] alarm skipped: linked workflow not running")
		return
	}
	go s.runAlarmWorkflow(targetID, alarm)
}

func (s *Service) runAlarmWorkflow(id string, alarm map[string]any) {
	row, err := s.store.getWorkflow(id)
	if err != nil || row.Status != "running" {
		return
	}
	detail, err := s.wfDetail(row)
	if err != nil {
		return
	}
	input := map[string]any{"alarm": alarm}
	runID, events, runErr := s.runWorkflowGraph(id, detail.Nodes, detail.Edges, input)

	runtimeMu.Lock()
	row2, err2 := s.store.getWorkflow(id)
	if err2 == nil {
		row2.CallCount++
		if runErr == nil {
			row2.SuccessCount++
		}
		if row2.CallCount > 0 {
			row2.SuccessRate = float64(row2.SuccessCount) / float64(row2.CallCount)
		}
		now := utils.NowISO()
		row2.LastRunAt = &now
		_ = s.store.saveWorkflow(&row2)
	}
	runtimeMu.Unlock()

	if runID != "" {
		storeDebugRun(runID, events)
	}
	app.Logger.Info().
		Str("workflow", id).
		Str("run_id", runID).
		Err(runErr).
		Int("events", len(events)).
		Msg("[workflows] alarm dispatched")
}
