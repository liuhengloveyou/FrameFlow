package aibiz

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/liuhengloveyou/FrameFlow/internal/api"
	"github.com/liuhengloveyou/FrameFlow/pkg/database"
	"gorm.io/gorm"
)

// Store 持久化。
type Store struct {
	db *database.DB
}

func NewStore(db *database.DB) (*Store, error) {
	if db == nil {
		return nil, fmt.Errorf("aibiz: database nil")
	}
	if err := db.AutoMigrate(
		&ModelRow{},
		&ModelUpdateRow{},
		&AlgorithmRow{},
		&BindingRow{},
		&WorkflowRow{},
		&AlarmRow{},
		&LinkageRuleRow{},
	); err != nil {
		return nil, fmt.Errorf("aibiz: automigrate: %w", err)
	}
	return &Store{db: db}, nil
}

// ——— models ———

type ModelRow struct {
	ID             string         `gorm:"column:id;primaryKey;size:64"`
	Name           string         `gorm:"column:name;size:256;not null"`
	Description    string         `gorm:"column:description;type:text;not null;default:''"`
	TagsJSON       string         `gorm:"column:tags_json;type:text;not null;default:'[]'"`
	FormatsJSON    string         `gorm:"column:formats_json;type:text;not null;default:'[]'"`
	LicenseStatus  string         `gorm:"column:license_status;size:32;not null;default:'authorized'"`
	ModelStatus    string         `gorm:"column:model_status;size:32;not null;default:'ready'"`
	Version        string         `gorm:"column:version;size:64;not null;default:'1.0.0'"`
	PublishedAt    string         `gorm:"column:published_at;size:64;not null;default:''"`
	LocalVersion   *string        `gorm:"column:local_version;size:64"`
	LocalUpdatedAt *string        `gorm:"column:local_updated_at;size:64"`
	RuntimeScale   float64        `gorm:"column:runtime_scale;not null;default:1"`
	FileName       string         `gorm:"column:file_name;size:512;not null;default:''"`
	FileSizeBytes  int64          `gorm:"column:file_size_bytes;not null;default:0"`
	LocalPath      string         `gorm:"column:local_path;size:1024;not null;default:''"`
	LabelsPath     string         `gorm:"column:labels_path;size:1024;not null;default:''"`
	CreatedAt      time.Time      `json:"-"`
	UpdatedAt      time.Time      `json:"-"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

func (ModelRow) TableName() string { return "ai_models" }

type ModelUpdateRow struct {
	ID             string `gorm:"column:id;primaryKey;size:64"`
	ModelName      string `gorm:"column:model_name;size:256;not null"`
	CurrentVersion string `gorm:"column:current_version;size:64;not null;default:''"`
	TargetVersion  string `gorm:"column:target_version;size:64;not null;default:''"`
	OperatedAt     string `gorm:"column:operated_at;size:64;not null"`
	Status         string `gorm:"column:status;size:32;not null;default:'success'"`
	Message        string `gorm:"column:message;type:text;not null;default:''"`
}

func (ModelUpdateRow) TableName() string { return "ai_model_updates" }

type AlgorithmRow struct {
	ID               string         `gorm:"column:id;primaryKey;size:64"`
	Name             string         `gorm:"column:name;size:256;not null"`
	Description      string         `gorm:"column:description;type:text;not null;default:''"`
	Category         string         `gorm:"column:category;size:128;not null;default:''"`
	ModelID          string         `gorm:"column:model_id;size:64;not null;index"`
	Status           string         `gorm:"column:status;size:32;not null;default:'draft';index"`
	BoundCameraCount int            `gorm:"column:bound_camera_count;not null;default:0"`
	TagsJSON         string         `gorm:"column:tags_json;type:text;not null;default:'[]'"`
	ParamsJSON       string         `gorm:"column:params_json;type:text;not null;default:'{}'"`
	CreatedAt        time.Time      `json:"-"`
	UpdatedAt        time.Time      `json:"-"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}

