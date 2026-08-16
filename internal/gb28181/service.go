package gb28181

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/liuhengloveyou/FrameFlow/internal/api"
	"github.com/liuhengloveyou/FrameFlow/internal/app"
	"github.com/liuhengloveyou/FrameFlow/pkg/gb28181"
	"github.com/liuhengloveyou/FrameFlow/pkg/utils"
)

// UpsertRegistry loads a device into the in-memory registry (no DB write).
// Used by cameras CRUD after writing the unified cameras row.
func UpsertRegistry(d *gb28181.Device) {
	if gw == nil || d == nil {
		return
	}
	gw.Registry.UpsertDevice(d)
}

// RemoveRegistry removes a device from the in-memory registry.
func RemoveRegistry(id string) {
	if gw == nil {
		return
	}
	gw.Registry.Delete(id)
}

// DeleteChannels removes gb_channels for a device without deleting the camera row.
func DeleteChannels(deviceID string) error {
	if DefaultStore() == nil {
		return nil
	}
	return DefaultStore().DeleteChannelsOnly(deviceID)
}

// Enabled reports whether the GB28181 SIP module started successfully.
func Enabled() bool {
	return endpoint != nil
}

// Status returns platform/runtime summary for GET /api/gb28181.
func Status() map[string]any {
	row, err := app.GetGB28181Config()
	if err != nil {
		row = app.DefaultGB28181Config()
	}
	pkg := toPkgConfig(row)
	out := map[string]any{
		"enabled":       row.Enabled,
		"listen":        pkg.Listen,
		"serial":        pkg.Serial,
		"realm":         pkg.Realm,
		"sip_ip":        pkg.SipIP,
		"media_ip":      pkg.MediaIP,
		"media_port":    pkg.MediaPort,
		"transport":     pkg.Transport,
		"db":            dbDSN(),
		"allow_unknown": pkg.AllowUnknown,
		"running":       endpoint != nil,
	}
	if gw == nil {
		out["devices"] = 0
		out["online"] = 0
		out["pending"] = 0
		return out
	}
	devices := ListDevices()
	online, pending := 0, 0
	for _, d := range devices {
		if d.Online {
			online++
		}
		if d.Status == gb28181.DeviceStatusPending {
			pending++
		}
	}
	out["devices"] = len(devices)
	out["online"] = online
	out["pending"] = pending
	return out
}

func dbDSN() string {
	if db := app.DB(); db != nil {
		return db.Driver + ":" + db.DSN
	}
	return ""
}

// ListDevices returns persisted devices with runtime online/contact overlay from the registry.
func ListDevices() []*gb28181.Device {
	if DefaultStore() == nil {
		if gw == nil {
			return nil
		}
		return gw.Registry.List()
	}
	list, err := DefaultStore().ListDevices()
	if err != nil {
		app.Logger.Warn().Err(err).Msg("[gb28181] list devices")
		if gw != nil {
			return gw.Registry.List()
		}
		return nil
	}
	if gw == nil {
		return list
	}
	for _, d := range list {
		if live, ok := gw.Registry.Get(d.ID); ok {
			d.Online = live.Online
			d.RemoteAddr = live.RemoteAddr
			d.ContactURI = live.ContactURI
			d.Transport = live.Transport
			d.Registered = live.Registered
			d.LastKeepalive = live.LastKeepalive
			if live.Status != "" {
				d.Status = live.Status
			}
			if len(live.Channels) > 0 {
				d.Channels = live.Channels
			}
		}
	}
	return list
}

// GetDevice returns a device by id.
func GetDevice(id string) (*gb28181.Device, bool) {
	if gw != nil {
		if d, ok := gw.Registry.Get(id); ok {
			return d, true
		}
	}
	if DefaultStore() == nil {
		return nil, false
	}
	d, ok, err := DefaultStore().GetDevice(id)
	if err != nil || !ok {
		return nil, false
	}
	return d, true
}

