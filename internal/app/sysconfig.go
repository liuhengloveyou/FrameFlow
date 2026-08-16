package app

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

const sysConfigRowID = 1

// SysConfigRow 系统级单行配置。列类型与下方业务配置结构体一一对应（GORM JSON serializer）。
type SysConfigRow struct {
	gorm.Model
	RTMP    RTMPConfig    `gorm:"column:rtmp;serializer:json;not null"`
	RTSP    RTSPConfig    `gorm:"column:rtsp;serializer:json;not null"`
	ONVIF   ONVIFConfig   `gorm:"column:onvif;serializer:json"`   // nullable: SQLite ADD COLUMN 不能 NOT NULL 无 DEFAULT
	GB28181 GB28181Config `gorm:"column:gb28181;serializer:json"` // nullable: 同上
}

func (SysConfigRow) TableName() string { return "sys_config" }

// RTMPConfig 系统 RTMP 接入配置（sys_config.rtmp）。由 internal/rtmp 使用。
type RTMPConfig struct {
	Enabled        bool   `json:"enabled"`
	ListenPort     int    `json:"listenPort"`
	AdvertiseHost  string `json:"advertiseHost"`
	PushTokenQuery string `json:"pushTokenQuery,omitempty"`
}

// DefaultRTMPConfig 默认关闭，端口预填 1935。
func DefaultRTMPConfig() RTMPConfig {
	return RTMPConfig{Enabled: false, ListenPort: 1935}
}

// Normalize 补全默认端口。
func (c *RTMPConfig) Normalize() {
	if c.ListenPort <= 0 || c.ListenPort > 65535 {
		c.ListenPort = 1935
	}
}

// ListenAddr 启用时返回 ":port"；未启用返回空。
func (c RTMPConfig) ListenAddr() string {
	if !c.Enabled {
		return ""
	}
	port := c.ListenPort
	if port <= 0 {
		port = 1935
	}
	return fmt.Sprintf(":%d", port)
}

// UnmarshalJSON 兼容旧格式 {"listen":":1935",...}。
func (c *RTMPConfig) UnmarshalJSON(b []byte) error {
	type wire struct {
		Enabled        bool   `json:"enabled"`
		ListenPort     int    `json:"listenPort"`
		AdvertiseHost  string `json:"advertiseHost"`
		PushTokenQuery string `json:"pushTokenQuery"`
		Listen         string `json:"listen"`
	}
	var w wire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	*c = RTMPConfig{
		Enabled:        w.Enabled,
		ListenPort:     w.ListenPort,
		AdvertiseHost:  w.AdvertiseHost,
		PushTokenQuery: w.PushTokenQuery,
	}
	if !c.Enabled && c.ListenPort == 0 {
		if p := parseListenPortString(w.Listen); p > 0 {
			c.Enabled = true
			c.ListenPort = p
		}
	}
	c.Normalize()
	return nil
}

