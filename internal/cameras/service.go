package cameras

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/liuhengloveyou/FrameFlow/internal/api"
	"github.com/liuhengloveyou/FrameFlow/internal/app"
	"github.com/liuhengloveyou/FrameFlow/internal/gb28181"
	"github.com/liuhengloveyou/FrameFlow/internal/streams"
	pkggb "github.com/liuhengloveyou/FrameFlow/pkg/gb28181"
)

type service struct {
	store Store
}

func (s *service) List(q api.CameraQuery) api.CameraPage {
	page, err := s.store.List(q)
	if err != nil {
		app.Logger.Error().Err(err).Msg("[cameras] list failed")
		return api.CameraPage{Items: []api.Camera{}, Page: q.Page, PageSize: q.PageSize}
	}
	gb28181.AttachCameraChannels(page.Items)
	for i := range page.Items {
		enrichPushFields(&page.Items[i], "")
	}
	app.Logger.Debug().
		Int("total", page.Total).
		Int("page", page.Page).
		Int("pageSize", page.PageSize).
		Str("name", q.Name).
		Str("protocol", q.Protocol).
		Msg("[cameras] list")
	return page
}

func (s *service) Get(id string) (api.Camera, bool) {
	uid, err := ParseCameraID(id)
	if err != nil {
		return api.Camera{}, false
	}
	c, ok, err := s.store.Get(uid)
	if err != nil {
		app.Logger.Error().Err(err).Str("id", id).Msg("[cameras] get failed")
		return api.Camera{}, false
	}
	if !ok {
		app.Logger.Debug().Str("id", id).Msg("[cameras] get: not found")
		return api.Camera{}, false
	}
	gb28181.AttachCameraChannel(&c)
	enrichPushFields(&c, "")
	app.Logger.Debug().
		Uint("id", c.ID).
		Str("protocol", string(c.Protocol)).
		Str("status", c.Status).
		Int("channels", c.ChannelCount).
		Msg("[cameras] get")
	return c, true
}

func (s *service) Create(in api.CameraInput) (api.Camera, error) {
	app.Logger.Info().
		Str("name", in.Name).
		Str("protocol", string(in.Protocol)).
		Msg("[cameras] create begin")

	cam, err := normalizeCamera("", in)
	if err != nil {
		app.Logger.Warn().Err(err).Str("name", in.Name).Msg("[cameras] create: invalid input")
		return api.Camera{}, err
	}
	cam.Status = "offline"
	cam.Online = false
	if cam.Protocol == api.CameraGB28181 {
		if !gb28181.Enabled() {
			err := fmt.Errorf("gb28181 disabled")
			app.Logger.Warn().Err(err).Msg("[cameras] create failed")
			return api.Camera{}, err
		}
		cam.Enrollment = "active"
	}

	if err := s.store.Insert(&cam); err != nil {
		app.Logger.Warn().Err(err).Str("name", cam.Name).Msg("[cameras] create failed")
		return api.Camera{}, err
	}

	var revealSecret string
	if isRTMPPush(cam) {
		key, genErr := generateRTMPStreamKey()
		if genErr != nil {
			_ = s.store.Delete(cam.ID)
			return api.Camera{}, genErr
		}
		cam.StreamKey = key
		revealSecret, err = setupRTMPPushSlot(&cam, in, nil)
		if err != nil {
			_ = s.store.Delete(cam.ID)
			app.Logger.Warn().Err(err).Uint("id", cam.ID).Msg("[cameras] create: rtmp push slot")
			return api.Camera{}, err
		}
		if err := s.store.Update(cam); err != nil {
			teardownRTMPPushSlot(cam)
			_ = s.store.Delete(cam.ID)
			return api.Camera{}, err
		}
	}
	if isRTSPPush(cam) {
		key, genErr := generateRTSPStreamKey()
		if genErr != nil {
			_ = s.store.Delete(cam.ID)
			return api.Camera{}, genErr
		}
		cam.StreamKey = key
		revealSecret, err = setupRTSPPushSlot(&cam, in, nil)
		if err != nil {
			_ = s.store.Delete(cam.ID)
			app.Logger.Warn().Err(err).Uint("id", cam.ID).Msg("[cameras] create: rtsp push slot")
			return api.Camera{}, err
		}
		if err := s.store.Update(cam); err != nil {
			teardownRTSPPushSlot(cam)
			_ = s.store.Delete(cam.ID)
			return api.Camera{}, err
		}
	}
	if cam.Protocol == api.CameraGB28181 {
		gb28181.UpsertRegistry(cameraToGBDevice(cam))
	}

	app.Logger.Info().
		Uint("id", cam.ID).
		Str("name", cam.Name).
		Str("protocol", string(cam.Protocol)).
		Str("streamUrl", cam.StreamURL).
		Str("ingestMode", string(cam.IngestMode)).
		Msg("[cameras] create ok")
	gb28181.AttachCameraChannel(&cam)
	s.syncStream(cam)
	enrichPushFields(&cam, revealSecret)
	return cam, nil
}

