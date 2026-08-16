package aibiz

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/liuhengloveyou/FrameFlow/internal/api"
	"github.com/liuhengloveyou/FrameFlow/pkg/utils"
	"gorm.io/gorm"
)

type AlarmRow struct {
	gorm.Model
	Title           string `gorm:"column:title;size:512;not null;default:''"`
	Level           string `gorm:"column:level;size:32;not null;default:'warn';index"`
	Status          string `gorm:"column:status;size:32;not null;default:'open';index"`
	CameraID        string `gorm:"column:camera_id;size:64;not null;index"`
	CameraName      string `gorm:"column:camera_name;size:256;not null;default:''"`
	CameraGroup     string `gorm:"column:camera_group;size:128;not null;default:''"`
	ChannelID       string `gorm:"column:channel_id;size:128;not null;default:''"`
	AlgorithmName   string `gorm:"column:algorithm_name;size:256;not null;default:'';index"`
	BindID          string `gorm:"column:bind_id;size:64;not null;default:'';index"`
	WorkflowID      string `gorm:"column:workflow_id;size:64;not null;default:''"`
	WorkflowName    string `gorm:"column:workflow_name;size:256;not null;default:''"`
	LinkageRuleID   uint   `gorm:"column:linkage_rule_id;not null;default:0;index"`
	LinkageRuleName string `gorm:"column:linkage_rule_name;size:256;not null;default:''"`
	Message         string `gorm:"column:message;type:text;not null;default:''"`
	OccurredAt      string `gorm:"column:occurred_at;size:64;not null;index"`
	EvidenceIDsJSON string `gorm:"column:evidence_ids_json;type:text;not null;default:'[]'"`
	EvidenceNote    string `gorm:"column:evidence_note;type:text;not null;default:''"`
	BoxesJSON       string `gorm:"column:boxes_json;type:text;not null;default:'[]'"`
}

func (AlarmRow) TableName() string { return "ai_alarms" }

type LinkageRuleRow struct {
	gorm.Model
	Name          string `gorm:"column:name;size:256;not null"`
	Enabled       bool   `gorm:"column:enabled;not null;default:true;index"`
	LevelsJSON    string `gorm:"column:levels_json;type:text;not null;default:'[]'"`
	AlgorithmName string `gorm:"column:algorithm_name;size:256;not null;default:''"`
	CameraGroup   string `gorm:"column:camera_group;size:128;not null;default:''"`
	WorkflowID    string `gorm:"column:workflow_id;size:64;not null"`
	WorkflowName  string `gorm:"column:workflow_name;size:256;not null;default:''"`
	Priority      int    `gorm:"column:priority;not null;default:100;index"`
}

func (LinkageRuleRow) TableName() string { return "ai_alarm_linkage_rules" }

func parseRowID(id string) (uint, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return 0, fmt.Errorf("id required")
	}
	n, err := strconv.ParseUint(id, 10, 64)
	if err != nil || n == 0 {
		return 0, fmt.Errorf("invalid id")
	}
	return uint(n), nil
}

func (s *Store) insertAlarm(row *AlarmRow) error {
	return s.db.Create(row).Error
}

func (s *Store) getAlarm(id uint) (AlarmRow, error) {
	var row AlarmRow
	err := s.db.First(&row, id).Error
	return row, err
}

func (s *Store) saveAlarm(row *AlarmRow) error {
	return s.db.Save(row).Error
}

