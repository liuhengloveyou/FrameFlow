package api

import (
	"github.com/liuhengloveyou/FrameFlow/pkg/fferrors"
	"net/http"
	"strconv"
	"strings"
)

// CamerasBackend 多协议摄像头统一业务后端（由 internal/cameras 实现）。
type CamerasBackend interface {
	List(q CameraQuery) CameraPage
	Get(id string) (Camera, bool)
	Create(in CameraInput) (Camera, error)
	Update(id string, in CameraInput) (Camera, error)
	Delete(id string) error
	Start(id string) (Camera, error)
	Stop(id string) (Camera, error)
	TestStream(id string) (StreamTestResult, error)
}

// CameraProtocol 摄像头接入协议。
type CameraProtocol string

const (
	CameraRTSP    CameraProtocol = "rtsp"
	CameraRTMP    CameraProtocol = "rtmp"
	CameraONVIF   CameraProtocol = "onvif"
	CameraHLS     CameraProtocol = "hls"
	CameraGB28181 CameraProtocol = "gb28181"
)

// CameraIngestMode 流接入方向：平台拉远端（pull）或设备推到平台（push）。
type CameraIngestMode string

const (
	CameraIngestPull CameraIngestMode = "pull"
	CameraIngestPush CameraIngestMode = "push"
)

// Camera 统一设备 DTO（各协议共用；国标字段平铺，低频进 ProtocolMeta）。
// 一机多流：行表示接入设备；Channels 为可播放通道子表（Catalog / 单流通用）。
type Camera struct {
	ID         uint           `json:"id"`
	Name       string         `json:"name"`
	IP         string         `json:"ip"`
	Port       int            `json:"port"`
	Vendor     string         `json:"vendor"`
	Group      string         `json:"group"`
	Resolution string         `json:"resolution"`
	FPS        int            `json:"fps"`
	Status     string         `json:"status"` // online|offline|error|running|stopped
	Protocol   CameraProtocol `json:"protocol"`
	StreamURL  string         `json:"streamUrl,omitempty"`
	Username   string         `json:"username,omitempty"`
	Password   string         `json:"password,omitempty"`
	Path       string         `json:"path,omitempty"`
	Channel    string         `json:"channel,omitempty"` // 默认通道（主预览）
	DeviceID   string         `json:"deviceId,omitempty"`
	UpdatedAt  string         `json:"updatedAt"`
	CreatedAt  string         `json:"createdAt,omitempty"`

	// RTMP / RTSP 推流接入（ingestMode=push）
	IngestMode     CameraIngestMode `json:"ingestMode,omitempty"`     // pull|push；默认 pull
	StreamKey      string           `json:"streamKey,omitempty"`      // 推流槽位名（系统随机生成）
	PublishURL     string           `json:"publishUrl,omitempty"`     // 展示用推流地址（不含密钥）
	PushTokenQuery string           `json:"pushTokenQuery,omitempty"` // 默认 push_key
	PushSecret     string           `json:"pushSecret,omitempty"`     // 仅创建/重置时返回明文
	AllowAnonymous bool             `json:"allowAnonymous,omitempty"`

	// 内部：推流密钥 sha256 hex，不对外返回
	PushSecretHash string `json:"-"`

	// 国标 / 协议扩展（平铺）
	Enrollment    string `json:"enrollment,omitempty"` // pending|active|rejected|disabled
	Online        bool   `json:"online,omitempty"`
	Transport     string `json:"transport,omitempty"`
	RemoteAddr    string `json:"remoteAddr,omitempty"`
	ContactURI    string `json:"contactUri,omitempty"`
	RegisteredAt  string `json:"registeredAt,omitempty"`
	LastKeepalive string `json:"lastKeepalive,omitempty"`

	// JSON 补充
	ProtocolMeta string `json:"protocolMeta,omitempty"`

	// 通道子表（List/Get 填充；非国标可为单条合成通道）
	Channels     []CameraChannel `json:"channels,omitempty"`
	ChannelCount int             `json:"channelCount,omitempty"`
}

// CameraChannel 设备下的可播放通道（国标 Catalog 项 / 单机单流）。
type CameraChannel struct {
	ID           uint   `json:"id"`
	ChannelID    string `json:"channelId"` // 国标通道编码；非国标为逻辑通道号
	DeviceID     string `json:"deviceId"`
	Name         string `json:"name,omitempty"`
	Manufacturer string `json:"manufacturer,omitempty"`
	Model        string `json:"model,omitempty"`
	Status       string `json:"status,omitempty"`
	StreamURL    string `json:"streamUrl,omitempty"`
	IsDefault    bool   `json:"isDefault,omitempty"`
}

// CameraInput 创建/更新入参。
type CameraInput struct {
	Name       string         `json:"name"`
	IP         string         `json:"ip"`
	Port       int            `json:"port"`
	Vendor     string         `json:"vendor"`
	Group      string         `json:"group"`
	Resolution string         `json:"resolution"`
	FPS        int            `json:"fps"`
	Protocol   CameraProtocol `json:"protocol"`
	StreamURL  string         `json:"streamUrl,omitempty"`
	Username   string         `json:"username,omitempty"`
	Password   string         `json:"password,omitempty"`
	Path       string         `json:"path,omitempty"`
	Channel    string         `json:"channel,omitempty"`
	DeviceID   string         `json:"deviceId,omitempty"`

	// RTMP 推流
	IngestMode     CameraIngestMode `json:"ingestMode,omitempty"`
	StreamKey      string           `json:"streamKey,omitempty"`
	AllowAnonymous bool             `json:"allowAnonymous,omitempty"`
	PushSecret     string           `json:"pushSecret,omitempty"`     // 可选；空且非匿名则服务端生成
	ResetPushSecret bool            `json:"resetPushSecret,omitempty"` // 更新时重新生成密钥
	AdvertiseHost  string           `json:"advertiseHost,omitempty"`  // 可选；推流 URL 展示主机，默认由客户端用访问主机填入

	// RTSP 拉流：RTP 传输 tcp|udp（默认 tcp）
	Transport string `json:"transport,omitempty"`
}

