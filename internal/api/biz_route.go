package api

import (
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/liuhengloveyou/FrameFlow/pkg/fferrors"
	passporthttp "github.com/liuhengloveyou/passport/v3/face/http"
)

// RegisterBizAPI 注册业务路由（Go 1.22+ 方法路由）。
//
//	method: GET/POST/PUT/PATCH/DELETE；空则匹配所有方法
//	path:   "api/v1/models" 或 "api/v1/models/{id}"（相对 basePath）；
//	        绝对路径以 / 开头；空 path 表示 basePath 根（静态站等）
//	needLogin: true 时校验 Passport 会话（UID>0）；不做 Casbin
//	comment: 非空时打 Info 日志；空则 Trace（旧模块大量无注释注册）
func RegisterBizAPI(method, path string, needLogin bool, comment string, h http.HandlerFunc) {
	if h == nil {
		return
	}
	fullPath := path
	if fullPath == "" || fullPath[0] != '/' {
		fullPath = basePath + "/" + fullPath
	}
	if allowPaths != nil && !slices.Contains(allowPaths, fullPath) {
		log.Trace().Str("path", fullPath).Msg("[api] ignore path not in allow_paths")
		return
	}

	pattern := fullPath
	method = strings.ToUpper(strings.TrimSpace(method))
	if method != "" {
		pattern = method + " " + fullPath
	}

	handler := h
	if needLogin {
		handler = requireLogin(h)
	}

	if comment != "" {
		log.Info().
			Str("pattern", pattern).
			Bool("need_login", needLogin).
			Str("comment", comment).
			Msg("[api] register")
	} else {
		log.Trace().Str("pattern", pattern).Msg("[api] register")
	}
	http.HandleFunc(pattern, handler)
}

func requireLogin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := passporthttp.GetSessionUser(r)
		if u.UID <= 0 {
			fferrors.RespondBizError(w, fferrors.ErrUnauthorized)
			return
		}
		next(w, r)
	}
}

// ---------------------------------------------------------------------------
// AI 业务 API（模型仓库 / 算法能力 / 通道绑定 / 工作流）：契约注入与路由挂载。
// 非多模型调度运行时。实现在 internal/aibiz；handler 见 models_api / algorithms_api /
// workflows_api / model_upload。
// ---------------------------------------------------------------------------

// AIBizBackend AI 业务 API 能力，由 internal/aibiz 注入实现。
// api 层只做 HTTP 编解码与路由，不直接依赖 aibiz 包，避免循环引用。
type AIBizBackend interface {
	// --- 模型仓库 ---
	ListModels() (items []ModelItem, total int, err error)
	RegisterModel(in ModelRegisterInput) (ModelItem, error) // 仅元数据
	UploadModel(in ModelRegisterInput, modelFileName string, modelReader io.Reader, labelsFileName string, labelsReader io.Reader) (ModelItem, error)
	DownloadModel(id string) (ModelItem, error)    // 拉取/标记本机可用
	RemoveLocalModel(id string) (ModelItem, error) // 删本机文件，保留登记
	CheckModelHealth(id string) (ModelHealthResult, error)
	ScaleModel(id string, scale float64) (ModelItem, error)
	DeleteModel(id string) error // 元数据 + 本地文件
	ListModelUpdates() ([]ModelUpdateRecord, error)

	// --- 算法能力目录 ---
	ListAlgorithms(q, category, status string) (AlgorithmPage, error)
	CreateAlgorithm(in AlgorithmInput) (AlgorithmCapability, error)
	UpdateAlgorithm(id string, in AlgorithmInput) (AlgorithmCapability, error)
	UpdateAlgorithmStatus(id, status string) (AlgorithmCapability, error)
	DeleteAlgorithm(id string) error
	InferTest(id, imageName string, conf float64, imageJPEG []byte) (map[string]any, error) // 样张试跑（图片/视频）

	// --- 通道算法绑定（设备 AI）---
	ListCameraAlgorithms(cameraID, channelID string) ([]CameraAlgorithm, error)
	BindCameraAlgorithm(cameraID, channelID, algorithmID string) (CameraAlgorithm, error)
	UnbindCameraAlgorithm(cameraID, channelID, bindID string) error
	SaveCameraAlgorithm(cameraID, channelID, bindID string, patch CameraAlgorithmPatch) (CameraAlgorithm, error)
	CameraAlgorithmAction(cameraID, channelID, bindID, action string) (CameraAlgorithm, error) // start/stop 等

	// --- 工作流编排 ---
	ListWorkflows() ([]WorkflowSummary, error)
	GetWorkflow(id string) (WorkflowDetail, error)
	CreateWorkflow(in WorkflowCreateInput) (WorkflowSummary, error)
	UpdateWorkflow(id string, name, desc *string, nodes []WorkflowNode, edges []WorkflowEdge, hasNodes, hasEdges bool) (WorkflowDetail, error)
	DeleteWorkflow(id string) error
	StartWorkflow(id string) (WorkflowSummary, error)
	StopWorkflow(id string) (WorkflowSummary, error)
	DebugWorkflow(id string, in WorkflowDebugInput) (runID string, events []DebugEvent, err error)
	ListNodeSpecs() []NodeSpec // 编辑器可用节点类型
	LatestWorkflowToast() (item map[string]any, seq int64)

	// --- 告警中心 ---
	ListAlarms(q AlarmQuery) (AlarmPage, error)
	UpdateAlarmStatus(id, status string) (AlarmRecord, error)
	LinkAlarmWorkflow(id string, workflowID *string, workflowName string) (AlarmRecord, error)
	ListLinkageRules() ([]AlarmLinkageRule, error)
	CreateLinkageRule(in AlarmLinkageRuleInput) (AlarmLinkageRule, error)
	UpdateLinkageRule(id string, in AlarmLinkageRuleInput) (AlarmLinkageRule, error)
	DeleteLinkageRule(id string) error
	ApplyLinkageRules(onlyUnlinked bool) (ApplyLinkageResult, error)
	AlarmEvidencePath(id string) (relPath string, err error)

	// --- 事件核查（MVP：ai_alarms 投影为 alert）---
	ListReviewEvents(q ReviewQuery) (ReviewEventPage, error)
	ReviewKindCounts(q ReviewQuery) (ReviewKindCounts, error)
	GetReviewEvent(id string) (ReviewEvent, error)
	ListReviewContext(id string, windowMinutes, page, pageSize int) (ReviewEventPage, error)
	MarkReviewEvents(ids []string, status string) (int, error)
	PromoteReviewDetection(id string) (ReviewEvent, error)
}

