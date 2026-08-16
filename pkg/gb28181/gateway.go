package gb28181

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Config holds GB28181 SIP/media settings.
type Config struct {
	Listen       string `yaml:"listen" json:"listen"`
	Serial       string `yaml:"serial" json:"serial"`
	Realm        string `yaml:"realm" json:"realm"`
	Password     string `yaml:"password" json:"-"`
	SipIP        string `yaml:"sip_ip" json:"sip_ip"`
	MediaIP      string `yaml:"media_ip" json:"media_ip"`
	MediaPort    string `yaml:"media_port" json:"media_port"`
	Transport    string `yaml:"transport" json:"transport"`
	DB           string `yaml:"db,omitempty" json:"db,omitempty"` // deprecated: use system database:
	AllowUnknown bool   `yaml:"allow_unknown" json:"allow_unknown"` // true=未知设备直接 active；false=写入 pending 待确认
}

func DefaultConfig() Config {
	return Config{
		Listen:       "", // empty = disabled; set listen in yaml to enable
		Serial:       "34020000002000000001",
		Realm:        "3402000000",
		Password:     "",
		MediaPort:    "10000-20000",
		Transport:    TransportUDP,
		AllowUnknown: false,
	}
}

// Gateway is the shared SIP+registry handle used by Producer Dial.
type Gateway struct {
	Config   Config
	Registry *Registry
	Ports    *PortPool

	mu       sync.Mutex
	inviteSN atomic.Uint32

	// SendMessage sends a SIP MESSAGE to a registered device.
	SendMessage func(deviceID, contentType, body string) error
	// InviteLive sends INVITE for live play; media socket is already listening on mediaPort (except tcp-active).
	// streamProfile: "main" | "sub" (empty = sub). answer is the 200 OK SDP body (may be empty).
	InviteLive func(deviceID, channelID, transport string, mediaPort uint16, ssrc, streamProfile string) (callID string, bye func() error, answer []byte, err error)
	// InvitePlayback sends INVITE for playback of [start, end].
	InvitePlayback func(deviceID, channelID, transport string, mediaPort uint16, ssrc string, start, end time.Time) (callID string, bye func() error, answer []byte, err error)
	// QueryRecordInfo queries device recording catalog for a channel and time range.
	QueryRecordInfo func(ctx context.Context, deviceID, channelID string, start, end time.Time) ([]RecordItem, error)
}

var (
	defaultGW   *Gateway
	defaultGWMu sync.RWMutex
)

func SetDefaultGateway(g *Gateway) {
	defaultGWMu.Lock()
	defaultGW = g
	defaultGWMu.Unlock()
}

func DefaultGateway() *Gateway {
	defaultGWMu.RLock()
	defer defaultGWMu.RUnlock()
	return defaultGW
}

func (g *Gateway) NextSSRC() string {
	n := g.inviteSN.Add(1)
	return MakeSSRC(n)
}

func (g *Gateway) KeepaliveTimeout() time.Duration {
	return 3 * time.Minute
}
