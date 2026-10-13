package aibiz

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/liuhengloveyou/FrameFlow/internal/api"
	"github.com/liuhengloveyou/FrameFlow/internal/app"
	"github.com/liuhengloveyou/FrameFlow/internal/vision"
	"github.com/liuhengloveyou/FrameFlow/pkg/utils"
)

const defaultAlarmCooldownSec = 30.0

var (
	alarmCooldownMu sync.Mutex
	alarmCooldownAt = map[string]time.Time{} // bindID -> last alarm time
)

func (s *Service) registerDetectionHook() {
	vision.SetDetectionHook(s.onVisionDetection)
}

func (s *Service) onVisionDetection(ev vision.DetectionEvent) {
	if !ev.Policy.Enabled || len(ev.Boxes) == 0 {
		return
	}
	cooldown := ev.Policy.CooldownSec
	if cooldown <= 0 {
		cooldown = defaultAlarmCooldownSec
	}
	key := ev.Policy.BindID
	if key == "" {
		key = ev.StreamKey
	}
	if !allowAlarmCooldown(key, time.Duration(cooldown*float64(time.Second))) {
		return
	}

	cameraID, channelID, _ := s.resolveCameraChannelFromStream(ev.StreamKey)
	camName, camGroup, resolvedCamID := s.resolveCameraMeta(cameraID)
	if resolvedCamID != "" {
		cameraID = resolvedCamID
	}
	algName := ev.Policy.AlgorithmName
	if algName == "" {
		algName = "算法"
	}
	level := normalizeAlarmLevel(ev.Policy.AlarmLevel)

	now := utils.NowISO()
	title := fmt.Sprintf("%s · 检出 %d 个目标", algName, len(ev.Boxes))
	msg := summarizeBoxes(ev.Boxes)

	row := AlarmRow{
		Title:         title,
		Level:         level,
		Status:        "open",
		CameraID:      cameraID,
		CameraName:    camName,
		CameraGroup:   camGroup,
		ChannelID:     channelID,
		AlgorithmName: algName,
		BindID:        ev.Policy.BindID,
		Message:       msg,
		OccurredAt:    now,
		BoxesJSON:     mustJSON(ev.Boxes),
	}

	rec := alarmToDTO(row)
	if rule := s.findMatchingLinkage(rec); rule != nil {
		row.LinkageRuleID = rule.ID
		row.LinkageRuleName = rule.Name
		row.WorkflowID = rule.WorkflowID
		row.WorkflowName = rule.WorkflowName
	}

	if err := s.store.insertAlarm(&row); err != nil {
		app.Logger.Warn().Err(err).Str("stream", ev.StreamKey).Msg("[alarms] insert failed")
		return
	}

	// 证据 JPEG 在「确定要落库」之后才编码：告警有冷却（默认 30s），
	// 检测热路径上逢框就编码 1080p JPEG（约 50ms/拍）纯属浪费。
	if path, err := writeAlarmEvidenceJPEG(row.ID, ev.EvidenceJPEG()); err == nil && path != "" {
		row.EvidenceIDsJSON = mustJSON([]string{path})
		row.EvidenceNote = "已保存检出抓拍"
		if err := s.store.saveAlarm(&row); err != nil {
			app.Logger.Warn().Err(err).Uint("alarm", row.ID).Msg("[alarms] save evidence meta")
		}
	} else if err != nil {
		app.Logger.Warn().Err(err).Uint("alarm", row.ID).Msg("[alarms] save evidence")
	}

	rec = alarmToDTO(row)
	app.Logger.Info().
		Uint("id", row.ID).
		Str("level", level).
		Str("camera", cameraID).
		Str("algorithm", algName).
		Int("boxes", len(ev.Boxes)).
		Msg("[alarms] created from detection")

	s.DispatchAlarm(alarmRecordToMap(rec))
}

func allowAlarmCooldown(key string, d time.Duration) bool {
	if d <= 0 {
		d = time.Duration(defaultAlarmCooldownSec) * time.Second
	}
	now := time.Now()
	alarmCooldownMu.Lock()
	defer alarmCooldownMu.Unlock()
	if last, ok := alarmCooldownAt[key]; ok && now.Sub(last) < d {
		return false
	}
	alarmCooldownAt[key] = now
	return true
}

func normalizeAlarmLevel(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "info", "warn", "critical":
		return strings.ToLower(strings.TrimSpace(v))
	default:
		return "warn"
	}
}

