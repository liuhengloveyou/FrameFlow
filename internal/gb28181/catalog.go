package gb28181

import (
	"fmt"
	"sync"
	"time"

	"github.com/liuhengloveyou/FrameFlow/internal/app"
	"github.com/liuhengloveyou/FrameFlow/pkg/gb28181"
)

const catalogDebounce = 45 * time.Second

var (
	catalogMu       sync.Mutex
	catalogLastSent = map[string]time.Time{}
)

// EnsureDeviceReachable marks the device online in the registry and sets RemoteAddr
// so outbound Catalog/MESSAGE can resolve a peer (Contact or host:port).
func EnsureDeviceReachable(deviceID, remote string) error {
	if gw == nil {
		return fmt.Errorf("gb28181 disabled")
	}
	deviceID = trimID(deviceID)
	if deviceID == "" {
		return fmt.Errorf("device id required")
	}
	if d, ok := gw.Registry.GetMutable(deviceID); ok {
		d.Online = true
		if remote != "" {
			d.RemoteAddr = remote
		}
		return nil
	}
	d := &gb28181.Device{ID: deviceID, Online: true, RemoteAddr: remote}
	if st := DefaultStore(); st != nil {
		if stored, ok, _ := st.GetDevice(deviceID); ok && stored != nil {
			d.Name = stored.Name
			d.Password = stored.Password
			d.Status = stored.Status
			d.ContactURI = stored.ContactURI
			d.Transport = stored.Transport
			if d.RemoteAddr == "" {
				d.RemoteAddr = stored.RemoteAddr
			}
			d.Channels = stored.Channels
		}
	}
	gw.Registry.UpsertDevice(d)
	return nil
}

// NeedsCatalog reports whether the platform still lacks a real channel for deviceID.
func NeedsCatalog(deviceID string) bool {
	st := DefaultStore()
	if st == nil {
		return true
	}
	return st.NeedsCatalog(deviceID)
}

// RequestCatalog sends a Catalog query if the device is reachable.
// force=true skips debounce (manual / confirm / register).
func RequestCatalog(deviceID string, force bool) error {
	deviceID = trimID(deviceID)
	if deviceID == "" {
		return fmt.Errorf("device id required")
	}
	if endpoint == nil {
		return fmt.Errorf("gb28181 disabled")
	}
	if !force {
		catalogMu.Lock()
		last := catalogLastSent[deviceID]
		if time.Since(last) < catalogDebounce {
			catalogMu.Unlock()
			return nil
		}
		catalogLastSent[deviceID] = time.Now()
		catalogMu.Unlock()
	} else {
		catalogMu.Lock()
		catalogLastSent[deviceID] = time.Now()
		catalogMu.Unlock()
	}

	if err := endpoint.SendCatalog(deviceID); err != nil {
		return err
	}
	app.Logger.Info().Str("device", deviceID).Bool("force", force).Msg("[gb28181] catalog: query sent")
	return nil
}

// ScheduleCatalog sends Catalog asynchronously after a short delay (post-REGISTER).
func ScheduleCatalog(deviceID string, delay time.Duration, force bool) {
	deviceID = trimID(deviceID)
	if deviceID == "" || endpoint == nil {
		return
	}
	go func() {
		if delay > 0 {
			time.Sleep(delay)
		}
		if err := RequestCatalog(deviceID, force); err != nil {
			app.Logger.Warn().Err(err).Str("device", deviceID).Msg("[gb28181] catalog: query failed")
		}
	}()
}

func trimID(id string) string {
	for len(id) > 0 && (id[0] == ' ' || id[0] == '\t') {
		id = id[1:]
	}
	for len(id) > 0 && (id[len(id)-1] == ' ' || id[len(id)-1] == '\t') {
		id = id[:len(id)-1]
	}
	return id
}