func (s *service) Update(id string, in api.CameraInput) (api.Camera, error) {
	app.Logger.Info().
		Str("id", id).
		Str("name", in.Name).
		Str("protocol", string(in.Protocol)).
		Msg("[cameras] update begin")

	uid, err := ParseCameraID(id)
	if err != nil {
		return api.Camera{}, err
	}
	existing, ok, err := s.store.Get(uid)
	if err != nil {
		app.Logger.Warn().Err(err).Str("id", id).Msg("[cameras] update failed")
		return api.Camera{}, err
	}
	if !ok {
		err := fmt.Errorf("camera not found")
		app.Logger.Warn().Err(err).Str("id", id).Msg("[cameras] update failed")
		return api.Camera{}, err
	}

	cam, err := normalizeCamera(id, in)
	if err != nil {
		app.Logger.Warn().Err(err).Str("id", id).Msg("[cameras] update: invalid input")
		return api.Camera{}, err
	}
	cam.Status = existing.Status
	cam.Online = existing.Online
	cam.Enrollment = existing.Enrollment
	cam.Transport = existing.Transport
	cam.RemoteAddr = existing.RemoteAddr
	cam.ContactURI = existing.ContactURI
	cam.RegisteredAt = existing.RegisteredAt
	cam.LastKeepalive = existing.LastKeepalive
	cam.ProtocolMeta = existing.ProtocolMeta
	cam.CreatedAt = existing.CreatedAt
	cam.ID = uid
	if in.Password == "" {
		cam.Password = existing.Password
	}
	cam.PushSecretHash = existing.PushSecretHash
	cam.AllowAnonymous = existing.AllowAnonymous
	if existing.IngestMode != "" && cam.IngestMode == "" {
		cam.IngestMode = existing.IngestMode
	}
	if cam.StreamKey == "" {
		cam.StreamKey = existing.StreamKey
	}
	// 媒体流传输模式：RTSP / GB28181 / RTMP 拉流可更新；推流不改
	if cam.Protocol == api.CameraRTSP && cam.IngestMode != api.CameraIngestPush {
		if t := strings.ToLower(strings.TrimSpace(in.Transport)); t == "udp" || t == "tcp" {
			cam.Transport = t
		} else if cam.Transport == "" {
			cam.Transport = "tcp"
		}
		if built, berr := buildStreamURL(cam); berr == nil {
			cam.StreamURL = built
		}
	}
	if cam.Protocol == api.CameraGB28181 {
		if t := strings.ToLower(strings.TrimSpace(in.Transport)); t != "" {
			cam.Transport = normalizeGBTransport(t)
		} else if cam.Transport == "" {
			cam.Transport = "udp"
		} else {
			cam.Transport = normalizeGBTransport(cam.Transport)
		}
		ch := cam.Channel
		if ch == "" {
			ch = cam.DeviceID
		}
		cam.StreamURL = "gb28181://" + cam.DeviceID + "/" + ch + "?transport=" + cam.Transport
	}
	if cam.Protocol == api.CameraRTMP && cam.IngestMode != api.CameraIngestPush {
		if t := strings.ToLower(strings.TrimSpace(in.Transport)); t == "udp" || t == "tcp" {
			cam.Transport = t
		} else if cam.Transport == "" {
			cam.Transport = "tcp"
		}
	}

	if cam.Protocol == api.CameraGB28181 && !gb28181.Enabled() {
		err := fmt.Errorf("gb28181 disabled")
		app.Logger.Warn().Err(err).Str("id", id).Msg("[cameras] update failed")
		return api.Camera{}, err
	}

	// 从推流转拉流或换协议：清旧槽位
	if isRTMPPush(existing) && !isRTMPPush(cam) {
		teardownRTMPPushSlot(existing)
		cam.PushSecretHash = ""
		cam.StreamKey = ""
		cam.AllowAnonymous = false
	}
	if isRTSPPush(existing) && !isRTSPPush(cam) {
		teardownRTSPPushSlot(existing)
		cam.PushSecretHash = ""
		cam.StreamKey = ""
		cam.AllowAnonymous = false
	}

	var revealSecret string
	if isRTMPPush(cam) {
		// stream_key 由系统在创建时生成，更新时沿用，不接受客户端改写
		cam.StreamKey = existing.StreamKey
		if cam.StreamKey == "" {
			key, genErr := generateRTMPStreamKey()
			if genErr != nil {
				return api.Camera{}, genErr
			}
			cam.StreamKey = key
		}
		oldKey := strings.TrimSpace(existing.StreamKey)
		newKey := cam.StreamKey
		if oldKey != "" && oldKey != newKey {
			streams.DeleteRTMPIngestSlot(oldKey)
		}
		revealSecret, err = setupRTMPPushSlot(&cam, in, &existing)
		if err != nil {
			app.Logger.Warn().Err(err).Str("id", id).Msg("[cameras] update: rtmp push slot")
			return api.Camera{}, err
		}
	}
	if isRTSPPush(cam) {
		cam.StreamKey = existing.StreamKey
		if cam.StreamKey == "" {
			key, genErr := generateRTSPStreamKey()
			if genErr != nil {
				return api.Camera{}, genErr
			}
			cam.StreamKey = key
		}
		oldKey := strings.TrimSpace(existing.StreamKey)
		newKey := cam.StreamKey
		if oldKey != "" && oldKey != newKey {
			streams.DeleteRTSPIngestSlot(oldKey)
		}
		revealSecret, err = setupRTSPPushSlot(&cam, in, &existing)
		if err != nil {
			app.Logger.Warn().Err(err).Str("id", id).Msg("[cameras] update: rtsp push slot")
			return api.Camera{}, err
		}
	}

	if err := s.store.Update(cam); err != nil {
		app.Logger.Warn().Err(err).Str("id", id).Msg("[cameras] update failed")
		return api.Camera{}, err
	}
	if got, ok, err := s.store.Get(cam.ID); err == nil && ok {
		cam.CreatedAt, cam.UpdatedAt = got.CreatedAt, got.UpdatedAt
	}
	if cam.Protocol == api.CameraGB28181 {
		gb28181.UpsertRegistry(cameraToGBDevice(cam))
	}
	app.Logger.Info().
		Uint("id", cam.ID).
		Str("name", cam.Name).
		Str("protocol", string(cam.Protocol)).
		Str("streamUrl", cam.StreamURL).
		Str("ingestMode", string(cam.IngestMode)).
		Msg("[cameras] update ok")
	gb28181.AttachCameraChannel(&cam)
	s.syncStream(cam)
	enrichPushFields(&cam, revealSecret)
	return cam, nil
}