// AddDevice persists a pre-registered device and loads it into the registry.
func AddDevice(id, name, password string) (*gb28181.Device, error) {
	if gw == nil {
		return nil, fmt.Errorf("gb28181 disabled")
	}
	if DefaultStore() == nil {
		return nil, fmt.Errorf("store unavailable")
	}
	d, err := DefaultStore().AddDevice(id, name, password)
	if err != nil {
		return nil, err
	}
	gw.Registry.UpsertDevice(d)
	app.Logger.Info().Str("device", d.ID).Str("name", d.Name).Str("status", d.Status).Msg("[gb28181] device added")
	return d, nil
}

// ConfirmDevice marks a pending device as active (admin approve), then refresh Catalog.
func ConfirmDevice(id string) (*gb28181.Device, error) {
	d, err := setDeviceStatus(id, gb28181.DeviceStatusActive)
	if err != nil {
		return nil, err
	}
	if d != nil {
		remote := d.RemoteAddr
		go func(deviceID, remoteAddr string) {
			_ = EnsureDeviceReachable(deviceID, remoteAddr)
			if err := RequestCatalog(deviceID, true); err != nil {
				app.Logger.Warn().Err(err).Str("device", deviceID).Msg("[gb28181] confirm: catalog query failed")
			}
		}(id, remote)
	}
	return d, nil
}

// RejectDevice marks a device as rejected; further REGISTER returns 403.
func RejectDevice(id string) (*gb28181.Device, error) {
	d, err := setDeviceStatus(id, gb28181.DeviceStatusRejected)
	if err != nil {
		return nil, err
	}
	if gw != nil {
		gw.Registry.SetOffline(id)
	}
	if DefaultStore() != nil {
		_ = DefaultStore().MarkOffline(id)
	}
	return d, nil
}

func setDeviceStatus(id, status string) (*gb28181.Device, error) {
	if gw == nil {
		return nil, fmt.Errorf("gb28181 disabled")
	}
	if DefaultStore() == nil {
		return nil, fmt.Errorf("store unavailable")
	}
	if err := DefaultStore().SetStatus(id, status); err != nil {
		return nil, err
	}
	if d, ok := gw.Registry.GetMutable(id); ok {
		d.Status = status
	} else if d, ok, _ := DefaultStore().GetDevice(id); ok {
		gw.Registry.UpsertDevice(d)
	}
	d, ok := GetDevice(id)
	if !ok {
		return nil, fmt.Errorf("device %s not found", id)
	}
	app.Logger.Info().Str("device", id).Str("status", status).Msg("[gb28181] device status updated")
	return d, nil
}

// UpdateDevice updates name and optionally password.
func UpdateDevice(id, name string, password *string) (*gb28181.Device, error) {
	if gw == nil {
		return nil, fmt.Errorf("gb28181 disabled")
	}
	if DefaultStore() == nil {
		return nil, fmt.Errorf("store unavailable")
	}
	pass := ""
	setPass := false
	if password != nil {
		pass = *password
		setPass = true
	}
	if err := DefaultStore().UpdateDevice(id, name, pass, setPass); err != nil {
		return nil, err
	}
	if d, ok := gw.Registry.GetMutable(id); ok {
		d.Name = name
		if setPass {
			d.Password = pass
		}
	}
	d, ok := gw.Registry.Get(id)
	if !ok {
		return nil, fmt.Errorf("device %s not found", id)
	}
	return d, nil
}

// DeleteDevice removes device from sqlite and memory.
func DeleteDevice(id string) error {
	if gw == nil {
		return fmt.Errorf("gb28181 disabled")
	}
	if DefaultStore() == nil {
		return fmt.Errorf("store unavailable")
	}
	if err := DefaultStore().DeleteDevice(id); err != nil {
		return err
	}
	gw.Registry.Delete(id)
	return nil
}

