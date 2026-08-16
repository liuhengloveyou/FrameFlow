package aibiz

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/liuhengloveyou/FrameFlow/internal/app"
	"github.com/liuhengloveyou/FrameFlow/internal/vision"
)

// registerStreamAIPolicy 把「流 → DB running 绑定 → 算法 → 上传模型」解析器注入 vision。
func (s *Service) registerStreamAIPolicy() {
	vision.SetStreamAIPolicyResolver(s.resolveStreamAIPolicy)
}

func (s *Service) resolveStreamAIPolicy(streamKey string) (vision.StreamAIPolicy, bool) {
	cameraID, channelID, ok := s.resolveCameraChannelFromStream(streamKey)
	if !ok {
		return vision.StreamAIPolicy{}, false
	}
	candidates := s.cameraIDCandidates(cameraID)
	var running *BindingRow
	var algName string
	for _, cam := range candidates {
		rows, err := s.store.listBindings(cam, channelID)
		if err != nil {
			continue
		}
		for i := range rows {
			if rows[i].Status != "running" {
				continue
			}
			running = &rows[i]
			if a, e := s.store.getAlgorithm(rows[i].AlgorithmID); e == nil {
				algName = a.Name
			}
			break
		}
		if running != nil {
			break
		}
	}
	if running == nil {
		return vision.StreamAIPolicy{
			Enabled:       false,
			AlgorithmName: algName,
		}, true
	}

	policy := vision.DefaultStreamAIPolicy()
	policy.Enabled = true
	policy.BindID = running.ID
	policy.AlgorithmName = algName
	policy.Regions = regionsToNorm(parseRegions(running.RegionsJSON))

	// 模型路径来自算法关联的上传文件；检测阈值只读绑定 rules（每路可不同）。
	if alg, err := s.store.getAlgorithm(running.AlgorithmID); err == nil {
		if policy.AlgorithmName == "" {
			policy.AlgorithmName = alg.Name
		}
		if m, err := s.store.getModel(alg.ModelID); err == nil {
			policy.ModelPath = resolveModelPath(m.LocalPath)
			policy.LabelsPath = resolveModelPath(m.LabelsPath)
		}
	}

	rules := parseRulesMap(running.RulesJSON)
	if v, ok := floatRule(rules, "conf"); ok {
		policy.Conf = float32(v)
	}
	if v, ok := floatRule(rules, "iou"); ok {
		policy.NMSIoU = float32(v)
	}
	if v, ok := floatRule(rules, "max_det"); ok && v >= 1 {
		policy.MaxDet = int(v)
	}
	if lvl, ok := stringRule(rules, "alarm_level"); ok && lvl != "" {
		policy.AlarmLevel = lvl
	}
	if v, ok := floatRule(rules, "cooldown_sec"); ok && v >= 0 {
		policy.CooldownSec = v
	}
	if b, ok := boolRule(rules, "pull_keyframe_only"); ok {
		policy.PullKeyframeOnly = b
	}
	if v, ok := floatRule(rules, "pull_sample_ms"); ok && v >= 50 {
		policy.PullSampleMS = int(v)
	}
	if v, ok := floatRule(rules, "pull_max_gop_mb"); ok && v >= 1 {
		policy.PullMaxGOPBytes = int(v) << 20
	}
	return policy, true
}

// resolveCameraChannelFromStream 把 vision/streams 键解析为摄像头与通道。
// 支持：gb28181://device/channel；以及 RTSP/RTMP 推流槽位名 ingest_*（查 cameras.stream_key）。
func (s *Service) resolveCameraChannelFromStream(streamKey string) (cameraID, channelID string, ok bool) {
	if cam, ch, ok := parseStreamCameraChannel(streamKey); ok {
		return cam, ch, true
	}
	return s.lookupPushIngestCamera(streamKey)
}

