package gb28181

// Device enrollment / admin review status.
const (
	DeviceStatusPending  = "pending"  // 设备主动注册，待管理员确认
	DeviceStatusActive   = "active"   // 已确认（预登记或管理员确认）
	DeviceStatusRejected = "rejected" // 已拒绝，后续 REGISTER 返回 403
	DeviceStatusDisabled = "disabled" // 停用
)

// AllowsRegister reports whether SIP REGISTER may succeed for this status.
func AllowsRegister(status string) bool {
	switch status {
	case DeviceStatusPending, DeviceStatusActive, "":
		return true
	default:
		return false
	}
}

// AllowsInvite reports whether live INVITE / play is permitted.
func AllowsInvite(status string) bool {
	return status == DeviceStatusActive || status == ""
}

// DeviceAuth is the business lookup result for REGISTER.
type DeviceAuth struct {
	Known    bool
	Password string
	Status   string // pending|active|rejected|disabled；Known=false 时为空
}

// Handler is the business-side policy for SIP protocol events.
// The protocol SIPEndpoint owns SIP/Digest/Registry; Handler owns persistence and product rules.
type Handler interface {
	// LookupDevice returns store state for deviceID (Known=false if never seen).
	LookupDevice(deviceID string) (DeviceAuth, error)

	// OnDiscover persists a newly seen device as pending (or active when AllowUnknown).
	// Called only when LookupDevice reports Known=false.
	// remote is SIP peer "ip:port" used as a default display name.
	OnDiscover(deviceID, remote string) (DeviceAuth, error)

	// OnRegistered is called after a successful REGISTER (200) and Registry.MergeOnline.
	OnRegistered(dev *Device)

	// OnUnregistered is called after Expires=0 unregister.
	OnUnregistered(deviceID string)

	// OnKeepalive is called for SIP MESSAGE Keepalive.
	// remote is the SIP peer address. Returns false if the platform should
	// reject the keepalive (e.g. rejected/disabled enrollment).
	OnKeepalive(deviceID, remote string) (ok bool)

	// OnCatalog is called after channels are upserted into the Registry.
	OnCatalog(deviceID string, channels []*Channel)
}

// NopHandler ignores all business callbacks (useful for protocol tests).
type NopHandler struct{}

func (NopHandler) LookupDevice(string) (DeviceAuth, error) {
	return DeviceAuth{Known: true, Status: DeviceStatusActive}, nil
}
func (NopHandler) OnDiscover(string, string) (DeviceAuth, error) {
	return DeviceAuth{Known: true, Status: DeviceStatusPending}, nil
}
func (NopHandler) OnRegistered(*Device)  {}
func (NopHandler) OnUnregistered(string) {}
func (NopHandler) OnKeepalive(string, string) bool {
	return true
}
func (NopHandler) OnCatalog(string, []*Channel) {}