// RTSPConfig 系统 RTSP 服务配置（sys_config.rtsp）。由 internal/rtsp 使用。
type RTSPConfig struct {
	Enabled       bool   `json:"enabled"`
	ListenPort    int    `json:"listenPort"`
	AdvertiseHost string `json:"advertiseHost"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	DefaultQuery  string `json:"defaultQuery"`
	PacketSize    int    `json:"packetSize,omitempty"`
}

// DefaultRTSPConfig 默认关闭，端口预填 8554。
func DefaultRTSPConfig() RTSPConfig {
	return RTSPConfig{Enabled: false, ListenPort: 8554, DefaultQuery: "video&audio"}
}

// Normalize 补全默认端口与 defaultQuery。
func (c *RTSPConfig) Normalize() {
	if c.ListenPort <= 0 || c.ListenPort > 65535 {
		c.ListenPort = 8554
	}
	if strings.TrimSpace(c.DefaultQuery) == "" {
		c.DefaultQuery = "video&audio"
	}
}

// ListenAddr 启用时返回 ":port"；未启用返回空。
func (c RTSPConfig) ListenAddr() string {
	if !c.Enabled {
		return ""
	}
	port := c.ListenPort
	if port <= 0 {
		port = 8554
	}
	return fmt.Sprintf(":%d", port)
}

// UnmarshalJSON 兼容旧格式 {"listen":":8554",...}。
func (c *RTSPConfig) UnmarshalJSON(b []byte) error {
	type wire struct {
		Enabled       bool   `json:"enabled"`
		ListenPort    int    `json:"listenPort"`
		AdvertiseHost string `json:"advertiseHost"`
		Username      string `json:"username"`
		Password      string `json:"password"`
		DefaultQuery  string `json:"defaultQuery"`
		PacketSize    int    `json:"packetSize"`
		Listen        string `json:"listen"`
	}
	var w wire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	*c = RTSPConfig{
		Enabled:       w.Enabled,
		ListenPort:    w.ListenPort,
		AdvertiseHost: w.AdvertiseHost,
		Username:      w.Username,
		Password:      w.Password,
		DefaultQuery:  w.DefaultQuery,
		PacketSize:    w.PacketSize,
	}
	if !c.Enabled && c.ListenPort == 0 {
		if p := parseListenPortString(w.Listen); p > 0 {
			c.Enabled = true
			c.ListenPort = p
		}
	}
	c.Normalize()
	return nil
}

// ONVIFConfig 系统 ONVIF 配置（sys_config.onvif）。
// Client：发现默认账号；Server：是否对外提供 /onvif/ 设备服务。
type ONVIFConfig struct {
	ServerEnabled      bool   `json:"serverEnabled"`
	DefaultUsername    string `json:"defaultUsername"`
	DefaultPassword    string `json:"defaultPassword"`
	DiscoverTimeoutSec int    `json:"discoverTimeoutSec"`
}

// DefaultONVIFConfig 默认开启本机 ONVIF 服务端，发现超时 5s。
func DefaultONVIFConfig() ONVIFConfig {
	return ONVIFConfig{ServerEnabled: true, DiscoverTimeoutSec: 5}
}

// Normalize 补全发现超时。
func (c *ONVIFConfig) Normalize() {
	if c.DiscoverTimeoutSec <= 0 || c.DiscoverTimeoutSec > 60 {
		c.DiscoverTimeoutSec = 5
	}
}

// GB28181Config 系统国标平台 SIP/媒体配置（sys_config.gb28181）。由 internal/gb28181 使用。
type GB28181Config struct {
	Enabled      bool   `json:"enabled"`
	ListenPort   int    `json:"listenPort"`
	Serial       string `json:"serial"`
	Realm        string `json:"realm"`
	Password     string `json:"password,omitempty"`
	SipIP        string `json:"sipIp"`
	MediaIP      string `json:"mediaIp"`
	MediaPort    string `json:"mediaPort"`
	Transport    string `json:"transport"`
	AllowUnknown bool   `json:"allowUnknown"`
}

// DefaultGB28181Config 默认关闭，SIP 端口预填 5060。
func DefaultGB28181Config() GB28181Config {
	return GB28181Config{
		Enabled:    false,
		ListenPort: 5060,
		Serial:     "34020000002000000001",
		Realm:      "3402000000",
		MediaPort:  "10000-20000",
		Transport:  "udp",
	}
}

// Normalize 补全端口与常用默认值。
func (c *GB28181Config) Normalize() {
	if c.ListenPort <= 0 || c.ListenPort > 65535 {
		c.ListenPort = 5060
	}
	if strings.TrimSpace(c.Serial) == "" {
		c.Serial = "34020000002000000001"
	}
	if strings.TrimSpace(c.Realm) == "" {
		c.Realm = "3402000000"
	}
	if strings.TrimSpace(c.MediaPort) == "" {
		c.MediaPort = "10000-20000"
	}
	if strings.TrimSpace(c.Transport) == "" {
		c.Transport = "udp"
	}
}

// ListenAddr 启用时返回 ":port"；未启用返回空。
func (c GB28181Config) ListenAddr() string {
	if !c.Enabled {
		return ""
	}
	port := c.ListenPort
	if port <= 0 {
		port = 5060
	}
	return fmt.Sprintf(":%d", port)
}

// IsUnset 判断列是否尚未写入（NULL/零值），用于 YAML 一次性种子。
func (c GB28181Config) IsUnset() bool {
	return !c.Enabled && c.ListenPort == 0 && strings.TrimSpace(c.Serial) == "" &&
		strings.TrimSpace(c.Realm) == "" && strings.TrimSpace(c.MediaPort) == ""
}

func parseListenPortString(listen string) int {
	listen = strings.TrimSpace(listen)
	if listen == "" {
		return 0
	}
	if _, port, err := net.SplitHostPort(listen); err == nil {
		if p, err := strconv.Atoi(port); err == nil && p > 0 {
			return p
		}
	}
	if strings.HasPrefix(listen, ":") {
		if p, err := strconv.Atoi(strings.TrimPrefix(listen, ":")); err == nil && p > 0 {
			return p
		}
	}
	return 0
}

// GetRTMPConfig 读取系统 RTMP 配置；无行则返回默认值。
func GetRTMPConfig() (RTMPConfig, error) {
	if systemDB == nil {
		return DefaultRTMPConfig(), fmt.Errorf("app: database not initialized")
	}
	var row SysConfigRow
	err := systemDB.First(&row, sysConfigRowID).Error
	if err == gorm.ErrRecordNotFound {
		return DefaultRTMPConfig(), nil
	}
	if err != nil {
		return DefaultRTMPConfig(), err
	}
	row.RTMP.Normalize()
	return row.RTMP, nil
}

// PutRTMPConfig 写入系统 RTMP 配置。
func PutRTMPConfig(cfg RTMPConfig) error {
	cfg.Normalize()
	return putSysConfig(func(r *SysConfigRow) { r.RTMP = cfg })
}

// GetRTSPConfig 读取系统 RTSP 配置；无行则返回默认值。
func GetRTSPConfig() (RTSPConfig, error) {
	if systemDB == nil {
		return DefaultRTSPConfig(), fmt.Errorf("app: database not initialized")
	}
	var row SysConfigRow
	err := systemDB.First(&row, sysConfigRowID).Error
	if err == gorm.ErrRecordNotFound {
		return DefaultRTSPConfig(), nil
	}
	if err != nil {
		return DefaultRTSPConfig(), err
	}
	row.RTSP.Normalize()
	return row.RTSP, nil
}

// PutRTSPConfig 写入系统 RTSP 配置。
func PutRTSPConfig(cfg RTSPConfig) error {
	cfg.Normalize()
	return putSysConfig(func(r *SysConfigRow) { r.RTSP = cfg })
}

// GetONVIFConfig 读取系统 ONVIF 配置；无行则返回默认值。
func GetONVIFConfig() (ONVIFConfig, error) {
	if systemDB == nil {
		return DefaultONVIFConfig(), fmt.Errorf("app: database not initialized")
	}
	var row SysConfigRow
	err := systemDB.First(&row, sysConfigRowID).Error
	if err == gorm.ErrRecordNotFound {
		return DefaultONVIFConfig(), nil
	}
	if err != nil {
		return DefaultONVIFConfig(), err
	}
	// 新加列在已有行上为 NULL，GORM 解成零值；与「未配置」同等对待。
	if !row.ONVIF.ServerEnabled && row.ONVIF.DefaultUsername == "" && row.ONVIF.DefaultPassword == "" && row.ONVIF.DiscoverTimeoutSec == 0 {
		return DefaultONVIFConfig(), nil
	}
	row.ONVIF.Normalize()
	return row.ONVIF, nil
}

// PutONVIFConfig 写入系统 ONVIF 配置。
func PutONVIFConfig(cfg ONVIFConfig) error {
	cfg.Normalize()
	return putSysConfig(func(r *SysConfigRow) { r.ONVIF = cfg })
}

// GetGB28181Config 读取系统国标配置；无行或列未写入则返回默认值。
func GetGB28181Config() (GB28181Config, error) {
	cfg, _, err := GetGB28181ConfigEx()
	return cfg, err
}

// GetGB28181ConfigEx 同 GetGB28181Config，并返回是否已从库中读到有效配置（非 unset）。
func GetGB28181ConfigEx() (cfg GB28181Config, persisted bool, err error) {
	if systemDB == nil {
		return DefaultGB28181Config(), false, fmt.Errorf("app: database not initialized")
	}
	var row SysConfigRow
	err = systemDB.First(&row, sysConfigRowID).Error
	if err == gorm.ErrRecordNotFound {
		return DefaultGB28181Config(), false, nil
	}
	if err != nil {
		return DefaultGB28181Config(), false, err
	}
	if row.GB28181.IsUnset() {
		return DefaultGB28181Config(), false, nil
	}
	row.GB28181.Normalize()
	return row.GB28181, true, nil
}

// PutGB28181Config 写入系统国标配置。
func PutGB28181Config(cfg GB28181Config) error {
	cfg.Normalize()
	return putSysConfig(func(r *SysConfigRow) { r.GB28181 = cfg })
}

// ParseListenPort 解析 ":5060" / "0.0.0.0:5060" 形式的监听地址端口。
func ParseListenPort(listen string) int {
	return parseListenPortString(listen)
}

func putSysConfig(mutate func(*SysConfigRow)) error {
	if systemDB == nil {
		return fmt.Errorf("app: database not initialized")
	}
	var row SysConfigRow
	err := systemDB.First(&row, sysConfigRowID).Error
	if err == gorm.ErrRecordNotFound {
		row = SysConfigRow{
			Model: gorm.Model{ID: sysConfigRowID},
			RTMP:  DefaultRTMPConfig(),
			RTSP:  DefaultRTSPConfig(),
			ONVIF: DefaultONVIFConfig(),
			// GB28181 保持零值直至 PutGB28181Config，以便 YAML 一次性种子
		}
		mutate(&row)
		return systemDB.Create(&row).Error
	}
	if err != nil {
		return err
	}
	mutate(&row)
	row.ID = sysConfigRowID
	return systemDB.Save(&row).Error
}