func summarizeBoxes(boxes []vision.VisionBBox) string {
	type hit struct {
		label string
		score float64
	}
	best := map[string]float64{}
	for _, b := range boxes {
		label := strings.TrimSpace(b.Label)
		if label == "" {
			label = strings.TrimSpace(b.Text)
		}
		if label == "" {
			label = "object"
		}
		if b.Score > best[label] {
			best[label] = b.Score
		}
	}
	list := make([]hit, 0, len(best))
	for k, v := range best {
		list = append(list, hit{k, v})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].score == list[j].score {
			return list[i].label < list[j].label
		}
		return list[i].score > list[j].score
	})
	parts := make([]string, 0, len(list))
	for i, h := range list {
		if i >= 5 {
			parts = append(parts, "…")
			break
		}
		parts = append(parts, fmt.Sprintf("%s %.0f%%", h.label, h.score*100))
	}
	if len(parts) == 0 {
		return fmt.Sprintf("检出 %d 个目标", len(boxes))
	}
	return strings.Join(parts, " · ")
}

func (s *Service) resolveCameraMeta(idFromStream string) (name, group, cameraID string) {
	candidates := s.cameraIDCandidates(idFromStream)
	type row struct {
		ID    string `gorm:"column:id"`
		Name  string `gorm:"column:name"`
		Group string `gorm:"column:grp"`
	}
	for _, id := range candidates {
		var r row
		if err := s.store.db.Table("cameras").
			Select("id, name, grp").
			Where("id = ?", id).
			First(&r).Error; err == nil {
			return r.Name, r.Group, r.ID
		}
	}
	if idFromStream != "" {
		return idFromStream, "", idFromStream
	}
	return "未知摄像头", "", ""
}

func (s *Service) findMatchingLinkage(alarm api.AlarmRecord) *api.AlarmLinkageRule {
	rows, err := s.store.listLinkageRules()
	if err != nil {
		return nil
	}
	rules := make([]api.AlarmLinkageRule, 0, len(rows))
	for _, r := range rows {
		rules = append(rules, linkageToDTO(r))
	}
	sort.Slice(rules, func(i, j int) bool {
		if rules[i].Priority == rules[j].Priority {
			return rules[i].Name < rules[j].Name
		}
		return rules[i].Priority < rules[j].Priority
	})
	for i := range rules {
		if ruleMatchesAlarm(rules[i], alarm) {
			return &rules[i]
		}
	}
	return nil
}

