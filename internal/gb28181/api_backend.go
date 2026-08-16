package gb28181

import (
	"fmt"
	"time"

	"github.com/liuhengloveyou/FrameFlow/internal/api"
	"github.com/liuhengloveyou/FrameFlow/pkg/gb28181"
)

// apiBackend 将本包状态适配为 api.GB28181Backend（HTTP 挂载由 api.MountGB28181 完成）。
type apiBackend struct{}

func (apiBackend) Enabled() bool { return Enabled() }

func (apiBackend) Status() map[string]any { return Status() }

func (apiBackend) ListDevices() []api.GB28181Device {
	list := ListDevices()
	out := make([]api.GB28181Device, 0, len(list))
	for _, d := range list {
		out = append(out, toAPIDevice(d))
	}
	return out
}

func (apiBackend) GetDevice(id string) (api.GB28181Device, bool) {
	d, ok := GetDevice(id)
	if !ok {
		return api.GB28181Device{}, false
	}
	return toAPIDevice(d), true
}

func (apiBackend) AddDevice(id, name, password string) (api.GB28181Device, error) {
	d, err := AddDevice(id, name, password)
	if err != nil {
		return api.GB28181Device{}, err
	}
	return toAPIDevice(d), nil
}

func (apiBackend) UpdateDevice(id, name string, password *string) (api.GB28181Device, error) {
	d, err := UpdateDevice(id, name, password)
	if err != nil {
		return api.GB28181Device{}, err
	}
	return toAPIDevice(d), nil
}

func (apiBackend) DeleteDevice(id string) error { return DeleteDevice(id) }

func (apiBackend) ConfirmDevice(id string) (api.GB28181Device, error) {
	d, err := ConfirmDevice(id)
	if err != nil {
		return api.GB28181Device{}, err
	}
	return toAPIDevice(d), nil
}

func (apiBackend) RejectDevice(id string) (api.GB28181Device, error) {
	d, err := RejectDevice(id)
	if err != nil {
		return api.GB28181Device{}, err
	}
	return toAPIDevice(d), nil
}

func (apiBackend) ListChannels(deviceID string) ([]api.GB28181Channel, error) {
	if st := DefaultStore(); st != nil {
		rows, err := st.ListChannelRows(deviceID)
		if err != nil {
			return nil, err
		}
		out := make([]api.GB28181Channel, 0, len(rows))
		for _, r := range rows {
			out = append(out, api.GB28181Channel{
				ID: r.ID, ChannelID: r.ChannelID, DeviceID: r.DeviceID,
				Name: r.Name, Manufacturer: r.Manufacturer, Model: r.DeviceModel, Status: r.Status,
				StreamURL: "gb28181://" + deviceID + "/" + r.ChannelID,
			})
		}
		return out, nil
	}
	chs, err := ListChannels(deviceID)
	if err != nil {
		return nil, err
	}
	out := make([]api.GB28181Channel, 0, len(chs))
	for _, c := range chs {
		out = append(out, api.GB28181Channel{
			ChannelID: c.ID, DeviceID: c.DeviceID, Name: c.Name,
			Manufacturer: c.Manufacturer, Model: c.Model, Status: c.Status,
			StreamURL: "gb28181://" + deviceID + "/" + c.ID,
		})
	}
	return out, nil
}

func (apiBackend) TriggerCatalog(deviceID string) error { return TriggerCatalog(deviceID) }

func (apiBackend) PTZ(deviceID, channelID, cmd string, speed byte) (string, error) {
	return PTZ(deviceID, channelID, cmd, speed)
}

func (apiBackend) QueryRecords(deviceID, channelID string, start, end time.Time) ([]api.GB28181RecordItem, error) {
	items, err := QueryRecords(deviceID, channelID, start, end)
	if err != nil {
		return nil, err
	}
	out := make([]api.GB28181RecordItem, 0, len(items))
	for _, it := range items {
		st, e1 := gb28181.ParseRecordTime(it.StartTime)
		et, e2 := gb28181.ParseRecordTime(it.EndTime)
		streamURL := ""
		if e1 == nil && e2 == nil {
			streamURL = fmt.Sprintf(
				"gb28181://%s/%s?start=%d&end=%d",
				deviceID,
				channelID,
				st.Unix(),
				et.Unix(),
			)
		}
		out = append(out, api.GB28181RecordItem{
			DeviceID:  it.DeviceID,
			Name:      it.Name,
			StartTime: it.StartTime,
			EndTime:   it.EndTime,
			Type:      it.Type,
			FilePath:  it.FilePath,
			FileSize:  it.FileSize,
			StreamURL: streamURL,
		})
	}
	return out, nil
}

func toAPIDevice(d *gb28181.Device) api.GB28181Device {
	if d == nil {
		return api.GB28181Device{}
	}
	dto := api.GB28181Device{
		ID: d.ID, Name: d.Name, Status: d.Status, Online: d.Online,
		RemoteAddr: d.RemoteAddr, Transport: d.Transport,
		Channels: len(d.Channels),
	}
	chID := d.ID
	for id := range d.Channels {
		chID = id
		if id != d.ID {
			break
		}
	}
	dto.StreamURL = "gb28181://" + d.ID + "/" + chID
	return dto
}

// provideHTTPBackend 仅向 api 注入后端，不挂载路由。
func provideHTTPBackend() {
	api.ProvideGB28181(apiBackend{})
}
