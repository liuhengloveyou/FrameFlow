package gb28181

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/liuhengloveyou/FrameFlow/internal/api"
	"github.com/liuhengloveyou/FrameFlow/internal/app"
	"github.com/liuhengloveyou/FrameFlow/internal/streams"
	"github.com/liuhengloveyou/FrameFlow/pkg/database"
	"github.com/liuhengloveyou/FrameFlow/pkg/gb28181"
	"github.com/liuhengloveyou/FrameFlow/pkg/utils"
	"gorm.io/gorm"
)

// Store persists GB28181 devices on the unified cameras table + gb_channels.
type Store struct {
	db *database.DB
}

// cameraDeviceRow maps GB28181 device fields onto the cameras table
// (column-aligned with cameras.CameraRow).
type cameraDeviceRow struct {
	gorm.Model
	Name          string `gorm:"column:name;size:256;not null;default:''"`
	Password      string `gorm:"column:password;size:256;not null;default:''"`
	Status        string `gorm:"column:status;size:32;not null;default:'offline'"`
	Protocol      string `gorm:"column:protocol;size:32;not null;default:''"`
	StreamURL     string `gorm:"column:stream_url;size:1024;not null;default:''"`
	Channel       string `gorm:"column:channel;size:128;not null;default:''"`
	DeviceID      string `gorm:"column:device_id;size:128;not null;default:'';index"`
	IP            string `gorm:"column:ip;size:128;not null;default:''"`
	Port          int    `gorm:"column:port;not null;default:0"`
	Resolution    string `gorm:"column:resolution;size:64;not null;default:''"`
	FPS           int    `gorm:"column:fps;not null;default:0"`
	Enrollment    string `gorm:"column:enrollment;size:32;not null;default:''"`
	Online        int    `gorm:"column:online;not null;default:0"`
	Transport     string `gorm:"column:transport;size:32;not null;default:''"`
	RemoteAddr    string `gorm:"column:remote_addr;size:128;not null;default:''"`
	ContactURI    string `gorm:"column:contact_uri;size:512;not null;default:''"`
	RegisteredAt  string `gorm:"column:registered_at;size:64;not null;default:''"`
	LastKeepalive string `gorm:"column:last_keepalive;size:64;not null;default:''"`
}

func (cameraDeviceRow) TableName() string { return "cameras" }

// ChannelRow 对应 gb_channels：uint 主键；国标通道编码在 channel_id。
type ChannelRow struct {
	gorm.Model
	ChannelID    string `gorm:"column:channel_id;size:64;not null;uniqueIndex:uidx_gb_ch_dev;index"`
	DeviceID     string `gorm:"column:device_id;size:64;not null;uniqueIndex:uidx_gb_ch_dev;index"`
	Name         string `gorm:"column:name;size:256;not null;default:''"`
	Manufacturer string `gorm:"column:manufacturer;size:128;not null;default:''"`
	DeviceModel  string `gorm:"column:model;size:128;not null;default:''"`
	Owner        string `gorm:"column:owner;size:128;not null;default:''"`
	CivilCode    string `gorm:"column:civil_code;size:64;not null;default:''"`
	Address      string `gorm:"column:address;size:256;not null;default:''"`
	Parental     string `gorm:"column:parental;size:32;not null;default:''"`
	Status       string `gorm:"column:status;size:32;not null;default:''"`
}

func (ChannelRow) TableName() string { return "gb_channels" }

var (
	storeOnce sync.Once
	storeInst *Store
	storeErr  error
)

// InitStore 基于系统级 app.DB() 初始化 Store 单例。
func InitStore() error {
	storeOnce.Do(func() {
		storeInst, storeErr = openStore(app.DB())
	})
	return storeErr
}

// DefaultStore 返回系统级 Store 单例；未 InitStore 成功时为 nil。
func DefaultStore() *Store {
	return storeInst
}

func openStore(db *database.DB) (*Store, error) {
	if db == nil {
		return nil, fmt.Errorf("gb28181: database not initialized")
	}
	if err := db.AutoMigrate(&ChannelRow{}); err != nil {
		return nil, fmt.Errorf("gb28181: automigrate: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return nil }

func cameraProtocolGB28181() string { return string(api.CameraGB28181) }

func (s *Store) AddDevice(id, name, password string) (*gb28181.Device, error) {
	return s.addDevice(id, name, password, gb28181.DeviceStatusActive)
}

// DiscoverDevice inserts a self-registered device as pending (or active when forceActive).
// remote is SIP peer address (host:port); used as the default display name when empty.
func (s *Store) DiscoverDevice(id string, forceActive bool, remote string) (gb28181.DeviceAuth, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return gb28181.DeviceAuth{}, fmt.Errorf("device id required")
	}
	if auth, ok, err := s.LookupAuth(id); err != nil {
		return gb28181.DeviceAuth{}, err
	} else if ok {
		return auth, nil
	}
	status := gb28181.DeviceStatusPending
	if forceActive {
		status = gb28181.DeviceStatusActive
	}
	name := defaultDiscoverName(remote)
	d, err := s.addDevice(id, name, "", status)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			auth, _, lerr := s.LookupAuth(id)
			return auth, lerr
		}
		return gb28181.DeviceAuth{}, err
	}
	return gb28181.DeviceAuth{Known: true, Password: d.Password, Status: status}, nil
}