func ruleMatchesAlarm(rule api.AlarmLinkageRule, alarm api.AlarmRecord) bool {
	if !rule.Enabled {
		return false
	}
	if len(rule.Levels) > 0 {
		ok := false
		for _, l := range rule.Levels {
			if l == alarm.Level {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if rule.AlgorithmName != "" && rule.AlgorithmName != alarm.AlgorithmName {
		return false
	}
	if rule.CameraGroup != "" && rule.CameraGroup != alarm.CameraGroup {
		return false
	}
	return true
}

func alarmRecordToMap(rec api.AlarmRecord) map[string]any {
	m := map[string]any{
		"id":            rec.ID,
		"title":         rec.Title,
		"level":         rec.Level,
		"status":        rec.Status,
		"cameraId":      rec.CameraID,
		"cameraName":    rec.CameraName,
		"cameraGroup":   rec.CameraGroup,
		"algorithmName": rec.AlgorithmName,
		"message":       rec.Message,
		"occurredAt":    rec.OccurredAt,
	}
	if rec.WorkflowID != "" {
		m["workflowId"] = rec.WorkflowID
		m["workflowName"] = rec.WorkflowName
	}
	if len(rec.EvidenceFileIds) > 0 {
		m["evidenceFileIds"] = rec.EvidenceFileIds
		m["evidencePath"] = rec.EvidenceFileIds[0]
	}
	return m
}

// ——— HTTP 业务方法 ———

func (s *Service) ListAlarms(q api.AlarmQuery) (api.AlarmPage, error) {
	rows, total, err := s.store.listAlarms(q)
	if err != nil {
		return api.AlarmPage{}, err
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	pageSize := q.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	items := make([]api.AlarmRecord, 0, len(rows))
	for _, r := range rows {
		items = append(items, alarmToDTO(r))
	}
	return api.AlarmPage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *Service) UpdateAlarmStatus(id, status string) (api.AlarmRecord, error) {
	status = strings.TrimSpace(status)
	switch status {
	case "open", "ack", "closed":
	default:
		return api.AlarmRecord{}, fmt.Errorf("无效状态: %s", status)
	}
	uid, err := parseRowID(id)
	if err != nil {
		return api.AlarmRecord{}, err
	}
	row, err := s.store.getAlarm(uid)
	if err != nil {
		return api.AlarmRecord{}, err
	}
	row.Status = status
	if err := s.store.saveAlarm(&row); err != nil {
		return api.AlarmRecord{}, err
	}
	return alarmToDTO(row), nil
}

func (s *Service) LinkAlarmWorkflow(id string, workflowID *string, workflowName string) (api.AlarmRecord, error) {
	uid, err := parseRowID(id)
	if err != nil {
		return api.AlarmRecord{}, err
	}
	row, err := s.store.getAlarm(uid)
	if err != nil {
		return api.AlarmRecord{}, err
	}
	if workflowID == nil || strings.TrimSpace(*workflowID) == "" {
		row.WorkflowID = ""
		row.WorkflowName = ""
		row.LinkageRuleID = 0
		row.LinkageRuleName = ""
	} else {
		row.WorkflowID = strings.TrimSpace(*workflowID)
		row.WorkflowName = strings.TrimSpace(workflowName)
		if row.WorkflowName == "" {
			if wf, e := s.store.getWorkflow(row.WorkflowID); e == nil {
				row.WorkflowName = wf.Name
			}
		}
		// 手工联动：清规则来源
		row.LinkageRuleID = 0
		row.LinkageRuleName = ""
	}
	if err := s.store.saveAlarm(&row); err != nil {
		return api.AlarmRecord{}, err
	}
	rec := alarmToDTO(row)
	// 绑定后立即尝试执行（工作流须 running）
	if rec.WorkflowID != "" {
		s.DispatchAlarm(alarmRecordToMap(rec))
	}
	return rec, nil
}

func (s *Service) ListLinkageRules() ([]api.AlarmLinkageRule, error) {
	rows, err := s.store.listLinkageRules()
	if err != nil {
		return nil, err
	}
	out := make([]api.AlarmLinkageRule, 0, len(rows))
	for _, r := range rows {
		out = append(out, linkageToDTO(r))
	}
	return out, nil
}

func (s *Service) CreateLinkageRule(in api.AlarmLinkageRuleInput) (api.AlarmLinkageRule, error) {
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.WorkflowID) == "" {
		return api.AlarmLinkageRule{}, fmt.Errorf("名称与工作流不能为空")
	}
	row := linkageFromInput(in)
	if err := s.store.insertLinkageRule(&row); err != nil {
		return api.AlarmLinkageRule{}, err
	}
	return linkageToDTO(row), nil
}

func (s *Service) UpdateLinkageRule(id string, in api.AlarmLinkageRuleInput) (api.AlarmLinkageRule, error) {
	uid, err := parseRowID(id)
	if err != nil {
		return api.AlarmLinkageRule{}, err
	}
	old, err := s.store.getLinkageRule(uid)
	if err != nil {
		return api.AlarmLinkageRule{}, err
	}
	row := linkageFromInput(in)
	row.ID = old.ID
	row.CreatedAt = old.CreatedAt
	if err := s.store.saveLinkageRule(&row); err != nil {
		return api.AlarmLinkageRule{}, err
	}
	return linkageToDTO(row), nil
}

func (s *Service) DeleteLinkageRule(id string) error {
	uid, err := parseRowID(id)
	if err != nil {
		return err
	}
	return s.store.deleteLinkageRule(uid)
}

// AlarmEvidencePath 返回告警首张抓拍的相对路径（data/alarm-evidence/...）；无则空。
func (s *Service) AlarmEvidencePath(id string) (relPath string, err error) {
	uid, err := parseRowID(id)
	if err != nil {
		return "", err
	}
	row, err := s.store.getAlarm(uid)
	if err != nil {
		return "", err
	}
	ids := parseStringSlice(row.EvidenceIDsJSON)
	if len(ids) == 0 {
		return "", nil
	}
	p := strings.TrimSpace(ids[0])
	if p == "" {
		return "", nil
	}
	// 仅允许 data/alarm-evidence 下相对路径，防目录穿越
	clean := filepath.Clean(filepath.FromSlash(p))
	slash := filepath.ToSlash(clean)
	if !strings.HasPrefix(slash, "data/alarm-evidence/") || strings.Contains(slash, "..") {
		return "", fmt.Errorf("非法证据路径")
	}
	return slash, nil
}

func (s *Service) ApplyLinkageRules(onlyUnlinked bool) (api.ApplyLinkageResult, error) {
	rows, err := s.store.listAllOpenAlarms()
	if err != nil {
		return api.ApplyLinkageResult{}, err
	}
	updated := 0
	for i := range rows {
		row := &rows[i]
		// 手工联动：有 workflow 且无 linkageRuleId
		manual := row.WorkflowID != "" && row.LinkageRuleID == 0
		if onlyUnlinked && manual {
			continue
		}
		rec := alarmToDTO(*row)
		rule := s.findMatchingLinkage(rec)
		changed := false
		if rule == nil {
			if row.LinkageRuleID != 0 {
				row.LinkageRuleID = 0
				row.LinkageRuleName = ""
				row.WorkflowID = ""
				row.WorkflowName = ""
				changed = true
			}
		} else if row.LinkageRuleID != rule.ID || row.WorkflowID != rule.WorkflowID {
			row.LinkageRuleID = rule.ID
			row.LinkageRuleName = rule.Name
			row.WorkflowID = rule.WorkflowID
			row.WorkflowName = rule.WorkflowName
			changed = true
		}
		if changed {
			if err := s.store.saveAlarm(row); err == nil {
				updated++
			}
		}
	}
	return api.ApplyLinkageResult{Updated: updated}, nil
}
