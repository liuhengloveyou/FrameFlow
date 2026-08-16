package gb28181

import (
	"sync"
	"time"
)

// Channel is a GB28181 media channel under a device.
type Channel struct {
	ID         string `json:"id"`
	DeviceID   string `json:"device_id"`
	Name       string `json:"name,omitempty"`
	Manufacturer string `json:"manufacturer,omitempty"`
	Model      string `json:"model,omitempty"`
	Owner      string `json:"owner,omitempty"`
	CivilCode  string `json:"civil_code,omitempty"`
	Address    string `json:"address,omitempty"`
	Parental   string `json:"parental,omitempty"`
	SafetyWay  string `json:"safety_way,omitempty"`
	RegisterWay string `json:"register_way,omitempty"`
	Secrecy    string `json:"secrecy,omitempty"`
	Status     string `json:"status,omitempty"`
}

// Device is a registered GB28181 endpoint (IPC/NVR).
type Device struct {
	ID            string              `json:"id"`
	Name          string              `json:"name,omitempty"`
	Transport     string              `json:"transport,omitempty"` // udp / tcp
	RemoteAddr    string              `json:"remote_addr,omitempty"`
	ContactURI    string              `json:"contact_uri,omitempty"`
	Password      string              `json:"-"`
	Status        string              `json:"status,omitempty"` // pending|active|rejected|disabled
	Online        bool                `json:"online"`
	Registered    time.Time           `json:"registered,omitempty"`
	Expires       time.Duration       `json:"expires,omitempty"`
	LastKeepalive time.Time           `json:"last_keepalive,omitempty"`
	Channels      map[string]*Channel `json:"channels,omitempty"`
}

// Registry stores devices and channels in memory.
type Registry struct {
	mu      sync.RWMutex
	devices map[string]*Device
}

func NewRegistry() *Registry {
	return &Registry{devices: make(map[string]*Device)}
}

func (r *Registry) UpsertDevice(d *Device) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if old, ok := r.devices[d.ID]; ok && d.Channels == nil {
		d.Channels = old.Channels
	}
	if d.Channels == nil {
		d.Channels = make(map[string]*Channel)
	}
	r.devices[d.ID] = d
}

func (r *Registry) Get(id string) (*Device, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.devices[id]
	if !ok {
		return nil, false
	}
	cp := *d
	cp.Channels = make(map[string]*Channel, len(d.Channels))
	for k, v := range d.Channels {
		ch := *v
		cp.Channels[k] = &ch
	}
	return &cp, true
}

func (r *Registry) GetMutable(id string) (*Device, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.devices[id]
	return d, ok
}

func (r *Registry) SetOffline(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if d, ok := r.devices[id]; ok {
		d.Online = false
	}
}

func (r *Registry) TouchKeepalive(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if d, ok := r.devices[id]; ok {
		d.LastKeepalive = time.Now()
		d.Online = true
	}
}

func (r *Registry) UpsertChannels(deviceID string, channels []*Channel) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.devices[deviceID]
	if !ok {
		d = &Device{ID: deviceID, Channels: make(map[string]*Channel), Online: true}
		r.devices[deviceID] = d
	}
	if d.Channels == nil {
		d.Channels = make(map[string]*Channel)
	}
	for _, ch := range channels {
		if ch == nil || ch.ID == "" {
			continue
		}
		ch.DeviceID = deviceID
		d.Channels[ch.ID] = ch
	}
}

func (r *Registry) List() []*Device {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Device, 0, len(r.devices))
	for _, d := range r.devices {
		cp := *d
		cp.Channels = make(map[string]*Channel, len(d.Channels))
		for k, v := range d.Channels {
			ch := *v
			cp.Channels[k] = &ch
		}
		out = append(out, &cp)
	}
	return out
}

func (r *Registry) Channel(deviceID, channelID string) (*Device, *Channel, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.devices[deviceID]
	if !ok {
		return nil, nil, false
	}
	ch, ok := d.Channels[channelID]
	if !ok {
		// some IPC use deviceID as sole channel
		if channelID == deviceID {
			ch = &Channel{ID: channelID, DeviceID: deviceID, Name: d.Name, Status: "ON"}
			return d, ch, true
		}
		return d, nil, false
	}
	cp := *ch
	return d, &cp, true
}

// Delete removes a device from the registry.
func (r *Registry) Delete(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.devices, id)
}

// MergeOnline upserts runtime fields while keeping Name/Password/Status from existing record when empty.
func (r *Registry) MergeOnline(d *Device) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if old, ok := r.devices[d.ID]; ok {
		if d.Name == "" {
			d.Name = old.Name
		}
		if d.Password == "" {
			d.Password = old.Password
		}
		if d.Status == "" {
			d.Status = old.Status
		}
		if d.Channels == nil {
			d.Channels = old.Channels
		}
	}
	if d.Status == "" {
		d.Status = DeviceStatusActive
	}
	if d.Channels == nil {
		d.Channels = make(map[string]*Channel)
	}
	r.devices[d.ID] = d
}

// SweepOffline marks devices offline when keepalive exceeds timeout.
// Returns device IDs that newly transitioned to offline.
func (r *Registry) SweepOffline(timeout time.Duration) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	var offline []string
	for _, d := range r.devices {
		if !d.Online {
			continue
		}
		last := d.LastKeepalive
		if last.IsZero() {
			last = d.Registered
		}
		if last.IsZero() {
			continue
		}
		if now.Sub(last) > timeout {
			d.Online = false
			offline = append(offline, d.ID)
		}
	}
	return offline
}
