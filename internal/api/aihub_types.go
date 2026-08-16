package api

// DTO 与前端 types/api.ts 对齐（JSON camelCase）。

type ModelItem struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Tags           []string `json:"tags"`
	Formats        []string `json:"formats"`
	LicenseStatus  string   `json:"licenseStatus"`
	ModelStatus    string   `json:"modelStatus"`
	Version        string   `json:"version"`
	PublishedAt    string   `json:"publishedAt"`
	LocalVersion   *string  `json:"localVersion"`
	LocalUpdatedAt *string  `json:"localUpdatedAt"`
	RuntimeScale   float64  `json:"runtimeScale,omitempty"`
	FileSizeBytes  int64    `json:"fileSizeBytes,omitempty"`
	FileName       string   `json:"fileName,omitempty"`
	LocalPath      string   `json:"localPath,omitempty"`
	LabelsPath     string   `json:"labelsPath,omitempty"`
}

type ModelRegisterInput struct {
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Version       string   `json:"version"`
	Formats       []string `json:"formats"`
	Tags          []string `json:"tags"`
	FileName      string   `json:"fileName"`
	FileSizeBytes int64    `json:"fileSizeBytes"`
	LocalPath     string   `json:"localPath"`
	LabelsPath    string   `json:"labelsPath"`
}

type ModelUpdateRecord struct {
	ID             string `json:"id"`
	ModelName      string `json:"modelName"`
	CurrentVersion string `json:"currentVersion"`
	TargetVersion  string `json:"targetVersion"`
	OperatedAt     string `json:"operatedAt"`
	Status         string `json:"status"`
	Message        string `json:"message"`
}

type ModelHealthResult struct {
	OK        bool   `json:"ok"`
	Message   string `json:"message"`
	LatencyMs int    `json:"latencyMs"`
}

type AlgorithmParams struct {
	// 检测阈值（conf/iou/maxDet）已迁到摄像头绑定 rules，不再作为算法级配置。
	// 字段保留仅为兼容旧 API / 库内 JSON，新写入可为空对象。
	ConfThreshold float64 `json:"confThreshold,omitempty"`
	IouThreshold  float64 `json:"iouThreshold,omitempty"`
	MaxDetections int     `json:"maxDetections,omitempty"`
}

type AlgorithmCapability struct {
	ID               string          `json:"id"`
	Name             string          `json:"name"`
	Description      string          `json:"description"`
	Category         string          `json:"category"`
	ModelID          string          `json:"modelId"`
	ModelName        string          `json:"modelName"`
	Status           string          `json:"status"`
	BoundCameraCount int             `json:"boundCameraCount"`
	Tags             []string        `json:"tags"`
	Params           AlgorithmParams `json:"params"`
	UpdatedAt        string          `json:"updatedAt"`
}

type AlgorithmInput struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Category    string           `json:"category"`
	ModelID     string           `json:"modelId"`
	Tags        []string         `json:"tags"`
	Status      string           `json:"status"`
	Params      *AlgorithmParams `json:"params,omitempty"`
}

type AlgorithmPage struct {
	Items    []AlgorithmCapability `json:"items"`
	Total    int                   `json:"total"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"pageSize"`
}

// AlgorithmRuleField 设备 AI 规则表单字段（与前端 AlgorithmRuleField 对齐）。
type AlgorithmRuleField struct {
	Key         string              `json:"key"`
	Label       string              `json:"label"`
	Type        string              `json:"type"`            // number|boolean|select|string
	Group       string              `json:"group,omitempty"` // 表单分组，如 检测 / 跟踪 / 告警
	Description string              `json:"description,omitempty"`
	Min         *float64            `json:"min,omitempty"`
	Max         *float64            `json:"max,omitempty"`
	Step        *float64            `json:"step,omitempty"`
	Required    bool                `json:"required,omitempty"`
	Options     []map[string]string `json:"options,omitempty"`
}

