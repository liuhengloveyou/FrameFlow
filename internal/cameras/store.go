package cameras

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/liuhengloveyou/FrameFlow/internal/api"
	"github.com/liuhengloveyou/FrameFlow/pkg/database"
	"github.com/liuhengloveyou/FrameFlow/pkg/utils"
	"gorm.io/gorm"
)

// Store 摄像头持久化（系统级 GORM；含国标平铺字段）。
type Store interface {
	List(q api.CameraQuery) (api.CameraPage, error)
	Get(id uint) (api.Camera, bool, error)
	Insert(c *api.Camera) error
	Update(c api.Camera) error
	Delete(id uint) error
	SetStatus(id uint, status string) (api.Camera, error)
	Close() error
}

// CameraRow 对应 cameras 表（协议字段超集 + protocol_meta JSON）。
type CameraRow struct {
	gorm.Model
	Name       string `gorm:"column:name;size:256;not null;default:''"`
	IP         string `gorm:"column:ip;size:128;not null;default:''"`
	Port       int    `gorm:"column:port;not null;default:0"`
	Vendor     string `gorm:"column:vendor;size:128;not null;default:''"`
	Group      string `gorm:"column:grp;size:128;not null;default:'';index"`
	Resolution string `gorm:"column:resolution;size:64;not null;default:'1920x1080'"`
	FPS        int    `gorm:"column:fps;not null;default:25"`
	Status     string `gorm:"column:status;size:32;not null;default:'offline';index"` // online|offline|…
	Protocol   string `gorm:"column:protocol;size:32;not null;default:'rtsp';index"`
	StreamURL  string `gorm:"column:stream_url;size:1024;not null;default:''"`
	Username   string `gorm:"column:username;size:128;not null;default:''"`
	Password   string `gorm:"column:password;size:256;not null;default:''"`
	Path       string `gorm:"column:path;size:256;not null;default:''"`
	Channel    string `gorm:"column:channel;size:128;not null;default:''"`
	DeviceID   string `gorm:"column:device_id;size:128;not null;default:'';index"`

	// 国标平铺
	Enrollment    string `gorm:"column:enrollment;size:32;not null;default:'';index"` // pending|active|…
	Online        int    `gorm:"column:online;not null;default:0"`
	Transport     string `gorm:"column:transport;size:32;not null;default:''"`
	RemoteAddr    string `gorm:"column:remote_addr;size:128;not null;default:''"`
	ContactURI    string `gorm:"column:contact_uri;size:512;not null;default:''"`
	RegisteredAt  string `gorm:"column:registered_at;size:64;not null;default:''"`
	LastKeepalive string `gorm:"column:last_keepalive;size:64;not null;default:''"`
	ProtocolMeta  string `gorm:"column:protocol_meta;type:text;not null;default:''"`

	// RTMP 推流接入
	IngestMode     string `gorm:"column:ingest_mode;size:16;not null;default:'pull'"`
	StreamKey      string `gorm:"column:stream_key;size:128;not null;default:''"`
	PushSecretHash string `gorm:"column:push_secret_hash;size:128;not null;default:''"`
	AllowAnonymous int    `gorm:"column:allow_anonymous;not null;default:0"`
}

func (CameraRow) TableName() string { return "cameras" }

type gormStore struct {
	db *database.DB
}

// NewStore 打开系统库并迁移 cameras 表。
func NewStore(db *database.DB) (Store, error) {
	if db == nil {
		return nil, fmt.Errorf("cameras: database not initialized")
	}
	if err := db.AutoMigrate(&CameraRow{}); err != nil {
		return nil, fmt.Errorf("cameras: automigrate: %w", err)
	}
	return &gormStore{db: db}, nil
}

func (s *gormStore) Close() error { return nil }