func (AlgorithmRow) TableName() string { return "ai_algorithms" }

// BindingRow 摄像头通道 ↔ 算法绑定。
type BindingRow struct {
	ID          string         `gorm:"column:id;primaryKey;size:64"`
	CameraID    string         `gorm:"column:camera_id;size:64;not null;index"`
	ChannelID   string         `gorm:"column:channel_id;size:128;not null;index"`
	AlgorithmID string         `gorm:"column:algorithm_id;size:64;not null;index"`
	Status      string         `gorm:"column:status;size:32;not null;default:'uninitialized'"`
	RulesJSON   string         `gorm:"column:rules_json;type:text;not null;default:'{}'"`
	RegionsJSON string         `gorm:"column:regions_json;type:text;not null;default:'[]'"`
	CreatedAt   time.Time      `json:"-"`
	UpdatedAt   time.Time      `json:"-"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (BindingRow) TableName() string { return "ai_camera_bindings" }

type WorkflowRow struct {
	ID           string         `gorm:"column:id;primaryKey;size:64"`
	Name         string         `gorm:"column:name;size:256;not null"`
	Description  string         `gorm:"column:description;type:text;not null;default:''"`
	Status       string         `gorm:"column:status;size:32;not null;default:'stopped';index"`
	NodeCount    int            `gorm:"column:node_count;not null;default:0"`
	EdgeCount    int            `gorm:"column:edge_count;not null;default:0"`
	CallCount    int            `gorm:"column:call_count;not null;default:0"`
	SuccessCount int            `gorm:"column:success_count;not null;default:0"`
	SuccessRate  float64        `gorm:"column:success_rate;not null;default:0"`
	LastRunAt    *string        `gorm:"column:last_run_at;size:64"`
	NodesJSON    string         `gorm:"column:nodes_json;type:text;not null;default:'[]'"`
	EdgesJSON    string         `gorm:"column:edges_json;type:text;not null;default:'[]'"`
	CreatedAt    time.Time      `json:"-"`
	UpdatedAt    time.Time      `json:"-"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

func (WorkflowRow) TableName() string { return "ai_workflows" }

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func parseStringSlice(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return []string{}
	}
	if out == nil {
		return []string{}
	}
	return out
}

func defaultAlgorithmParams() api.AlgorithmParams {
	// 检测阈值已迁到绑定 rules；算法 params 默认空。
	return api.AlgorithmParams{}
}

func normalizeAlgorithmParams(p *api.AlgorithmParams) api.AlgorithmParams {
	if p == nil {
		return api.AlgorithmParams{}
	}
	return *p
}

func parseAlgorithmParams(s string) api.AlgorithmParams {
	out := defaultAlgorithmParams()
	s = strings.TrimSpace(s)
	if s == "" || s == "{}" {
		return out
	}
	var p api.AlgorithmParams
	if err := json.Unmarshal([]byte(s), &p); err != nil {
		return out
	}
	return normalizeAlgorithmParams(&p)
}

func newID(prefix string) string {
	return prefix + uuid.NewString()[:8]
}

func fileExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func resolveModelPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		return p
	}
	// 相对仓库工作目录
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

func (s *Store) countModels() (int64, error) {
	var n int64
	err := s.db.Model(&ModelRow{}).Count(&n).Error
	return n, err
}

func (s *Store) insertModel(row *ModelRow) error {
	return s.db.Create(row).Error
}

func (s *Store) listModels() ([]ModelRow, error) {
	var rows []ModelRow
	err := s.db.Order("updated_at DESC").Find(&rows).Error
	return rows, err
}

func (s *Store) getModel(id string) (ModelRow, error) {
	var row ModelRow
	err := s.db.Where("id = ?", id).First(&row).Error
	return row, err
}

func (s *Store) saveModel(row *ModelRow) error {
	return s.db.Save(row).Error
}

