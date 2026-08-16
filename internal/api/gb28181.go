package api

import (
	"errors"
	"github.com/liuhengloveyou/FrameFlow/pkg/fferrors"
	"net/http"
	"strings"
	"time"
)

// GB28181Backend 由 internal/gb28181 实现。
// 通过接口注入避免 api → gb28181 → streams → api 的循环依赖。
type GB28181Backend interface {
	Enabled() bool
	Status() map[string]any
	ListDevices() []GB28181Device
	GetDevice(id string) (GB28181Device, bool)
	AddDevice(id, name, password string) (GB28181Device, error)
	UpdateDevice(id, name string, password *string) (GB28181Device, error)
	DeleteDevice(id string) error
	ConfirmDevice(id string) (GB28181Device, error)
	RejectDevice(id string) (GB28181Device, error)
	ListChannels(deviceID string) ([]GB28181Channel, error)
	TriggerCatalog(deviceID string) error
	PTZ(deviceID, channelID, cmd string, speed byte) (string, error)
	QueryRecords(deviceID, channelID string, start, end time.Time) ([]GB28181RecordItem, error)
}

// GB28181Device 国标设备对外 DTO。
type GB28181Device struct {
	ID         string `json:"id"`
	Name       string `json:"name,omitempty"`
	Status     string `json:"status,omitempty"` // pending|active|rejected|disabled
	Online     bool   `json:"online"`
	RemoteAddr string `json:"remote_addr,omitempty"`
	Transport  string `json:"transport,omitempty"`
	Channels   int    `json:"channels"`
	StreamURL  string `json:"stream_url,omitempty"` // 建议拉流地址 gb28181://device/channel
}

// GB28181Channel 国标通道对外 DTO。
type GB28181Channel struct {
	ID           uint   `json:"id"`
	ChannelID    string `json:"channel_id"`
	DeviceID     string `json:"device_id"`
	Name         string `json:"name,omitempty"`
	Manufacturer string `json:"manufacturer,omitempty"`
	Model        string `json:"model,omitempty"`
	Status       string `json:"status,omitempty"`
	StreamURL    string `json:"stream_url,omitempty"`
}