func (s *service) Delete(id string) error {
	app.Logger.Info().Str("id", id).Msg("[cameras] delete begin")
	uid, err := ParseCameraID(id)
	if err != nil {
		return err
	}
	cam, ok, err := s.store.Get(uid)
	if err != nil {
		app.Logger.Warn().Err(err).Str("id", id).Msg("[cameras] delete failed")
		return err
	}
	if !ok {
		err := fmt.Errorf("camera not found")
		app.Logger.Warn().Err(err).Str("id", id).Msg("[cameras] delete failed")
		return err
	}
	if cam.Protocol == api.CameraGB28181 {
		deviceID := cam.DeviceID
		if deviceID == "" {
			deviceID = CameraStreamName(cam)
		}
		if err := gb28181.DeleteChannels(deviceID); err != nil {
			app.Logger.Warn().Err(err).Str("device", deviceID).Msg("[cameras] delete channels")
		}
		gb28181.RemoveRegistry(deviceID)
	}
	if isRTMPPush(cam) {
		teardownRTMPPushSlot(cam)
	}
	if isRTSPPush(cam) {
		teardownRTSPPushSlot(cam)
	}
	if err := s.store.Delete(uid); err != nil {
		app.Logger.Warn().Err(err).Str("id", id).Msg("[cameras] delete failed")
		return err
	}
	s.removeStream(CameraStreamName(cam))
	app.Logger.Info().
		Str("id", id).
		Str("protocol", string(cam.Protocol)).
		Msg("[cameras] delete ok")
	return nil
}