var aiBizBackend AIBizBackend

// ProvideAIBiz 注入 AI 业务 API 后端；须在 MountAIBizAPIs 之前由 aibiz.Init 调用。
func ProvideAIBiz(b AIBizBackend) {
	aiBizBackend = b
}

// MountAIBizAPIs 注册模型 / 算法 / 通道绑定 / 工作流路由（显式 NeedLogin，不做 Casbin）。
// 须在 ProvideAIBiz 之后由 main 调用；backend 未注入时跳过挂载。
func MountAIBizAPIs() {
	if aiBizBackend == nil {
		log.Debug().Msg("[aibiz] skip mount http: backend nil")
		return
	}

	const login = true

	// --- 模型仓库 ---
	RegisterBizAPI("GET", "api/v1/models", login, "模型列表", handleModelsList)
	RegisterBizAPI("POST", "api/v1/models/register", login, "登记模型元数据", handleModelRegister)
	RegisterBizAPI("POST", "api/v1/models/upload", login, "上传并登记模型", handleModelUpload)
	RegisterBizAPI("GET", "api/v1/models/updates", login, "模型更新记录", handleModelUpdates)
	RegisterBizAPI("DELETE", "api/v1/models/{id}", login, "删除模型", handleModelDelete)
	RegisterBizAPI("POST", "api/v1/models/{id}/download", login, "标记/下载模型到本地", handleModelDownload)
	RegisterBizAPI("DELETE", "api/v1/models/{id}/local", login, "移除本地模型文件", handleModelRemoveLocal)
	RegisterBizAPI("POST", "api/v1/models/{id}/health", login, "模型健康检查", handleModelHealth)
	RegisterBizAPI("POST", "api/v1/models/{id}/scale", login, "调整模型 scale", handleModelScale)

	// --- 算法接入 ---
	RegisterBizAPI("GET", "api/v1/algorithms", login, "算法列表", handleAlgorithmsList)
	RegisterBizAPI("POST", "api/v1/algorithms", login, "创建算法", handleAlgorithmCreate)
	RegisterBizAPI("PUT", "api/v1/algorithms/{id}", login, "更新算法", handleAlgorithmUpdate)
	RegisterBizAPI("PATCH", "api/v1/algorithms/{id}", login, "更新算法状态", handleAlgorithmPatchStatus)
	RegisterBizAPI("DELETE", "api/v1/algorithms/{id}", login, "删除算法", handleAlgorithmDelete)
	RegisterBizAPI("POST", "api/v1/algorithms/{id}/infer-test", login, "算法推理试跑（图片/视频）", handleAlgorithmInferTest)

	// --- 设备 AI：通道算法绑定 ---
	RegisterBizAPI("GET", "api/v1/cameras/{id}/channels/{cid}/algorithms", login, "通道算法绑定列表", handleCameraAlgorithmsList)
	RegisterBizAPI("POST", "api/v1/cameras/{id}/channels/{cid}/algorithms", login, "绑定算法到通道", handleCameraAlgorithmBind)
	RegisterBizAPI("PUT", "api/v1/cameras/{id}/channels/{cid}/algorithms/{bid}", login, "保存通道算法配置", handleCameraAlgorithmSave)
	RegisterBizAPI("DELETE", "api/v1/cameras/{id}/channels/{cid}/algorithms/{bid}", login, "解绑通道算法", handleCameraAlgorithmUnbind)
	RegisterBizAPI("POST", "api/v1/cameras/{id}/channels/{cid}/algorithms/{bid}/{action}", login, "通道算法启停", handleCameraAlgorithmAction)

	// --- 工作流 ---
	RegisterBizAPI("GET", "api/v1/workflows", login, "工作流列表", handleWorkflowsList)
	RegisterBizAPI("POST", "api/v1/workflows", login, "创建工作流", handleWorkflowCreate)
	RegisterBizAPI("GET", "api/v1/workflows/node-specs", login, "工作流节点规格", handleWorkflowNodeSpecs)
	RegisterBizAPI("GET", "api/v1/workflows/notifications/latest", login, "最新工作流页面通知", handleWorkflowNotifyLatest)
	RegisterBizAPI("GET", "api/v1/workflows/debug/stream", login, "调试事件 SSE", handleWorkflowDebugStream)
	RegisterBizAPI("GET", "api/v1/workflows/{id}", login, "工作流详情", handleWorkflowGet)
	RegisterBizAPI("PUT", "api/v1/workflows/{id}", login, "更新工作流", handleWorkflowUpdate)
	RegisterBizAPI("DELETE", "api/v1/workflows/{id}", login, "删除工作流", handleWorkflowDelete)
	RegisterBizAPI("POST", "api/v1/workflows/{id}/start", login, "启动工作流", handleWorkflowStart)
	RegisterBizAPI("POST", "api/v1/workflows/{id}/stop", login, "停止工作流", handleWorkflowStop)
	RegisterBizAPI("POST", "api/v1/workflows/{id}/debug", login, "调试执行工作流", handleWorkflowDebug)

	// --- 告警中心 ---
	RegisterBizAPI("GET", "api/v1/alarms", login, "告警列表", handleAlarmsList)
	RegisterBizAPI("GET", "api/v1/alarms/{id}/evidence", login, "告警抓拍证据", handleAlarmEvidence)
	RegisterBizAPI("PATCH", "api/v1/alarms/{id}", login, "更新告警状态", handleAlarmPatchStatus)
	RegisterBizAPI("PATCH", "api/v1/alarms/{id}/workflow", login, "告警手工联动工作流", handleAlarmPatchWorkflow)
	RegisterBizAPI("GET", "api/v1/alarms/linkage-rules", login, "联动规则列表", handleLinkageRulesList)
	RegisterBizAPI("POST", "api/v1/alarms/linkage-rules", login, "创建联动规则", handleLinkageRuleCreate)
	RegisterBizAPI("PUT", "api/v1/alarms/linkage-rules/{id}", login, "更新联动规则", handleLinkageRuleUpdate)
	RegisterBizAPI("DELETE", "api/v1/alarms/linkage-rules/{id}", login, "删除联动规则", handleLinkageRuleDelete)
	RegisterBizAPI("POST", "api/v1/alarms/linkage-rules/apply", login, "应用联动规则", handleLinkageRulesApply)

	// --- 事件核查 ---
	RegisterBizAPI("GET", "api/v1/review/events", login, "核查事件列表", handleReviewEventsList)
	RegisterBizAPI("GET", "api/v1/review/events/counts", login, "核查事件计数", handleReviewKindCounts)
	RegisterBizAPI("POST", "api/v1/review/events/mark", login, "批量标记核查状态", handleReviewEventsMark)
	RegisterBizAPI("GET", "api/v1/review/events/{id}", login, "核查事件详情", handleReviewEventGet)
	RegisterBizAPI("GET", "api/v1/review/events/{id}/context", login, "核查事件上下文", handleReviewEventContext)
	RegisterBizAPI("POST", "api/v1/review/events/{id}/promote", login, "检测提升为警报", handleReviewEventPromote)

	log.Info().Msg("[aibiz] http routes registered")
}