// ListChannels returns channels for a device.
func ListChannels(deviceID string) ([]*gb28181.Channel, error) {
	if DefaultStore() == nil {
		dev, ok := GetDevice(deviceID)
		if !ok {
			return nil, fmt.Errorf("device not found")
		}
		chs := make([]*gb28181.Channel, 0, len(dev.Channels))
		for _, c := range dev.Channels {
			chs = append(chs, c)
		}
		return chs, nil
	}
	return DefaultStore().ListChannels(deviceID)
}

// CameraChannels maps persisted GB channels to product CameraChannel DTOs.
func CameraChannels(deviceID, defaultChannel string) ([]api.CameraChannel, error) {
	if st := DefaultStore(); st != nil {
		rows, err := st.ListChannelRows(deviceID)
		if err != nil {
			return nil, err
		}
		out := make([]api.CameraChannel, 0, len(rows))
		for _, r := range rows {
			if strings.TrimSpace(r.ChannelID) == "" {
				continue
			}
			out = append(out, api.CameraChannel{
				ID:           r.ID,
				ChannelID:    r.ChannelID,
				DeviceID:     deviceID,
				Name:         r.Name,
				Manufacturer: r.Manufacturer,
				Model:        r.DeviceModel,
				Status:       r.Status,
				StreamURL:    "gb28181://" + deviceID + "/" + r.ChannelID,
			})
		}
		markDefaultChannel(out, deviceID, defaultChannel)
		return out, nil
	}

	chs, err := ListChannels(deviceID)
	if err != nil {
		return nil, err
	}
	out := make([]api.CameraChannel, 0, len(chs))
	for _, c := range chs {
		if c == nil || c.ID == "" {
			continue
		}
		out = append(out, api.CameraChannel{
			ChannelID:    c.ID,
			DeviceID:     deviceID,
			Name:         c.Name,
			Manufacturer: c.Manufacturer,
			Model:        c.Model,
			Status:       c.Status,
			StreamURL:    "gb28181://" + deviceID + "/" + c.ID,
		})
	}
	markDefaultChannel(out, deviceID, defaultChannel)
	return out, nil
}

func markDefaultChannel(out []api.CameraChannel, deviceID, defaultChannel string) {
	if len(out) == 0 {
		return
	}
	def := 0
	if defaultChannel != "" {
		for i := range out {
			if out[i].ChannelID == defaultChannel {
				def = i
				break
			}
		}
	} else {
		for i := range out {
			if out[i].ChannelID != deviceID {
				def = i
				break
			}
		}
	}
	out[def].IsDefault = true
}

// AttachCameraChannels fills Channels / ChannelCount on GB28181 camera DTOs.
func AttachCameraChannels(cams []api.Camera) {
	for i := range cams {
		attachOne(&cams[i])
	}
}

func AttachCameraChannel(cam *api.Camera) {
	if cam == nil {
		return
	}
	attachOne(cam)
}