// CameraAlgorithm 摄像头通道上的算法绑定。
type CameraAlgorithm struct {
	ID          string               `json:"id"`
	AlgorithmID string               `json:"algorithmId"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Status      string               `json:"status"` // uninitialized|running|stopped|error
	ModelID     string               `json:"modelId,omitempty"`
	ChannelID   string               `json:"channelId,omitempty"`
	CameraID    string               `json:"cameraId,omitempty"`
	Rules       map[string]any       `json:"rules"`
	RuleSchema  []AlgorithmRuleField `json:"ruleSchema"`
	Regions     [][]map[string]any   `json:"regions"` // [{x,y},...]
}

type CameraAlgorithmBindInput struct {
	AlgorithmID string `json:"algorithmId"`
}

type CameraAlgorithmPatch struct {
	Rules   map[string]any     `json:"rules"`
	Regions [][]map[string]any `json:"regions"`
}

type WorkflowSummary struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	Status       string  `json:"status"`
	NodeCount    int     `json:"nodeCount"`
	EdgeCount    int     `json:"edgeCount"`
	CallCount    int     `json:"callCount"`
	SuccessCount int     `json:"successCount"`
	SuccessRate  float64 `json:"successRate"`
	LastRunAt    *string `json:"lastRunAt"`
	UpdatedAt    string  `json:"updatedAt"`
}

type WorkflowNode struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Position map[string]any `json:"position"`
	Data     map[string]any `json:"data"`
}

type WorkflowEdge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
}

type WorkflowDetail struct {
	WorkflowSummary
	Nodes []WorkflowNode `json:"nodes"`
	Edges []WorkflowEdge `json:"edges"`
}

type WorkflowCreateInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type WorkflowDebugInput struct {
	InputMode   string            `json:"inputMode"` // latest_alarm | custom
	CustomInput map[string]string `json:"customInput"`
	ReturnMode  string            `json:"returnMode"` // sync | stream
}

type DebugEvent struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	NodeID  string `json:"nodeId,omitempty"`
	Message string `json:"message"`
	At      string `json:"at"`
	Payload any    `json:"payload,omitempty"`
}

type WorkflowPatch struct {
	Name        *string        `json:"name"`
	Description *string        `json:"description"`
	Nodes       []WorkflowNode `json:"nodes"`
	Edges       []WorkflowEdge `json:"edges"`
	HasNodes    bool           `json:"-"`
	HasEdges    bool           `json:"-"`
}

type NodeFieldSpec struct {
	Key         string              `json:"key"`
	Label       string              `json:"label"`
	Type        string              `json:"type"`
	Required    bool                `json:"required,omitempty"`
	Description string              `json:"description,omitempty"`
	Options     []map[string]string `json:"options,omitempty"`
	Min         *float64            `json:"min,omitempty"`
	Max         *float64            `json:"max,omitempty"`
	Step        *float64            `json:"step,omitempty"`
}

type NodePortSpec struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Type  string `json:"type"`
}

type NodeSpec struct {
	Type        string          `json:"type"`
	Label       string          `json:"label"`
	Category    string          `json:"category"`
	Description string          `json:"description"`
	IsTrigger   bool            `json:"isTrigger,omitempty"`
	Inputs      []NodeFieldSpec `json:"inputs"`
	Outputs     []NodePortSpec  `json:"outputs"`
}

// ——— 告警中心 ———

type AlarmRecord struct {
	ID              uint     `json:"id"`
	Title           string   `json:"title"`
	Level           string   `json:"level"`
	Status          string   `json:"status"`
	CameraID        string   `json:"cameraId"`
	CameraName      string   `json:"cameraName"`
	CameraGroup     string   `json:"cameraGroup,omitempty"`
	AlgorithmName   string   `json:"algorithmName"`
	WorkflowID      string   `json:"workflowId,omitempty"`
	WorkflowName    string   `json:"workflowName,omitempty"`
	LinkageRuleID   uint     `json:"linkageRuleId,omitempty"`
	LinkageRuleName string   `json:"linkageRuleName,omitempty"`
	Message         string   `json:"message"`
	OccurredAt      string   `json:"occurredAt"`
	EvidenceFileIds []string `json:"evidenceFileIds,omitempty"`
	EvidenceNote    string   `json:"evidenceNote,omitempty"`
}

type AlarmPage struct {
	Items    []AlarmRecord `json:"items"`
	Total    int           `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"pageSize"`
}

type AlarmQuery struct {
	Page      int
	PageSize  int
	Level     string
	Status    string
	Q         string
	CameraIDs []string // 空切片=无匹配；nil=不限
	From      string   // ISO 下界（含）
	To        string   // ISO 上界（含）
	OrderAsc  bool     // true=occurred_at ASC；默认 DESC
}

type AlarmStatusPatch struct {
	Status string `json:"status"`
}

type AlarmWorkflowPatch struct {
	WorkflowID *string `json:"workflowId"` // null 清除；有值时需带名称（可选）
	Name       string  `json:"name,omitempty"`
}

type AlarmLinkageRule struct {
	ID            uint     `json:"id"`
	Name          string   `json:"name"`
	Enabled       bool     `json:"enabled"`
	Levels        []string `json:"levels"`
	AlgorithmName string   `json:"algorithmName,omitempty"`
	CameraGroup   string   `json:"cameraGroup,omitempty"`
	WorkflowID    string   `json:"workflowId"`
	WorkflowName  string   `json:"workflowName"`
	Priority      int      `json:"priority"`
	UpdatedAt     string   `json:"updatedAt"`
}

type AlarmLinkageRuleInput struct {
	Name          string   `json:"name"`
	Enabled       bool     `json:"enabled"`
	Levels        []string `json:"levels"`
	AlgorithmName string   `json:"algorithmName"`
	CameraGroup   string   `json:"cameraGroup"`
	WorkflowID    string   `json:"workflowId"`
	WorkflowName  string   `json:"workflowName"`
	Priority      int      `json:"priority"`
}

type ApplyLinkageInput struct {
	OnlyUnlinked *bool `json:"onlyUnlinked"`
}

type ApplyLinkageResult struct {
	Updated int `json:"updated"`
}

// ——— 事件核查（MVP：由 ai_alarms 投影）———

type ReviewBox struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	W     float64 `json:"w"`
	H     float64 `json:"h"`
	Label string  `json:"label,omitempty"`
}

type ReviewEvent struct {
	ID              string      `json:"id"`
	Kind            string      `json:"kind"`
	Status          string      `json:"status"`
	Title           string      `json:"title"`
	CameraID        string      `json:"cameraId"`
	CameraName      string      `json:"cameraName"`
	Label           string      `json:"label"`
	ObjectClass     string      `json:"objectClass,omitempty"`
	Score           float64     `json:"score,omitempty"`
	OccurredAt      string      `json:"occurredAt"`
	ThumbURL        string      `json:"thumbUrl,omitempty"`
	EvidenceFileIds []string    `json:"evidenceFileIds,omitempty"`
	Message         string      `json:"message,omitempty"`
	ClipSec         int         `json:"clipSec,omitempty"`
	Boxes           []ReviewBox `json:"boxes,omitempty"`
	PromotedAlertID string      `json:"promotedAlertId,omitempty"`
}

type ReviewEventPage struct {
	Items    []ReviewEvent `json:"items"`
	Total    int           `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"pageSize"`
}

type ReviewQuery struct {
	Page      int
	PageSize  int
	Kind      string
	Status    string
	Q         string
	CameraIDs []string
	From      string
	To        string
}

type ReviewKindCounts struct {
	Alert      int `json:"alert"`
	Detection  int `json:"detection"`
	Motion     int `json:"motion"`
	Unreviewed int `json:"unreviewed"`
}

type ReviewMarkInput struct {
	IDs    []string `json:"ids"`
	Status string   `json:"status"`
}