func defaultDiscoverName(remote string) string {
	remote = strings.TrimSpace(remote)
	if remote != "" {
		return remote
	}
	return "未命名设备"
}

func splitHostPort(addr string) (host, port string, err error) {
	return net.SplitHostPort(addr)
}

func (s *Store) addDevice(id, name, password, enrollment string) (*gb28181.Device, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("device id required")
	}
	if enrollment == "" {
		enrollment = gb28181.DeviceStatusActive
	}
	name = strings.TrimSpace(name)
	ip := id
	port := 5060
	// 自发现名称为 host:port 时，拆到 ip/port 列便于展示
	if host, portStr, err := splitHostPort(name); err == nil {
		ip = host
		if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
			port = p
		}
	}
	row := cameraDeviceRow{
		Name: name, Password: password,
		Status: "offline", Protocol: cameraProtocolGB28181(),
		// 通道号等 Catalog 同步后再写入，勿用设备编号冒充通道
		StreamURL: "", Channel: "", DeviceID: id,
		IP: ip, Port: port, Resolution: "1920x1080", FPS: 25,
		Enrollment: enrollment, Online: 0,
	}
	var n int64
	if err := s.db.Model(&cameraDeviceRow{}).
		Where("device_id = ? AND protocol = ?", id, cameraProtocolGB28181()).
		Count(&n).Error; err != nil {
		return nil, err
	}
	if n > 0 {
		return nil, fmt.Errorf("device %s already exists", id)
	}
	err := s.db.Create(&row).Error
	if err != nil {
		if database.IsUniqueErr(err) {
			return nil, fmt.Errorf("device %s already exists", id)
		}
		return nil, err
	}
	return &gb28181.Device{ID: id, Name: name, Password: password, Status: enrollment, Online: false}, nil
}

// SetStatus updates enrollment status on cameras.enrollment.
func (s *Store) SetStatus(id, enrollment string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("device id required")
	}
	switch enrollment {
	case gb28181.DeviceStatusPending, gb28181.DeviceStatusActive,
		gb28181.DeviceStatusRejected, gb28181.DeviceStatusDisabled:
	default:
		return fmt.Errorf("invalid status %q", enrollment)
	}
	res := s.db.Model(&cameraDeviceRow{}).
		Where("device_id = ? AND protocol = ?", id, cameraProtocolGB28181()).
		Updates(map[string]any{
			"enrollment": enrollment,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("device %s not found", id)
	}
	return nil
}

func (s *Store) LookupAuth(id string) (gb28181.DeviceAuth, bool, error) {
	var rows []cameraDeviceRow
	err := s.db.Select("password", "enrollment").
		Where("device_id = ? AND protocol = ?", id, cameraProtocolGB28181()).
		Limit(1).Find(&rows).Error
	if err != nil {
		return gb28181.DeviceAuth{}, false, err
	}
	if len(rows) == 0 {
		return gb28181.DeviceAuth{}, false, nil
	}
	enrollment := rows[0].Enrollment
	if enrollment == "" {
		enrollment = gb28181.DeviceStatusActive
	}
	return gb28181.DeviceAuth{Known: true, Password: rows[0].Password, Status: enrollment}, true, nil
}

func (s *Store) PasswordFor(id string) (string, bool, error) {
	auth, ok, err := s.LookupAuth(id)
	if err != nil || !ok {
		return "", ok, err
	}
	return auth.Password, true, nil
}

func (s *Store) UpdateDevice(id, name, password string, setPassword bool) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("device id required")
	}
	updates := map[string]any{
		"name": strings.TrimSpace(name),
	}
	if setPassword {
		updates["password"] = password
	}
	res := s.db.Model(&cameraDeviceRow{}).
		Where("device_id = ? AND protocol = ?", id, cameraProtocolGB28181()).
		Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("device %s not found", id)
	}
	return nil
}