func attachOne(cam *api.Camera) {
	if cam.Protocol != api.CameraGB28181 {
		// 非国标：合成单通道，便于统一「设备+流」模型
		if cam.StreamURL == "" {
			cam.Channels = nil
			cam.ChannelCount = 0
			return
		}
		chID := cam.Channel
		if chID == "" {
			chID = "1"
		}
		cam.Channels = []api.CameraChannel{{
			ChannelID: chID,
			DeviceID:  strconv.FormatUint(uint64(cam.ID), 10),
			Name:      cam.Name,
			Status:    cam.Status,
			StreamURL: cam.StreamURL,
			IsDefault: true,
		}}
		cam.ChannelCount = 1
		return
	}
	deviceID := cam.DeviceID
	if deviceID == "" && cam.ID != 0 {
		deviceID = strconv.FormatUint(uint64(cam.ID), 10)
	}
	if d, ok := GetDevice(deviceID); ok {
		cam.Online = d.Online
		if d.Online {
			cam.Status = "online"
		} else if cam.Status == "online" {
			cam.Status = "offline"
		}
		if !d.LastKeepalive.IsZero() {
			cam.LastKeepalive = utils.FormatISO(d.LastKeepalive)
		} else if !d.Registered.IsZero() && cam.LastKeepalive == "" {
			cam.LastKeepalive = utils.FormatISO(d.Registered)
		}
		if d.RemoteAddr != "" {
			cam.RemoteAddr = d.RemoteAddr
		}
	}
	chs, err := CameraChannels(deviceID, cam.Channel)
	if err != nil {
		app.Logger.Warn().Err(err).Str("device", deviceID).Msg("[gb28181] attach channels")
		cam.Channels = []api.CameraChannel{}
		cam.ChannelCount = 0
		return
	}
	cam.Channels = chs
	cam.ChannelCount = len(chs)
	tr := mediaTransportQuery(cam.Transport)
	if cam.StreamURL != "" {
		cam.StreamURL = withGB28181Transport(cam.StreamURL, tr)
	}
	for i := range cam.Channels {
		if cam.Channels[i].StreamURL != "" {
			cam.Channels[i].StreamURL = withGB28181Transport(cam.Channels[i].StreamURL, tr)
		}
	}
}

func mediaTransportQuery(t string) string {
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

func withGB28181Transport(rawURL, transport string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" || !strings.HasPrefix(strings.ToLower(rawURL), "gb28181://") {
		return rawURL
	}
	// strip existing transport=
	cleaned := rawURL
	for _, p := range []string{
		"transport=udp",
		"transport=tcp-passive",
		"transport=tcp-active",
		"transport=tcp",
		"transport=auto",
	} {
		cleaned = strings.ReplaceAll(cleaned, "?"+p+"&", "?")
		cleaned = strings.ReplaceAll(cleaned, "&"+p+"&", "&")
		cleaned = strings.ReplaceAll(cleaned, "?"+p, "")
		cleaned = strings.ReplaceAll(cleaned, "&"+p, "")
	}
	cleaned = strings.TrimSuffix(cleaned, "?")
	cleaned = strings.TrimSuffix(cleaned, "&")
	sep := "?"
	if strings.Contains(cleaned, "?") {
		sep = "&"
	}
	return cleaned + sep + "transport=" + transport
}

// TriggerCatalog sends a Catalog query to the device (manual sync, skips debounce).
func TriggerCatalog(deviceID string) error {
	if endpoint == nil {
		return fmt.Errorf("gb28181 disabled")
	}
	remote := ""
	if d, ok := GetDevice(deviceID); ok {
		remote = d.RemoteAddr
	}
	if err := EnsureDeviceReachable(deviceID, remote); err != nil {
		return err
	}
	return RequestCatalog(deviceID, true)
}

// PTZ sends a DeviceControl PTZ command to channelID via deviceID's SIP session.
func PTZ(deviceID, channelID, cmd string, speed byte) (string, error) {
	if endpoint == nil {
		return "", fmt.Errorf("gb28181 disabled")
	}
	ptz, err := gb28181.BuildPTZCmd(cmd, speed)
	if err != nil {
		return "", err
	}
	xmlBody := gb28181.BuildDeviceControl(channelID, ptz)
	if err := endpoint.SendMessage(deviceID, "Application/MANSCDP+xml", xmlBody); err != nil {
		return "", err
	}
	return ptz, nil
}

// QueryRecords queries GB28181 RecordInfo for a channel time range.
func QueryRecords(deviceID, channelID string, start, end time.Time) ([]gb28181.RecordItem, error) {
	if endpoint == nil || gw == nil {
		return nil, fmt.Errorf("gb28181 disabled")
	}
	if gw.QueryRecordInfo == nil {
		return nil, fmt.Errorf("gb28181: recordinfo not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 65*time.Second)
	defer cancel()
	return gw.QueryRecordInfo(ctx, deviceID, channelID, start, end)
}