func (s *service) Start(id string) (api.Camera, error) {
	app.Logger.Info().Str("id", id).Msg("[cameras] start begin")
	uid, err := ParseCameraID(id)
	if err != nil {
		return api.Camera{}, err
	}
	cam, err := s.store.SetStatus(uid, "running")
	if err != nil {
		app.Logger.Warn().Err(err).Str("id", id).Msg("[cameras] start failed")
		return api.Camera{}, err
	}
	app.Logger.Info().Str("id", id).Str("status", cam.Status).Msg("[cameras] start ok")
	return cam, nil
}

func (s *service) Stop(id string) (api.Camera, error) {
	app.Logger.Info().Str("id", id).Msg("[cameras] stop begin")
	uid, err := ParseCameraID(id)
	if err != nil {
		return api.Camera{}, err
	}
	cam, err := s.store.SetStatus(uid, "stopped")
	if err != nil {
		app.Logger.Warn().Err(err).Str("id", id).Msg("[cameras] stop failed")
		return api.Camera{}, err
	}
	app.Logger.Info().Str("id", id).Str("status", cam.Status).Msg("[cameras] stop ok")
	return cam, nil
}

func (s *service) TestStream(id string) (api.StreamTestResult, error) {
	app.Logger.Info().Str("id", id).Msg("[cameras] test-stream begin")
	uid, err := ParseCameraID(id)
	if err != nil {
		return api.StreamTestResult{}, err
	}
	cam, ok, err := s.store.Get(uid)
	if err != nil {
		app.Logger.Warn().Err(err).Str("id", id).Msg("[cameras] test-stream failed")
		return api.StreamTestResult{}, err
	}
	if !ok {
		err := fmt.Errorf("camera not found")
		app.Logger.Warn().Err(err).Str("id", id).Msg("[cameras] test-stream failed")
		return api.StreamTestResult{}, err
	}
	if cam.StreamURL == "" && !isPushIngest(cam) {
		res := api.StreamTestResult{OK: false, Message: "未配置流地址"}
		app.Logger.Info().Str("id", id).Bool("ok", false).Str("message", res.Message).Msg("[cameras] test-stream ok")
		return res, nil
	}
	if isRTMPPush(cam) {
		key := CameraStreamName(cam)
		st := streams.Get(key)
		ok := st != nil
		msg := "RTMP 推流槽位已登记 · " + key
		if !ok {
			msg = "RTMP 推流槽位未就绪 · " + key
		}
		res := api.StreamTestResult{
			OK: ok, LatencyMs: 20,
			Message: msg,
			Codec:   "H.264", Resolution: cam.Resolution,
		}
		app.Logger.Info().Str("id", id).Bool("ok", ok).Str("streamKey", key).Msg("[cameras] test-stream ok")
		return res, nil
	}
	if isRTSPPush(cam) {
		key := CameraStreamName(cam)
		st := streams.Get(key)
		ok := st != nil
		msg := "RTSP 推流槽位已登记 · " + key
		if !ok {
			msg = "RTSP 推流槽位未就绪 · " + key
		}
		res := api.StreamTestResult{
			OK: ok, LatencyMs: 20,
			Message: msg,
			Codec:   "H.264", Resolution: cam.Resolution,
		}
		app.Logger.Info().Str("id", id).Bool("ok", ok).Str("streamKey", key).Msg("[cameras] test-stream ok")
		return res, nil
	}
	if cam.Protocol == api.CameraGB28181 {
		online := cam.Online
		if d, ok := gb28181.GetDevice(cam.DeviceID); ok {
			online = d.Online
		}
		if !online {
			res := api.StreamTestResult{
				OK: false, LatencyMs: 50,
				Message: "国标设备未在线（未完成 SIP 注册）",
			}
			app.Logger.Info().
				Str("id", id).
				Bool("ok", false).
				Str("deviceId", cam.DeviceID).
				Str("message", res.Message).
				Msg("[cameras] test-stream ok")
			return res, nil
		}
		res := api.StreamTestResult{
			OK: true, LatencyMs: 80,
			Message: "国标设备在线 · " + cam.StreamURL,
			Codec:   "PS/H.264", Resolution: cam.Resolution,
		}
		app.Logger.Info().
			Str("id", id).
			Bool("ok", true).
			Str("streamUrl", cam.StreamURL).
			Msg("[cameras] test-stream ok")
		return res, nil
	}
	res := api.StreamTestResult{
		OK: true, LatencyMs: 40,
		Message: "已登记流地址 · " + cam.StreamURL,
		Codec:   "H.264", Resolution: cam.Resolution,
	}
	app.Logger.Info().
		Str("id", id).
		Bool("ok", true).
		Str("protocol", string(cam.Protocol)).
		Str("streamUrl", cam.StreamURL).
		Msg("[cameras] test-stream ok")
	return res, nil
}

