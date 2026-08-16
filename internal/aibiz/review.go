package aibiz

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/liuhengloveyou/FrameFlow/internal/api"
)

// ListReviewEvents MVP：告警投影为 kind=alert；detection/motion 暂无独立数据源。
func (s *Service) ListReviewEvents(q api.ReviewQuery) (api.ReviewEventPage, error) {
	if q.Kind != "" && q.Kind != "alert" {
		page := q.Page
		if page < 1 {
			page = 1
		}
		pageSize := q.PageSize
		if pageSize < 1 {
			pageSize = 20
		}
		return api.ReviewEventPage{Items: []api.ReviewEvent{}, Total: 0, Page: page, PageSize: pageSize}, nil
	}
	aq := reviewToAlarmQuery(q)
	rows, total, err := s.store.listAlarms(aq)
	if err != nil {
		return api.ReviewEventPage{}, err
	}
	items := make([]api.ReviewEvent, 0, len(rows))
	for _, r := range rows {
		items = append(items, alarmRowToReview(r))
	}
	page := aq.Page
	pageSize := aq.PageSize
	return api.ReviewEventPage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *Service) ReviewKindCounts(q api.ReviewQuery) (api.ReviewKindCounts, error) {
	aq := reviewToAlarmQuery(q)
	aq.Page = 1
	aq.PageSize = 1
	// alert total
	aq.Status = mapReviewStatusToAlarm(q.Status)
	_, alertTotal, err := s.store.listAlarms(aq)
	if err != nil {
		return api.ReviewKindCounts{}, err
	}
	// unreviewed = open
	aqOpen := aq
	aqOpen.Status = "open"
	_, openTotal, err := s.store.listAlarms(aqOpen)
	if err != nil {
		return api.ReviewKindCounts{}, err
	}
	return api.ReviewKindCounts{
		Alert:      alertTotal,
		Detection:  0,
		Motion:     0,
		Unreviewed: openTotal,
	}, nil
}

func (s *Service) GetReviewEvent(id string) (api.ReviewEvent, error) {
	uid, err := parseRowID(id)
	if err != nil {
		return api.ReviewEvent{}, fmt.Errorf("事件不存在")
	}
	row, err := s.store.getAlarm(uid)
	if err != nil {
		return api.ReviewEvent{}, fmt.Errorf("事件不存在")
	}
	return alarmRowToReview(row), nil
}

func (s *Service) ListReviewContext(id string, windowMinutes, page, pageSize int) (api.ReviewEventPage, error) {
	uid, err := parseRowID(id)
	if err != nil {
		return api.ReviewEventPage{}, fmt.Errorf("事件不存在")
	}
	row, err := s.store.getAlarm(uid)
	if err != nil {
		return api.ReviewEventPage{}, fmt.Errorf("事件不存在")
	}
	if windowMinutes <= 0 {
		windowMinutes = 45
	}
	if pageSize < 1 {
		pageSize = 15
	}
	if pageSize > 50 {
		pageSize = 50
	}

	center := parseOccurredAt(row.OccurredAt)
	if center.IsZero() {
		return api.ReviewEventPage{}, fmt.Errorf("事件时间无效")
	}
	fromT := center.Add(-time.Duration(windowMinutes) * time.Minute)
	toT := center.Add(time.Duration(windowMinutes) * time.Minute)

	// 先按摄像头拉取（避免 occurred_at 字符串格式不一致导致 SQL 区间比较丢数据），再按解析时间过滤。
	rows, _, err := s.store.listAlarms(api.AlarmQuery{
		Page:      1,
		PageSize:  500,
		CameraIDs: []string{row.CameraID},
		OrderAsc:  true,
	})
	if err != nil {
		return api.ReviewEventPage{}, err
	}
	filtered := make([]AlarmRow, 0, len(rows))
	centerIdx := -1
	for _, r := range rows {
		at := parseOccurredAt(r.OccurredAt)
		if at.IsZero() || at.Before(fromT) || at.After(toT) {
			continue
		}
		if r.ID == row.ID {
			centerIdx = len(filtered)
		}
		filtered = append(filtered, r)
	}
	total := len(filtered)
	if total == 0 {
		return api.ReviewEventPage{Items: []api.ReviewEvent{}, Total: 0, Page: 1, PageSize: pageSize}, nil
	}

	// page < 1：定位到当前事件所在页
	if page < 1 {
		if centerIdx < 0 {
			page = 1
		} else {
			page = centerIdx/pageSize + 1
		}
	}
	totalPages := (total + pageSize - 1) / pageSize
	if page > totalPages {
		page = totalPages
	}
	start := (page - 1) * pageSize
	end := start + pageSize
	if end > total {
		end = total
	}
	slice := filtered[start:end]
	items := make([]api.ReviewEvent, 0, len(slice))
	for _, r := range slice {
		items = append(items, alarmRowToReview(r))
	}
	return api.ReviewEventPage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func parseOccurredAt(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999999Z07:00",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func (s *Service) MarkReviewEvents(ids []string, status string) (int, error) {
	alarmStatus := mapReviewStatusToAlarm(status)
	if alarmStatus == "" {
		return 0, fmt.Errorf("无效核查状态: %s", status)
	}
	updated := 0
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, err := s.UpdateAlarmStatus(id, alarmStatus); err == nil {
			updated++
		}
	}
	return updated, nil
}