// CameraQuery 列表筛选。
type CameraQuery struct {
	Page     int
	PageSize int
	Name     string
	Status   string
	Group    string
	Protocol string
}

// CameraPage 分页结果。
type CameraPage struct {
	Items    []Camera `json:"items"`
	Total    int      `json:"total"`
	Page     int      `json:"page"`
	PageSize int      `json:"pageSize"`
}

// StreamTestResult 测流结果。
type StreamTestResult struct {
	OK         bool   `json:"ok"`
	LatencyMs  int    `json:"latencyMs"`
	Message    string `json:"message"`
	Codec      string `json:"codec,omitempty"`
	Resolution string `json:"resolution,omitempty"`
}

var camerasBackend CamerasBackend

// ProvideCameras 注入统一摄像头后端（不挂路由）。
func ProvideCameras(b CamerasBackend) {
	camerasBackend = b
}

// MountCameras 集中注册摄像头业务路由（显式 NeedLogin，不做 Casbin）。
// 成功体仍为裸 JSON（前端 nativeHttp 直接解 DTO，非 {code,data} 信封）。
// 须在 cameras.Init Provide 之后由 main 调用。
func MountCameras() {
	if camerasBackend == nil {
		log.Debug().Msg("[cameras] skip mount http: backend nil")
		return
	}

	const login = true

	RegisterBizAPI("GET", "api/cameras", login, "摄像头分页列表", handleCamerasList)
	RegisterBizAPI("POST", "api/cameras", login, "新增摄像头", handleCameraCreate)
	RegisterBizAPI("GET", "api/cameras/{id}", login, "摄像头详情", handleCameraGet)
	RegisterBizAPI("PUT", "api/cameras/{id}", login, "更新摄像头", handleCameraUpdate)
	RegisterBizAPI("DELETE", "api/cameras/{id}", login, "删除摄像头", handleCameraDelete)
	RegisterBizAPI("GET", "api/cameras/{id}/channels", login, "摄像头通道列表", handleCameraChannels)
	RegisterBizAPI("POST", "api/cameras/{id}/start", login, "启动摄像头", handleCameraStart)
	RegisterBizAPI("POST", "api/cameras/{id}/stop", login, "停止摄像头", handleCameraStop)
	RegisterBizAPI("POST", "api/cameras/{id}/test-stream", login, "测流", handleCameraTestStream)

	log.Info().Msg("[cameras] http routes registered (/api/cameras)")
}

func handleCamerasList(w http.ResponseWriter, r *http.Request) {
	ResponseJSON(w, camerasBackend.List(parseCameraQuery(r)))
}

func handleCameraCreate(w http.ResponseWriter, r *http.Request) {
	var in CameraInput
	if err := DecodeJSON(r, &in); err != nil {
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg(err.Error()))
		return
	}
	cam, err := camerasBackend.Create(in)
	if err != nil {
		fferrors.RespondErr(w, err)
		return
	}
	ResponseJSON(w, cam)
}

func handleCameraGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cam, ok := camerasBackend.Get(id)
	if !ok {
		fferrors.RespondBizError(w, fferrors.ErrNotFound.SetMsg("camera not found"))
		return
	}
	ResponseJSON(w, cam)
}

func handleCameraUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in CameraInput
	if err := DecodeJSON(r, &in); err != nil {
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg(err.Error()))
		return
	}
	cam, err := camerasBackend.Update(id, in)
	if err != nil {
		fferrors.RespondErr(w, err)
		return
	}
	ResponseJSON(w, cam)
}

func handleCameraDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := camerasBackend.Delete(id); err != nil {
		fferrors.RespondBizError(w, fferrors.ErrNotFound.SetMsg(err.Error()))
		return
	}
	ResponseJSON(w, map[string]string{"status": "ok"})
}

func handleCameraChannels(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cam, ok := camerasBackend.Get(id)
	if !ok {
		fferrors.RespondBizError(w, fferrors.ErrNotFound.SetMsg("camera not found"))
		return
	}
	chs := cam.Channels
	if chs == nil {
		chs = []CameraChannel{}
	}
	ResponseJSON(w, chs)
}

func handleCameraStart(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cam, err := camerasBackend.Start(id)
	if err != nil {
		fferrors.RespondErr(w, err)
		return
	}
	ResponseJSON(w, cam)
}

func handleCameraStop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cam, err := camerasBackend.Stop(id)
	if err != nil {
		fferrors.RespondErr(w, err)
		return
	}
	ResponseJSON(w, cam)
}

func handleCameraTestStream(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	res, err := camerasBackend.TestStream(id)
	if err != nil {
		fferrors.RespondErr(w, err)
		return
	}
	ResponseJSON(w, res)
}

func parseCameraQuery(r *http.Request) CameraQuery {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(q.Get("pageSize"))
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return CameraQuery{
		Page:     page,
		PageSize: pageSize,
		Name:     strings.TrimSpace(q.Get("name")),
		Status:   strings.TrimSpace(q.Get("status")),
		Group:    strings.TrimSpace(q.Get("group")),
		Protocol: strings.TrimSpace(q.Get("protocol")),
	}
}