// lookupPushIngestCamera：推流设备的 streams 键就是 stream_key（如 ingest_dd09…），需反查 cameras 表。
func (s *Service) lookupPushIngestCamera(streamKey string) (cameraID, channelID string, ok bool) {
	streamKey = strings.TrimSpace(streamKey)
	if streamKey == "" || strings.Contains(streamKey, "://") {
		return "", "", false
	}
	var row struct {
		ID uint `gorm:"column:id"`
	}
	err := s.store.db.Table("cameras").
		Select("id").
		Where("stream_key = ?", streamKey).
		Limit(1).
		Take(&row).Error
	if err != nil || row.ID == 0 {
		return "", "", false
	}
	cameraID = strconv.FormatUint(uint64(row.ID), 10)

	var binds []BindingRow
	if err := s.store.db.Where("camera_id = ? AND status = ?", cameraID, "running").
		Find(&binds).Error; err == nil && len(binds) > 0 {
		return cameraID, binds[0].ChannelID, true
	}
	if err := s.store.db.Where("camera_id = ?", cameraID).Find(&binds).Error; err == nil && len(binds) > 0 {
		return cameraID, binds[0].ChannelID, true
	}
	// 尚无绑定：与前端推流相机默认通道一致
	return cameraID, "1", true
}

// parseStreamCameraChannel 从 streams 键解析摄像头/通道。
// 支持：gb28181://deviceId/channelId?stream=sub
func parseStreamCameraChannel(streamKey string) (cameraID, channelID string, ok bool) {
	streamKey = strings.TrimSpace(streamKey)
	if streamKey == "" {
		return "", "", false
	}
	u, err := url.Parse(streamKey)
	if err != nil {
		return "", "", false
	}
	switch strings.ToLower(u.Scheme) {
	case "gb28181":
		cameraID = strings.TrimSpace(u.Host)
		channelID = strings.Trim(u.Path, "/")
		if i := strings.IndexByte(channelID, '/'); i >= 0 {
			channelID = channelID[:i]
		}
		return cameraID, channelID, cameraID != "" && channelID != ""
	default:
		// 其它协议：推流槽位由 lookupPushIngestCamera 处理
		return "", "", false
	}
}

// cameraIDCandidates：stream 里的 id 可能是 cameras.id 或 device_id。
func (s *Service) cameraIDCandidates(idFromStream string) []string {
	idFromStream = strings.TrimSpace(idFromStream)
	out := []string{idFromStream}
	seen := map[string]bool{idFromStream: true}
	var rows []struct {
		ID uint `gorm:"column:id"`
	}
	if err := s.store.db.Table("cameras").
		Select("id").
		Where("id = ? OR device_id = ?", idFromStream, idFromStream).
		Find(&rows).Error; err != nil {
		app.Logger.Debug().Err(err).Str("id", idFromStream).Msg("[aibiz] camera id lookup")
		return out
	}
	for _, r := range rows {
		sid := strconv.FormatUint(uint64(r.ID), 10)
		if sid == "" || sid == "0" || seen[sid] {
			continue
		}
		seen[sid] = true
		out = append(out, sid)
	}
	return out
}

func regionsToNorm(raw [][]map[string]any) [][]vision.NormPoint {
	out := make([][]vision.NormPoint, 0, len(raw))
	for _, poly := range raw {
		pts := make([]vision.NormPoint, 0, len(poly))
		for _, p := range poly {
			x, _ := floatRule(p, "x")
			y, _ := floatRule(p, "y")
			pts = append(pts, vision.NormPoint{X: x, Y: y})
		}
		if len(pts) >= 3 {
			out = append(out, pts)
		}
	}
	return out
}

func floatRule(m map[string]any, key string) (float64, bool) {
	if m == nil {
		return 0, false
	}
	v, ok := m[key]
	if !ok || v == nil {
		return 0, false
	}
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	default:
		return 0, false
	}
}

func stringRule(m map[string]any, key string) (string, bool) {
	if m == nil {
		return "", false
	}
	v, ok := m[key]
	if !ok || v == nil {
		return "", false
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t), true
	default:
		return "", false
	}
}

func boolRule(m map[string]any, key string) (bool, bool) {
	if m == nil {
		return false, false
	}
	v, ok := m[key]
	if !ok || v == nil {
		return false, false
	}
	switch t := v.(type) {
	case bool:
		return t, true
	case float64:
		return t != 0, true
	case int:
		return t != 0, true
	case string:
		s := strings.TrimSpace(strings.ToLower(t))
		if s == "true" || s == "1" || s == "yes" {
			return true, true
		}
		if s == "false" || s == "0" || s == "no" {
			return false, true
		}
		return false, false
	default:
		return false, false
	}
}