func (s *Store) DeleteDevice(id string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("device_id = ?", id).Delete(&ChannelRow{}).Error; err != nil {
			return err
		}
		res := tx.Where("device_id = ? AND protocol = ?", id, cameraProtocolGB28181()).Delete(&cameraDeviceRow{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return fmt.Errorf("device %s not found", id)
		}
		return nil
	})
}

// DeleteChannelsOnly removes catalog rows for a device (camera row kept).
func (s *Store) DeleteChannelsOnly(deviceID string) error {
	return s.db.Unscoped().Where("device_id = ?", deviceID).Delete(&ChannelRow{}).Error
}

func (s *Store) GetDevice(id string) (*gb28181.Device, bool, error) {
	var row cameraDeviceRow
	err := s.db.Where("device_id = ? AND protocol = ?", id, cameraProtocolGB28181()).First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	d := rowToDevice(row)
	chs, err := s.ListChannels(id)
	if err != nil {
		return nil, false, err
	}
	d.Channels = make(map[string]*gb28181.Channel, len(chs))
	for _, ch := range chs {
		d.Channels[ch.ID] = ch
	}
	return d, true, nil
}

func (s *Store) ListDevices() ([]*gb28181.Device, error) {
	var rows []cameraDeviceRow
	if err := s.db.Where("protocol = ?", cameraProtocolGB28181()).Order("created_at").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*gb28181.Device, 0, len(rows))
	for _, row := range rows {
		d := rowToDevice(row)
		chs, err := s.ListChannels(d.ID)
		if err != nil {
			return nil, err
		}
		d.Channels = make(map[string]*gb28181.Channel, len(chs))
		for _, ch := range chs {
			d.Channels[ch.ID] = ch
		}
		out = append(out, d)
	}
	return out, nil
}

// ListChannelRows 返回设备下通道表行（含 uint 主键）。
func (s *Store) ListChannelRows(deviceID string) ([]ChannelRow, error) {
	var rows []ChannelRow
	err := s.db.Where("device_id = ?", deviceID).Order("channel_id").Find(&rows).Error
	return rows, err
}

func (s *Store) ListChannels(deviceID string) ([]*gb28181.Channel, error) {
	rows, err := s.ListChannelRows(deviceID)
	if err != nil {
		return nil, err
	}
	out := make([]*gb28181.Channel, 0, len(rows))
	for _, r := range rows {
		out = append(out, &gb28181.Channel{
			ID: r.ChannelID, DeviceID: r.DeviceID, Name: r.Name,
			Manufacturer: r.Manufacturer, Model: r.DeviceModel, Owner: r.Owner,
			CivilCode: r.CivilCode, Address: r.Address, Parental: r.Parental, Status: r.Status,
		})
	}
	return out, nil
}

// NeedsCatalog is true when the device has no catalog channels yet.
func (s *Store) NeedsCatalog(deviceID string) bool {
	var n int64
	if err := s.db.Model(&ChannelRow{}).Where("device_id = ?", deviceID).Count(&n).Error; err != nil {
		return true
	}
	if n == 0 {
		return true
	}
	return false
}

func (s *Store) SaveChannels(deviceID string, channels []*gb28181.Channel) error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("device_id = ?", deviceID).Delete(&ChannelRow{}).Error; err != nil {
			return err
		}
		for _, ch := range channels {
			if ch == nil || strings.TrimSpace(ch.ID) == "" {
				continue
			}
			row := ChannelRow{
				ChannelID: strings.TrimSpace(ch.ID), DeviceID: deviceID, Name: ch.Name,
				Manufacturer: ch.Manufacturer, DeviceModel: ch.Model, Owner: ch.Owner,
				CivilCode: ch.CivilCode, Address: ch.Address, Parental: ch.Parental, Status: ch.Status,
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		updates := map[string]any{}
		if primary := pickPrimaryChannel(deviceID, channels); primary != nil {
			updates["channel"] = primary.ID
			updates["stream_url"] = "gb28181://" + deviceID + "/" + primary.ID
			if primary.Manufacturer != "" {
				updates["vendor"] = primary.Manufacturer
			}
			// 仅在名称为空或仍是 ip:port 默认名时，用通道名覆盖
			if primary.Name != "" {
				updates["name"] = gorm.Expr(
					"CASE WHEN name = '' OR name LIKE '%.%.%.%:%' THEN ? ELSE name END",
					primary.Name,
				)
			}
		}
		if len(updates) == 0 {
			return tx.Model(&cameraDeviceRow{}).
				Where("device_id = ? AND protocol = ?", deviceID, cameraProtocolGB28181()).
				Update("updated_at", time.Now().UTC()).Error
		}
		return tx.Model(&cameraDeviceRow{}).
			Where("device_id = ? AND protocol = ?", deviceID, cameraProtocolGB28181()).
			Updates(updates).Error
	})
	if err != nil {
		return err
	}
	syncChannelStreams(deviceID, channels)
	return nil
}