func (s *Store) deleteModel(id string) error {
	res := s.db.Where("id = ?", id).Delete(&ModelRow{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Store) insertUpdate(row *ModelUpdateRow) error {
	return s.db.Create(row).Error
}

func (s *Store) listUpdates(limit int) ([]ModelUpdateRow, error) {
	var rows []ModelUpdateRow
	q := s.db.Order("operated_at DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Find(&rows).Error
	return rows, err
}

func (s *Store) countAlgorithms() (int64, error) {
	var n int64
	err := s.db.Model(&AlgorithmRow{}).Count(&n).Error
	return n, err
}

func (s *Store) insertAlgorithm(row *AlgorithmRow) error {
	return s.db.Create(row).Error
}

func (s *Store) listAlgorithms() ([]AlgorithmRow, error) {
	var rows []AlgorithmRow
	err := s.db.Order("updated_at DESC").Find(&rows).Error
	return rows, err
}

func (s *Store) getAlgorithm(id string) (AlgorithmRow, error) {
	var row AlgorithmRow
	err := s.db.Where("id = ?", id).First(&row).Error
	return row, err
}

func (s *Store) saveAlgorithm(row *AlgorithmRow) error {
	return s.db.Save(row).Error
}

func (s *Store) deleteAlgorithm(id string) error {
	return s.db.Where("id = ?", id).Delete(&AlgorithmRow{}).Error
}

func (s *Store) countAlgorithmsByModel(modelID string) (int64, error) {
	var n int64
	err := s.db.Model(&AlgorithmRow{}).Where("model_id = ?", modelID).Count(&n).Error
	return n, err
}

func (s *Store) listBindings(cameraID, channelID string) ([]BindingRow, error) {
	var rows []BindingRow
	err := s.db.Where("camera_id = ? AND channel_id = ?", cameraID, channelID).
		Order("created_at ASC").Find(&rows).Error
	return rows, err
}

func (s *Store) getBinding(id string) (BindingRow, error) {
	var row BindingRow
	err := s.db.Where("id = ?", id).First(&row).Error
	return row, err
}

func (s *Store) insertBinding(row *BindingRow) error {
	return s.db.Create(row).Error
}

func (s *Store) saveBinding(row *BindingRow) error {
	return s.db.Save(row).Error
}

func (s *Store) deleteBinding(id string) error {
	return s.db.Where("id = ?", id).Delete(&BindingRow{}).Error
}

func (s *Store) countBindingsByAlgorithm(algorithmID string) (int64, error) {
	var n int64
	err := s.db.Model(&BindingRow{}).Where("algorithm_id = ?", algorithmID).Count(&n).Error
	return n, err
}

func (s *Store) countDistinctCamerasByAlgorithm(algorithmID string) (int64, error) {
	var ids []string
	err := s.db.Model(&BindingRow{}).
		Where("algorithm_id = ?", algorithmID).
		Distinct().
		Pluck("camera_id", &ids).Error
	return int64(len(ids)), err
}

func (s *Store) countWorkflows() (int64, error) {
	var n int64
	err := s.db.Model(&WorkflowRow{}).Count(&n).Error
	return n, err
}

func (s *Store) insertWorkflow(row *WorkflowRow) error {
	return s.db.Create(row).Error
}

func (s *Store) listWorkflows() ([]WorkflowRow, error) {
	var rows []WorkflowRow
	err := s.db.Order("updated_at DESC").Find(&rows).Error
	return rows, err
}

func (s *Store) getWorkflow(id string) (WorkflowRow, error) {
	var row WorkflowRow
	err := s.db.Where("id = ?", id).First(&row).Error
	return row, err
}

func (s *Store) saveWorkflow(row *WorkflowRow) error {
	return s.db.Save(row).Error
}

func (s *Store) deleteWorkflow(id string) error {
	res := s.db.Where("id = ?", id).Delete(&WorkflowRow{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