func (s *service) removeStream(id string) {
	streams.Delete(id)
	app.Logger.Debug().Str("id", id).Msg("[cameras] stream removed")
}

func normalizeCamera(id string, in api.CameraInput) (api.Camera, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return api.Camera{}, fmt.Errorf("name required")
	}
	protocol := in.Protocol
	if protocol == "" {
		protocol = api.CameraRTSP
	}

	cam := api.Camera{
		Name:           name,
		IP:             strings.TrimSpace(in.IP),
		Port:           in.Port,
		Vendor:         strings.TrimSpace(in.Vendor),
		Group:          strings.TrimSpace(in.Group),
		Resolution:     strings.TrimSpace(in.Resolution),
		FPS:            in.FPS,
		Protocol:       protocol,
		Username:       strings.TrimSpace(in.Username),
		Password:       in.Password,
		Path:           strings.TrimSpace(in.Path),
		Channel:        strings.TrimSpace(in.Channel),
		DeviceID:       strings.TrimSpace(in.DeviceID),
		StreamURL:      strings.TrimSpace(in.StreamURL),
		IngestMode:     in.IngestMode,
		StreamKey:      "", // 推流由系统生成，不接受客户端传入
		AllowAnonymous: true,
		Transport:      strings.ToLower(strings.TrimSpace(in.Transport)),
	}
	if cam.IngestMode == "" {
		cam.IngestMode = api.CameraIngestPull
	}
	if cam.Protocol == api.CameraRTSP && cam.IngestMode != api.CameraIngestPush {
		if cam.Transport != "udp" {
			cam.Transport = "tcp"
		}
	}
	if cam.Protocol == api.CameraGB28181 {
		cam.Transport = normalizeGBTransport(cam.Transport)
	}
	if cam.Protocol == api.CameraRTMP && cam.IngestMode != api.CameraIngestPush {
		// RTMP 协议仅 TCP；仍持久化选项便于 UI 一致（udp 时按 tcp 拉）
		if cam.Transport != "udp" {
			cam.Transport = "tcp"
		}
	}
	if cam.Resolution == "" {
		cam.Resolution = "1920x1080"
	}
	if cam.FPS <= 0 {
		cam.FPS = 25
	}

	switch protocol {
	case api.CameraGB28181:
		deviceID := cam.DeviceID
		if deviceID == "" {
			deviceID = cam.IP
		}
		if deviceID == "" {
			return api.Camera{}, fmt.Errorf("deviceId required (20-digit GB28181 device id)")
		}
		if !isGBID(deviceID) {
			return api.Camera{}, fmt.Errorf("deviceId must be 20 digits")
		}
		channel := cam.Channel
		if channel == "" {
			channel = deviceID
		} else if !isGBID(channel) {
			return api.Camera{}, fmt.Errorf("channel must be 20 digits")
		}
		cam.DeviceID = deviceID
		cam.Channel = channel
		cam.IP = deviceID
		cam.StreamURL = "gb28181://" + deviceID + "/" + channel + "?transport=" + cam.Transport
	case api.CameraHLS:
		if cam.StreamURL == "" {
			return api.Camera{}, fmt.Errorf("streamUrl required for hls")
		}
		if !strings.HasPrefix(strings.ToLower(cam.StreamURL), "http://") &&
			!strings.HasPrefix(strings.ToLower(cam.StreamURL), "https://") {
			return api.Camera{}, fmt.Errorf("hls streamUrl must start with http(s)://")
		}
	case api.CameraRTMP:
		if cam.IngestMode == api.CameraIngestPush {
			// StreamKey 在 Create/Update 由系统随机生成或沿用
			cam.IP = ""
			cam.Port = 0
			break
		}
		fallthrough
	case api.CameraRTSP:
		if cam.IngestMode == api.CameraIngestPush {
			cam.IP = ""
			cam.Port = 0
			break
		}
		fallthrough
	default:
		if cam.IP == "" {
			return api.Camera{}, fmt.Errorf("ip required")
		}
		if cam.Port <= 0 {
			cam.Port = defaultPort(protocol)
		}
		built, err := buildStreamURL(cam)
		if err != nil {
			return api.Camera{}, err
		}
		cam.StreamURL = built
	}

	if id != "" {
		uid, err := ParseCameraID(id)
		if err != nil {
			return api.Camera{}, err
		}
		cam.ID = uid
	}

	return cam, nil
}