func syncChannelStreams(deviceID string, channels []*gb28181.Channel) {
	primary := pickPrimaryChannel(deviceID, channels)
	if primary != nil {
		url := "gb28181://" + deviceID + "/" + primary.ID
		if _, err := streams.Patch(deviceID, url); err != nil {
			app.Logger.Warn().Err(err).Str("device", deviceID).Msg("[gb28181] sync default stream")
		}
	}
	for _, ch := range channels {
		if ch == nil || ch.ID == "" {
			continue
		}
		url := "gb28181://" + deviceID + "/" + ch.ID
		name := deviceID
		if ch.ID != deviceID {
			name = deviceID + "/" + ch.ID
		}
		if name == deviceID && primary != nil && primary.ID == ch.ID {
			continue
		}
		if _, err := streams.Patch(name, url); err != nil {
			app.Logger.Warn().Err(err).Str("name", name).Msg("[gb28181] sync channel stream")
			continue
		}
	}
}

// pickPrimaryChannel prefers a catalog channel id that differs from the device id
// (IPC 常见：设备号 …0002，通道号 …0111).
func pickPrimaryChannel(deviceID string, channels []*gb28181.Channel) *gb28181.Channel {
	var fallback *gb28181.Channel
	for _, ch := range channels {
		if ch == nil || ch.ID == "" {
			continue
		}
		if fallback == nil {
			fallback = ch
		}
		if ch.ID != deviceID {
			return ch
		}
	}
	return fallback
}

func (s *Store) MarkOnline(d *gb28181.Device) error {
	online := 0
	status := "offline"
	if d.Online {
		online = 1
		status = "online"
	}
	reg := ""
	if !d.Registered.IsZero() {
		reg = utils.FormatISO(d.Registered)
	}
	keep := ""
	if !d.LastKeepalive.IsZero() {
		keep = utils.FormatISO(d.LastKeepalive)
	}
	updates := map[string]any{
		"transport":      d.Transport,
		"remote_addr":    d.RemoteAddr,
		"contact_uri":    d.ContactURI,
		"online":         online,
		"status":         status,
		"registered_at":  reg,
		"last_keepalive": keep,
	}
	// 无名称时用对端地址补默认名
	if strings.TrimSpace(d.RemoteAddr) != "" {
		updates["name"] = gorm.Expr(
			"CASE WHEN name = '' OR name IS NULL THEN ? ELSE name END",
			d.RemoteAddr,
		)
		if host, portStr, err := splitHostPort(d.RemoteAddr); err == nil {
			updates["ip"] = gorm.Expr(
				"CASE WHEN ip = '' OR ip = device_id THEN ? ELSE ip END",
				host,
			)
			if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
				updates["port"] = gorm.Expr(
					"CASE WHEN port = 0 OR port = 5060 THEN ? ELSE port END",
					p,
				)
			}
		}
	}
	return s.db.Model(&cameraDeviceRow{}).
		Where("device_id = ? AND protocol = ?", d.ID, cameraProtocolGB28181()).
		Updates(updates).Error
}

func (s *Store) TouchKeepalive(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("device id required")
	}
	now := utils.NowISO()
	res := s.db.Model(&cameraDeviceRow{}).
		Where("device_id = ? AND protocol = ?", id, cameraProtocolGB28181()).
		Updates(map[string]any{
			"online":         1,
			"status":         "online",
			"last_keepalive": now,
			// 自发现空名称时，用已落库的 remote_addr 补默认名
			"name": gorm.Expr(
				"CASE WHEN (name = '' OR name IS NULL) AND remote_addr != '' THEN remote_addr ELSE name END",
			),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("device %s not found", id)
	}
	return nil
}

func (s *Store) MarkOffline(id string) error {
	return s.db.Model(&cameraDeviceRow{}).
		Where("device_id = ? AND protocol = ?", id, cameraProtocolGB28181()).
		Updates(map[string]any{
			"online": 0,
			"status": "offline",
		}).Error
}

func rowToDevice(r cameraDeviceRow) *gb28181.Device {
	enrollment := r.Enrollment
	if enrollment == "" {
		enrollment = gb28181.DeviceStatusActive
	}
	return &gb28181.Device{
		ID: r.DeviceID, Name: r.Name, Password: r.Password, Status: enrollment,
		Transport: r.Transport, RemoteAddr: r.RemoteAddr, ContactURI: r.ContactURI,
		Online: r.Online != 0, Registered: utils.ParseISO(r.RegisteredAt), LastKeepalive: utils.ParseISO(r.LastKeepalive),
	}
}