// GB28181RecordItem 国标录像片段。
type GB28181RecordItem struct {
	DeviceID  string `json:"device_id,omitempty"`
	Name      string `json:"name,omitempty"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	Type      string `json:"type,omitempty"`
	FilePath  string `json:"file_path,omitempty"`
	FileSize  string `json:"file_size,omitempty"`
	StreamURL string `json:"stream_url,omitempty"`
}

var gb28181Backend GB28181Backend

// ProvideGB28181 注入国标业务后端（不挂路由）。由 internal/gb28181.Init 在 SIP/SQLite 就绪后调用。
func ProvideGB28181(b GB28181Backend) {
	gb28181Backend = b
}

// MountGB28181 集中注册国标协议面路由（显式 NeedLogin，不做 Casbin）。
// 设备 CRUD 走 /api/cameras；此处为平台状态、目录、通道、PTZ、录像查询。
// 成功体仍为裸 JSON（前端 nativeHttp 直接解 DTO）。
// 须在 gb28181.Init Provide 之后由 main 调用。
func MountGB28181() {
	if gb28181Backend == nil {
		log.Debug().Msg("[gb28181] skip mount http: backend nil")
		return
	}

	const login = true

	RegisterBizAPI("GET", "api/gb28181", login, "国标平台状态", handleGB28181Status)
	RegisterBizAPI("GET", "api/gb28181/devices", login, "国标设备列表", handleGB28181DevicesList)
	RegisterBizAPI("POST", "api/gb28181/devices", login, "国标预登记设备", handleGB28181DeviceAdd)
	RegisterBizAPI("GET", "api/gb28181/devices/{id}", login, "国标设备详情", handleGB28181DeviceGet)
	RegisterBizAPI("PUT", "api/gb28181/devices/{id}", login, "更新国标设备", handleGB28181DeviceUpdate)
	RegisterBizAPI("DELETE", "api/gb28181/devices/{id}", login, "删除国标设备", handleGB28181DeviceDelete)
	RegisterBizAPI("POST", "api/gb28181/devices/{id}/confirm", login, "确认国标设备", handleGB28181DeviceConfirm)
	RegisterBizAPI("POST", "api/gb28181/devices/{id}/reject", login, "拒绝国标设备", handleGB28181DeviceReject)
	RegisterBizAPI("POST", "api/gb28181/devices/{id}/catalog", login, "触发目录查询", handleGB28181DeviceCatalog)
	RegisterBizAPI("GET", "api/gb28181/devices/{id}/channels", login, "国标通道列表", handleGB28181ChannelsList)
	RegisterBizAPI("POST", "api/gb28181/devices/{id}/channels/{cid}/ptz", login, "云台控制", handleGB28181PTZ)
	RegisterBizAPI("GET", "api/gb28181/devices/{id}/channels/{cid}/records", login, "国标录像查询", handleGB28181Records)

	log.Info().Msg("[gb28181] http routes registered (/api/gb28181)")
}

func handleGB28181Status(w http.ResponseWriter, r *http.Request) {
	st := gb28181Backend.Status()
	log.Debug().Interface("status", st).Msg("[gb28181] status")
	ResponseJSON(w, st)
}

func handleGB28181DevicesList(w http.ResponseWriter, r *http.Request) {
	list := gb28181Backend.ListDevices()
	log.Debug().Int("count", len(list)).Msg("[gb28181] list devices")
	ResponseJSON(w, list)
}

func handleGB28181DeviceAdd(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		log.Warn().Err(err).Msg("[gb28181] add device: bad json")
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg(err.Error()))
		return
	}
	body.ID = strings.TrimSpace(body.ID)
	if body.ID == "" {
		log.Warn().Msg("[gb28181] add device: missing id")
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg("id required (20-digit GB28181 device id)"))
		return
	}
	log.Info().
		Str("device", body.ID).
		Str("name", body.Name).
		Msg("[gb28181] add device begin")
	d, err := gb28181Backend.AddDevice(body.ID, body.Name, body.Password)
	if err != nil {
		log.Warn().Err(err).Str("device", body.ID).Msg("[gb28181] add device failed")
		fferrors.RespondErr(w, err)
		return
	}
	log.Info().
		Str("device", d.ID).
		Str("name", d.Name).
		Str("stream_url", d.StreamURL).
		Msg("[gb28181] add device ok")
	ResponseJSON(w, d)
}

func handleGB28181DeviceGet(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("id")
	d, ok := gb28181Backend.GetDevice(deviceID)
	if !ok {
		log.Debug().Str("device", deviceID).Msg("[gb28181] get device: not found")
		fferrors.RespondBizError(w, fferrors.ErrNotFound.SetMsg("device not found"))
		return
	}
	log.Debug().Str("device", deviceID).Bool("online", d.Online).Msg("[gb28181] get device")
	ResponseJSON(w, d)
}

func handleGB28181DeviceUpdate(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("id")
	var body struct {
		Name     string  `json:"name"`
		Password *string `json:"password"` // nil=不改密码
	}
	if err := DecodeJSON(r, &body); err != nil {
		log.Warn().Err(err).Str("device", deviceID).Msg("[gb28181] update device: bad json")
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg(err.Error()))
		return
	}
	log.Info().
		Str("device", deviceID).
		Str("name", body.Name).
		Bool("set_password", body.Password != nil).
		Msg("[gb28181] update device begin")
	d, err := gb28181Backend.UpdateDevice(deviceID, body.Name, body.Password)
	if err != nil {
		log.Warn().Err(err).Str("device", deviceID).Msg("[gb28181] update device failed")
		fferrors.RespondErr(w, err)
		return
	}
	log.Info().Str("device", d.ID).Str("name", d.Name).Msg("[gb28181] update device ok")
	ResponseJSON(w, d)
}

func handleGB28181DeviceDelete(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("id")
	log.Info().Str("device", deviceID).Msg("[gb28181] delete device begin")
	if err := gb28181Backend.DeleteDevice(deviceID); err != nil {
		log.Warn().Err(err).Str("device", deviceID).Msg("[gb28181] delete device failed")
		fferrors.RespondBizError(w, fferrors.ErrNotFound.SetMsg(err.Error()))
		return
	}
	log.Info().Str("device", deviceID).Msg("[gb28181] delete device ok")
	ResponseJSON(w, map[string]string{"status": "ok"})
}

func handleGB28181DeviceConfirm(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("id")
	log.Info().Str("device", deviceID).Msg("[gb28181] confirm begin")
	d, err := gb28181Backend.ConfirmDevice(deviceID)
	if err != nil {
		log.Warn().Err(err).Str("device", deviceID).Msg("[gb28181] confirm failed")
		fferrors.RespondErr(w, err)
		return
	}
	log.Info().Str("device", d.ID).Str("status", d.Status).Msg("[gb28181] confirm ok")
	ResponseJSON(w, d)
}

func handleGB28181DeviceReject(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("id")
	log.Info().Str("device", deviceID).Msg("[gb28181] reject begin")
	d, err := gb28181Backend.RejectDevice(deviceID)
	if err != nil {
		log.Warn().Err(err).Str("device", deviceID).Msg("[gb28181] reject failed")
		fferrors.RespondErr(w, err)
		return
	}
	log.Info().Str("device", d.ID).Str("status", d.Status).Msg("[gb28181] reject ok")
	ResponseJSON(w, d)
}

func handleGB28181DeviceCatalog(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("id")
	log.Info().Str("device", deviceID).Msg("[gb28181] catalog begin")
	if err := gb28181Backend.TriggerCatalog(deviceID); err != nil {
		log.Warn().Err(err).Str("device", deviceID).Msg("[gb28181] catalog failed")
		fferrors.RespondErr(w, err)
		return
	}
	log.Info().Str("device", deviceID).Msg("[gb28181] catalog ok")
	ResponseJSON(w, map[string]string{"status": "ok"})
}

func handleGB28181ChannelsList(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("id")
	chs, err := gb28181Backend.ListChannels(deviceID)
	if err != nil {
		log.Debug().Err(err).Str("device", deviceID).Msg("[gb28181] list channels failed")
		fferrors.RespondBizError(w, fferrors.ErrNotFound.SetMsg(err.Error()))
		return
	}
	log.Debug().Str("device", deviceID).Int("count", len(chs)).Msg("[gb28181] list channels")
	ResponseJSON(w, chs)
}

func handleGB28181PTZ(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("id")
	channelID := r.PathValue("cid")
	var body struct {
		Cmd   string `json:"cmd"`   // left/right/up/down/zoomin/zoomout/stop
		Speed byte   `json:"speed"` // 1–255，0 表示默认
	}
	if err := DecodeJSON(r, &body); err != nil {
		log.Warn().Err(err).Str("device", deviceID).Msg("[gb28181] ptz: bad json")
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg(err.Error()))
		return
	}
	log.Info().
		Str("device", deviceID).
		Str("channel", channelID).
		Str("cmd", body.Cmd).
		Uint8("speed", body.Speed).
		Msg("[gb28181] ptz begin")
	ptz, err := gb28181Backend.PTZ(deviceID, channelID, body.Cmd, body.Speed)
	if err != nil {
		log.Warn().Err(err).
			Str("device", deviceID).
			Str("channel", channelID).
			Str("cmd", body.Cmd).
			Msg("[gb28181] ptz failed")
		fferrors.RespondErr(w, err)
		return
	}
	log.Info().
		Str("device", deviceID).
		Str("channel", channelID).
		Str("cmd", body.Cmd).
		Str("ptz", ptz).
		Msg("[gb28181] ptz ok")
	ResponseJSON(w, map[string]string{"status": "ok", "ptz": ptz})
}

func handleGB28181Records(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("id")
	channelID := r.PathValue("cid")
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")
	if startStr == "" || endStr == "" {
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg("start and end required (RFC3339 or 2006-01-02T15:04:05)"))
		return
	}
	start, err1 := parseAPITime(startStr)
	end, err2 := parseAPITime(endStr)
	if err1 != nil || err2 != nil || !end.After(start) {
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg("invalid start/end"))
		return
	}
	log.Info().
		Str("device", deviceID).
		Str("channel", channelID).
		Time("start", start).
		Time("end", end).
		Msg("[gb28181] records begin")
	items, err := gb28181Backend.QueryRecords(deviceID, channelID, start, end)
	if err != nil {
		log.Warn().Err(err).
			Str("device", deviceID).
			Str("channel", channelID).
			Msg("[gb28181] records failed")
		fferrors.RespondBizError(w, fferrors.ErrServer.SetMsg(err.Error()).SetHTTPStatus(http.StatusBadGateway))
		return
	}
	log.Info().
		Str("device", deviceID).
		Str("channel", channelID).
		Int("count", len(items)).
		Msg("[gb28181] records ok")
	ResponseJSON(w, items)
}

func parseAPITime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, errors.New("empty time")
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}
	var last error
	for _, layout := range layouts {
		var (
			t   time.Time
			err error
		)
		if layout == time.RFC3339Nano || layout == time.RFC3339 {
			t, err = time.Parse(layout, s)
		} else {
			t, err = time.ParseInLocation(layout, s, time.Local)
		}
		if err == nil {
			return t, nil
		}
		last = err
	}
	return time.Time{}, last
}
