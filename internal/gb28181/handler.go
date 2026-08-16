package gb28181

import (
	"time"

	"github.com/liuhengloveyou/FrameFlow/internal/app"
	"github.com/liuhengloveyou/FrameFlow/pkg/gb28181"
)

// storeHandler 实现 pkg/gb28181.Handler，把 SIP 协议事件落到 cameras 表。
type storeHandler struct{}

var _ gb28181.Handler = storeHandler{}

func (storeHandler) LookupDevice(deviceID string) (gb28181.DeviceAuth, error) {
	st := DefaultStore()
	if st == nil {
		return gb28181.DeviceAuth{}, nil
	}
	auth, ok, err := st.LookupAuth(deviceID)
	if err != nil || !ok {
		return gb28181.DeviceAuth{}, err
	}
	return auth, nil
}

func (storeHandler) OnDiscover(deviceID, remote string) (gb28181.DeviceAuth, error) {
	st := DefaultStore()
	if st == nil {
		return gb28181.DeviceAuth{Known: true, Status: gb28181.DeviceStatusPending}, nil
	}
	forceActive := gw != nil && gw.Config.AllowUnknown
	auth, err := st.DiscoverDevice(deviceID, forceActive, remote)
	if err != nil {
		return gb28181.DeviceAuth{}, err
	}
	if d, ok, _ := st.GetDevice(deviceID); ok && gw != nil {
		gw.Registry.UpsertDevice(d)
	}
	app.Logger.Info().
		Str("device", deviceID).
		Str("remote", remote).
		Str("status", auth.Status).
		Msg("[gb28181] persist: discovered device")
	return auth, nil
}

func (storeHandler) OnRegistered(dev *gb28181.Device) {
	if st := DefaultStore(); st != nil {
		if err := st.MarkOnline(dev); err != nil {
			app.Logger.Warn().Err(err).Str("device", dev.ID).Msg("[gb28181] persist: mark online failed")
		}
	}
	// REGISTER 成功后拉目录（主路径）；force 跳过 debounce
	ScheduleCatalog(dev.ID, 300*time.Millisecond, true)
}

func (storeHandler) OnUnregistered(deviceID string) {
	st := DefaultStore()
	if st == nil {
		return
	}
	if err := st.MarkOffline(deviceID); err != nil {
		app.Logger.Warn().Err(err).Str("device", deviceID).Msg("[gb28181] persist: mark offline failed")
	}
}

// OnKeepalive：未知设备先 Discover 入库；已拒绝/停用则返回 false（SIP 404，促使重新 REGISTER）。
func (storeHandler) OnKeepalive(deviceID, remote string) bool {
	st := DefaultStore()
	if st == nil {
		return false
	}

	auth, ok, err := st.LookupAuth(deviceID)
	if err != nil {
		app.Logger.Warn().Err(err).Str("device", deviceID).Msg("[gb28181] persist: keepalive lookup failed")
		return false
	}
	if !ok {
		auth, err = st.DiscoverDevice(deviceID, gw != nil && gw.Config.AllowUnknown, remote)
		if err != nil {
			app.Logger.Warn().Err(err).Str("device", deviceID).Msg("[gb28181] persist: keepalive discover failed")
			return false
		}
		if d, found, _ := st.GetDevice(deviceID); found && gw != nil {
			gw.Registry.UpsertDevice(d)
		}
		app.Logger.Info().
			Str("device", deviceID).
			Str("remote", remote).
			Str("status", auth.Status).
			Msg("[gb28181] persist: discovered via keepalive")
	}

	if !gb28181.AllowsRegister(auth.Status) {
		app.Logger.Warn().
			Str("device", deviceID).
			Str("status", auth.Status).
			Msg("[gb28181] keepalive: blocked by enrollment")
		return false
	}

	if err := st.TouchKeepalive(deviceID); err != nil {
		app.Logger.Warn().Err(err).Str("device", deviceID).Msg("[gb28181] persist: keepalive failed")
		return false
	}

	if err := EnsureDeviceReachable(deviceID, remote); err != nil {
		app.Logger.Warn().Err(err).Str("device", deviceID).Msg("[gb28181] keepalive: registry update failed")
	}

	// 尚无真实通道时补拉目录（辅路径，受 debounce 约束）
	if NeedsCatalog(deviceID) {
		go func(id string) {
			if err := RequestCatalog(id, false); err != nil {
				app.Logger.Warn().Err(err).Str("device", id).Msg("[gb28181] keepalive: catalog query failed")
			}
		}(deviceID)
	}
	return true
}

func (storeHandler) OnCatalog(deviceID string, channels []*gb28181.Channel) {
	st := DefaultStore()
	if st == nil {
		return
	}
	if err := st.SaveChannels(deviceID, channels); err != nil {
		app.Logger.Warn().Err(err).Str("device", deviceID).Msg("[gb28181] persist: save channels failed")
		return
	}
	primary := ""
	for _, ch := range channels {
		if ch != nil && ch.ID != "" && ch.ID != deviceID {
			primary = ch.ID
			break
		}
	}
	if primary == "" && len(channels) > 0 && channels[0] != nil {
		primary = channels[0].ID
	}
	app.Logger.Info().
		Str("device", deviceID).
		Int("channels", len(channels)).
		Str("primary", primary).
		Msg("[gb28181] catalog: channels saved")
}