func (s *Service) PromoteReviewDetection(id string) (api.ReviewEvent, error) {
	_ = id
	return api.ReviewEvent{}, fmt.Errorf("当前仅支持告警核查；独立检测提升尚未开放")
}

func reviewToAlarmQuery(q api.ReviewQuery) api.AlarmQuery {
	page := q.Page
	if page < 1 {
		page = 1
	}
	pageSize := q.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	aq := api.AlarmQuery{
		Page:      page,
		PageSize:  pageSize,
		Q:         q.Q,
		From:      q.From,
		To:        q.To,
		Status:    mapReviewStatusToAlarm(q.Status),
		CameraIDs: q.CameraIDs,
	}
	return aq
}

func mapReviewStatusToAlarm(status string) string {
	switch strings.TrimSpace(status) {
	case "unreviewed":
		return "open"
	case "reviewed":
		// 核查完成 → 确认（与告警中心 ack 对齐）
		return "ack"
	default:
		return ""
	}
}

func alarmRowToReview(row AlarmRow) api.ReviewEvent {
	st := "reviewed"
	if row.Status == "open" {
		st = "unreviewed"
	}
	ids := parseStringSlice(row.EvidenceIDsJSON)
	idStr := fmt.Sprintf("%d", row.ID)
	ev := api.ReviewEvent{
		ID:              idStr,
		Kind:            "alert",
		Status:          st,
		Title:           row.Title,
		CameraID:        row.CameraID,
		CameraName:      row.CameraName,
		Label:           row.AlgorithmName,
		ObjectClass:     firstBoxLabel(row.BoxesJSON),
		Score:           firstBoxScore(row.BoxesJSON),
		OccurredAt:      row.OccurredAt,
		EvidenceFileIds: ids,
		Message:         row.Message,
		Boxes:           parseReviewBoxes(row.BoxesJSON),
	}
	if len(ids) > 0 {
		ev.ThumbURL = "/api/v1/alarms/" + idStr + "/evidence"
	}
	return ev
}

func parseReviewBoxes(raw string) []api.ReviewBox {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return nil
	}
	var boxes []struct {
		X     float64 `json:"x"`
		Y     float64 `json:"y"`
		W     float64 `json:"w"`
		H     float64 `json:"h"`
		Label string  `json:"label"`
		Score float64 `json:"score"`
	}
	if err := json.Unmarshal([]byte(raw), &boxes); err != nil {
		return nil
	}
	out := make([]api.ReviewBox, 0, len(boxes))
	for _, b := range boxes {
		out = append(out, api.ReviewBox{X: b.X, Y: b.Y, W: b.W, H: b.H, Label: b.Label})
	}
	return out
}

func firstBoxLabel(raw string) string {
	boxes := parseReviewBoxes(raw)
	if len(boxes) == 0 {
		return ""
	}
	return boxes[0].Label
}

func firstBoxScore(raw string) float64 {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return 0
	}
	var boxes []struct {
		Score float64 `json:"score"`
	}
	if err := json.Unmarshal([]byte(raw), &boxes); err != nil || len(boxes) == 0 {
		return 0
	}
	return boxes[0].Score
}