func cameraToGBDevice(cam api.Camera) *pkggb.Device {
	status := cam.Enrollment
	if status == "" {
		status = pkggb.DeviceStatusActive
	}
	id := cam.DeviceID
	if id == "" {
		id = strconv.FormatUint(uint64(cam.ID), 10)
	}
	return &pkggb.Device{
		ID:       id,
		Name:     cam.Name,
		Password: cam.Password,
		Status:   status,
		Online:   cam.Online,
	}
}

func isGBID(s string) bool {
	if len(s) != 20 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func defaultPort(p api.CameraProtocol) int {
	switch p {
	case api.CameraRTMP:
		return 1935
	case api.CameraONVIF, api.CameraHLS:
		return 80
	case api.CameraGB28181:
		return 5060
	default:
		return 554
	}
}

// normalizeGBTransport maps UI/API aliases to canonical GB28181 media transport values.
func normalizeGBTransport(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "tcp", "tcp-passive", "passive":
		return "tcp-passive"
	case "tcp-active", "active":
		return "tcp-active"
	case "auto":
		return "auto"
	case "udp":
		return "udp"
	default:
		return "udp"
	}
}

func buildStreamURL(c api.Camera) (string, error) {
	host := c.IP
	port := c.Port
	if port <= 0 {
		port = defaultPort(c.Protocol)
	}
	auth := ""
	if c.Username != "" {
		auth = url.UserPassword(c.Username, c.Password).String() + "@"
	}
	path := strings.TrimLeft(c.Path, "/")

	switch c.Protocol {
	case api.CameraRTSP:
		if path == "" {
			path = "stream1"
		}
		u := fmt.Sprintf("rtsp://%s%s:%d/%s", auth, host, port, path)
		tr := strings.ToLower(strings.TrimSpace(c.Transport))
		if tr == "udp" {
			u += "#transport=udp"
		} else {
			u += "#transport=tcp"
		}
		return u, nil
	case api.CameraRTMP:
		if path == "" {
			path = "live/stream"
		}
		return fmt.Sprintf("rtmp://%s%s:%d/%s", auth, host, port, path), nil
	case api.CameraONVIF:
		// channel is 1-based profile number in the UI/DB; onvif client uses 0-based subtype.
		ch := strings.TrimSpace(c.Channel)
		if ch == "" {
			ch = "1"
		}
		subtype := ch
		if i, err := strconv.Atoi(ch); err == nil && i >= 1 {
			subtype = strconv.Itoa(i - 1)
		}
		return fmt.Sprintf("onvif://%s%s:%d?subtype=%s", auth, host, port, url.QueryEscape(subtype)), nil
	default:
		return "", fmt.Errorf("unsupported protocol %s", c.Protocol)
	}
}