func (s *Store) listAlarms(q api.AlarmQuery) ([]AlarmRow, int, error) {
	tx := s.db.Model(&AlarmRow{})
	if q.Level != "" {
		tx = tx.Where("level = ?", q.Level)
	}
	if q.Status != "" {
		tx = tx.Where("status = ?", q.Status)
	}
	if q.CameraIDs != nil {
		if len(q.CameraIDs) == 0 {
			return []AlarmRow{}, 0, nil
		}
		tx = tx.Where("camera_id IN ?", q.CameraIDs)
	}
	if from := strings.TrimSpace(q.From); from != "" {
		tx = tx.Where("occurred_at >= ?", from)
	}
	if to := strings.TrimSpace(q.To); to != "" {
		tx = tx.Where("occurred_at <= ?", to)
	}
	if qq := strings.TrimSpace(q.Q); qq != "" {
		like := "%" + qq + "%"
		tx = tx.Where(
			"title LIKE ? OR camera_name LIKE ? OR algorithm_name LIKE ? OR message LIKE ?",
			like, like, like, like,
		)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	pageSize := q.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 500 {
		pageSize = 500
	}
	order := "occurred_at DESC"
	if q.OrderAsc {
		order = "occurred_at ASC"
	}
	var rows []AlarmRow
	err := tx.Order(order).Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error
	return rows, int(total), err
}

func (s *Store) listAllOpenAlarms() ([]AlarmRow, error) {
	var rows []AlarmRow
	err := s.db.Where("status = ?", "open").Order("occurred_at DESC").Find(&rows).Error
	return rows, err
}

func (s *Store) listLinkageRules() ([]LinkageRuleRow, error) {
	var rows []LinkageRuleRow
	err := s.db.Order("priority ASC, name ASC").Find(&rows).Error
	return rows, err
}

func (s *Store) getLinkageRule(id uint) (LinkageRuleRow, error) {
	var row LinkageRuleRow
	err := s.db.First(&row, id).Error
	return row, err
}

func (s *Store) insertLinkageRule(row *LinkageRuleRow) error {
	return s.db.Create(row).Error
}

func (s *Store) saveLinkageRule(row *LinkageRuleRow) error {
	return s.db.Save(row).Error
}

func (s *Store) deleteLinkageRule(id uint) error {
	return s.db.Delete(&LinkageRuleRow{}, id).Error
}

func alarmEvidenceDir(day string) string {
	return filepath.Join("data", "alarm-evidence", day)
}

func writeAlarmEvidenceJPEG(alarmID uint, jpeg []byte) (relPath string, err error) {
	if len(jpeg) == 0 {
		return "", nil
	}
	day := time.Now().Format("20060102")
	dir := alarmEvidenceDir(day)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("%d.jpg", alarmID)
	abs := filepath.Join(dir, name)
	if err := os.WriteFile(abs, jpeg, 0o644); err != nil {
		return "", err
	}
	return filepath.ToSlash(filepath.Join(dir, name)), nil
}

func alarmToDTO(row AlarmRow) api.AlarmRecord {
	ids := parseStringSlice(row.EvidenceIDsJSON)
	rec := api.AlarmRecord{
		ID:            row.ID,
		Title:         row.Title,
		Level:         row.Level,
		Status:        row.Status,
		CameraID:      row.CameraID,
		CameraName:    row.CameraName,
		CameraGroup:   row.CameraGroup,
		AlgorithmName: row.AlgorithmName,
		Message:       row.Message,
		OccurredAt:    row.OccurredAt,
		EvidenceNote:  row.EvidenceNote,
	}
	if len(ids) > 0 {
		rec.EvidenceFileIds = ids
	}
	if row.WorkflowID != "" {
		rec.WorkflowID = row.WorkflowID
		rec.WorkflowName = row.WorkflowName
	}
	if row.LinkageRuleID != 0 {
		rec.LinkageRuleID = row.LinkageRuleID
		rec.LinkageRuleName = row.LinkageRuleName
	}
	return rec
}

func linkageToDTO(row LinkageRuleRow) api.AlarmLinkageRule {
	levels := parseStringSlice(row.LevelsJSON)
	if levels == nil {
		levels = []string{}
	}
	return api.AlarmLinkageRule{
		ID:            row.ID,
		Name:          row.Name,
		Enabled:       row.Enabled,
		Levels:        levels,
		AlgorithmName: row.AlgorithmName,
		CameraGroup:   row.CameraGroup,
		WorkflowID:    row.WorkflowID,
		WorkflowName:  row.WorkflowName,
		Priority:      row.Priority,
		UpdatedAt:     utils.FormatISO(row.UpdatedAt),
	}
}

func linkageFromInput(in api.AlarmLinkageRuleInput) LinkageRuleRow {
	levels := in.Levels
	if levels == nil {
		levels = []string{}
	}
	return LinkageRuleRow{
		Name:          strings.TrimSpace(in.Name),
		Enabled:       in.Enabled,
		LevelsJSON:    mustJSON(levels),
		AlgorithmName: strings.TrimSpace(in.AlgorithmName),
		CameraGroup:   strings.TrimSpace(in.CameraGroup),
		WorkflowID:    strings.TrimSpace(in.WorkflowID),
		WorkflowName:  strings.TrimSpace(in.WorkflowName),
		Priority:      in.Priority,
	}
}