func (s *gormStore) List(q api.CameraQuery) (api.CameraPage, error) {
	page := q.Page
	if page < 1 {
		page = 1
	}
	pageSize := q.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	tx := s.db.Model(&CameraRow{})
	if name := strings.TrimSpace(q.Name); name != "" {
		tx = tx.Where("LOWER(name) LIKE ?", "%"+strings.ToLower(name)+"%")
	}
	if q.Status != "" {
		tx = tx.Where("status = ?", q.Status)
	}
	if q.Group != "" {
		tx = tx.Where("grp = ?", q.Group)
	}
	if q.Protocol != "" {
		tx = tx.Where("protocol = ?", q.Protocol)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return api.CameraPage{}, err
	}

	var rows []CameraRow
	err := tx.Order("updated_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error
	if err != nil {
		return api.CameraPage{}, err
	}

	items := make([]api.Camera, 0, len(rows))
	for i := range rows {
		items = append(items, rowToCamera(rows[i]))
	}
	return api.CameraPage{
		Items:    items,
		Total:    int(total),
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *gormStore) Get(id uint) (api.Camera, bool, error) {
	var row CameraRow
	err := s.db.First(&row, id).Error
	if err == gorm.ErrRecordNotFound {
		return api.Camera{}, false, nil
	}
	if err != nil {
		return api.Camera{}, false, err
	}
	return rowToCamera(row), true, nil
}

func (s *gormStore) Insert(c *api.Camera) error {
	row := cameraToRow(*c)
	err := s.db.Create(row).Error
	if err != nil {
		if database.IsUniqueErr(err) {
			return fmt.Errorf("camera already exists")
		}
		return err
	}
	c.ID = row.ID
	c.CreatedAt = utils.FormatISO(row.CreatedAt)
	c.UpdatedAt = utils.FormatISO(row.UpdatedAt)
	return nil
}

func (s *gormStore) Update(c api.Camera) error {
	row := cameraToRow(c)
	res := s.db.Model(&CameraRow{}).Where("id = ?", c.ID).Select(
		"name", "ip", "port", "vendor", "grp", "resolution", "fps", "status", "protocol",
		"stream_url", "username", "password", "path", "channel", "device_id",
		"enrollment", "online", "transport", "remote_addr", "contact_uri",
		"registered_at", "last_keepalive", "protocol_meta",
		"ingest_mode", "stream_key", "push_secret_hash", "allow_anonymous",
	).Updates(row)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("camera not found")
	}
	return nil
}

func (s *gormStore) Delete(id uint) error {
	res := s.db.Delete(&CameraRow{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("camera not found")
	}
	return nil
}

func (s *gormStore) SetStatus(id uint, status string) (api.Camera, error) {
	c, ok, err := s.Get(id)
	if err != nil {
		return api.Camera{}, err
	}
	if !ok {
		return api.Camera{}, fmt.Errorf("camera not found")
	}
	c.Status = status
	if err := s.Update(c); err != nil {
		return api.Camera{}, err
	}
	out, _, err := s.Get(id)
	return out, err
}

func cameraToRow(c api.Camera) *CameraRow {
	online := 0
	if c.Online {
		online = 1
	}
	allowAnon := 0
	if c.AllowAnonymous {
		allowAnon = 1
	}
	ingest := string(c.IngestMode)
	if ingest == "" {
		ingest = string(api.CameraIngestPull)
	}
	row := &CameraRow{
		Name: c.Name, IP: c.IP, Port: c.Port, Vendor: c.Vendor, Group: c.Group,
		Resolution: c.Resolution, FPS: c.FPS, Status: c.Status, Protocol: string(c.Protocol),
		StreamURL: c.StreamURL, Username: c.Username, Password: c.Password,
		Path: c.Path, Channel: c.Channel, DeviceID: c.DeviceID,
		Enrollment: c.Enrollment, Online: online,
		Transport: c.Transport, RemoteAddr: c.RemoteAddr, ContactURI: c.ContactURI,
		RegisteredAt: c.RegisteredAt, LastKeepalive: c.LastKeepalive, ProtocolMeta: c.ProtocolMeta,
		IngestMode: ingest, StreamKey: c.StreamKey, PushSecretHash: c.PushSecretHash,
		AllowAnonymous: allowAnon,
	}
	row.ID = c.ID
	return row
}

func rowToCamera(r CameraRow) api.Camera {
	mode := api.CameraIngestMode(r.IngestMode)
	if mode == "" {
		mode = api.CameraIngestPull
	}
	return api.Camera{
		ID: r.ID, Name: r.Name, IP: r.IP, Port: r.Port, Vendor: r.Vendor, Group: r.Group,
		Resolution: r.Resolution, FPS: r.FPS, Status: r.Status, Protocol: api.CameraProtocol(r.Protocol),
		StreamURL: r.StreamURL, Username: r.Username, Password: r.Password,
		Path: r.Path, Channel: r.Channel, DeviceID: r.DeviceID,
		UpdatedAt: utils.FormatISO(r.UpdatedAt), CreatedAt: utils.FormatISO(r.CreatedAt),
		Enrollment: r.Enrollment, Online: r.Online != 0,
		Transport: r.Transport, RemoteAddr: r.RemoteAddr, ContactURI: r.ContactURI,
		RegisteredAt: r.RegisteredAt, LastKeepalive: r.LastKeepalive, ProtocolMeta: r.ProtocolMeta,
		IngestMode: mode, StreamKey: r.StreamKey, PushSecretHash: r.PushSecretHash,
		AllowAnonymous: r.AllowAnonymous != 0,
	}
}

// ParseCameraID 解析路径/查询中的摄像头主键。
func ParseCameraID(id string) (uint, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return 0, fmt.Errorf("invalid camera id")
	}
	n, err := strconv.ParseUint(id, 10, 64)
	if err != nil || n == 0 {
		return 0, fmt.Errorf("invalid camera id")
	}
	return uint(n), nil
}

// CameraStreamName 预览/配置用的流名：国标用 device_id，推流用 stream_key，其余用数字 id。
func CameraStreamName(cam api.Camera) string {
	if cam.Protocol == api.CameraGB28181 && strings.TrimSpace(cam.DeviceID) != "" {
		return strings.TrimSpace(cam.DeviceID)
	}
	if (cam.Protocol == api.CameraRTMP || cam.Protocol == api.CameraRTSP) && cam.IngestMode == api.CameraIngestPush {
		return strings.TrimSpace(cam.StreamKey)
	}
	if cam.ID == 0 {
		return ""
	}
	return strconv.FormatUint(uint64(cam.ID), 10)
}