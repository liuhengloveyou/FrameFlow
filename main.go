package main

import (
	"slices"

	"github.com/liuhengloveyou/FrameFlow/internal/aibiz"
	"github.com/liuhengloveyou/FrameFlow/internal/alsa"
	"github.com/liuhengloveyou/FrameFlow/internal/api"
	"github.com/liuhengloveyou/FrameFlow/internal/api/ws"
	"github.com/liuhengloveyou/FrameFlow/internal/app"
	"github.com/liuhengloveyou/FrameFlow/internal/bubble"
	"github.com/liuhengloveyou/FrameFlow/internal/cameras"
	"github.com/liuhengloveyou/FrameFlow/internal/debug"
	"github.com/liuhengloveyou/FrameFlow/internal/doorbird"
	"github.com/liuhengloveyou/FrameFlow/internal/dvrip"
	"github.com/liuhengloveyou/FrameFlow/internal/echo"
	"github.com/liuhengloveyou/FrameFlow/internal/eseecloud"
	"github.com/liuhengloveyou/FrameFlow/internal/exec"
	"github.com/liuhengloveyou/FrameFlow/internal/expr"
	"github.com/liuhengloveyou/FrameFlow/internal/ffmpeg"
	"github.com/liuhengloveyou/FrameFlow/internal/flussonic"
	"github.com/liuhengloveyou/FrameFlow/internal/gb28181"
	"github.com/liuhengloveyou/FrameFlow/internal/gopro"
	"github.com/liuhengloveyou/FrameFlow/internal/hass"
	"github.com/liuhengloveyou/FrameFlow/internal/hls"
	"github.com/liuhengloveyou/FrameFlow/internal/homekit"
	"github.com/liuhengloveyou/FrameFlow/internal/http"
	"github.com/liuhengloveyou/FrameFlow/internal/isapi"
	"github.com/liuhengloveyou/FrameFlow/internal/ivideon"
	"github.com/liuhengloveyou/FrameFlow/internal/kasa"
	"github.com/liuhengloveyou/FrameFlow/internal/mjpeg"
	"github.com/liuhengloveyou/FrameFlow/internal/mp4"
	"github.com/liuhengloveyou/FrameFlow/internal/mpeg"
	"github.com/liuhengloveyou/FrameFlow/internal/multitrans"
	"github.com/liuhengloveyou/FrameFlow/internal/nest"
	"github.com/liuhengloveyou/FrameFlow/internal/ngrok"
	"github.com/liuhengloveyou/FrameFlow/internal/onvif"
	"github.com/liuhengloveyou/FrameFlow/internal/pinggy"
	"github.com/liuhengloveyou/FrameFlow/internal/ring"
	"github.com/liuhengloveyou/FrameFlow/internal/roborock"
	"github.com/liuhengloveyou/FrameFlow/internal/rtmp"
	"github.com/liuhengloveyou/FrameFlow/internal/rtsp"
	"github.com/liuhengloveyou/FrameFlow/internal/srtp"
	"github.com/liuhengloveyou/FrameFlow/internal/streams"
	"github.com/liuhengloveyou/FrameFlow/internal/tapo"
	"github.com/liuhengloveyou/FrameFlow/internal/tuya"
	"github.com/liuhengloveyou/FrameFlow/internal/v4l2"
	"github.com/liuhengloveyou/FrameFlow/internal/vision"
	"github.com/liuhengloveyou/FrameFlow/internal/webrtc"
	"github.com/liuhengloveyou/FrameFlow/internal/webtorrent"
	"github.com/liuhengloveyou/FrameFlow/internal/wyoming"
	"github.com/liuhengloveyou/FrameFlow/internal/wyze"
	"github.com/liuhengloveyou/FrameFlow/internal/xiaomi"
	"github.com/liuhengloveyou/FrameFlow/internal/yandex"
	"github.com/liuhengloveyou/FrameFlow/pkg/shell"
)

func main() {
	// version will be set later from -buildvcs info, this used only as fallback
	app.Version = "1.9.14"

	// 配置与日志、打开数据库
	app.Init()

	type module struct {
		name string
		init func()
	}

	modules := []module{

		{"", initPassport},      // passport：表结构 / root 种子 /usercenter（须在 api.Init 前）
		{"api", api.Init},       // 内置 HTTP：api、api/config、api/exit、api/restart、api/log；api/webrtc 为调度入口
		{"ws", ws.Init},         // API：api/ws（WebSocket）
		{"", streams.Init},      // API：api/streams、api/streams.dot、api/devices/rtmp、api/preload、api/schemes
		{"vision", vision.Init}, // streams 附属数据：解析 → BroadcastStreamData；SetStreamDataHooks 启停 worker
		// Main sources and servers
		{"http", http.Init},     // API：api/stream；HTTP 拉流源
		{"rtsp", rtsp.Init},     // RTSP 服务
		{"webrtc", webrtc.Init}, // 向 api 注册 api/webrtc 处理逻辑；ICE 监听见 webrtc.listen
		// Main API
		{"mp4", mp4.Init},     // API：api/frame.mp4、api/stream.mp4
		{"hls", hls.Init},     // API：api/stream.m3u8、api/hls/*
		{"mjpeg", mjpeg.Init}, // API：api/frame.jpeg、api/stream.mjpeg、api/stream.ascii、api/stream.y4m
		// Other sources and servers
		{"hass", hass.Init},             // API：api/hass、/static、/streams、/stream/…
		{"homekit", homekit.Init},       // API：api/homekit、api/homekit/accessories、api/discovery/homekit；HAP Pair-Setup/Verify
		{"onvif", onvif.Init},           // API：api/onvif、/onvif/…
		{"cameras", cameras.Init},       // 多协议摄像头统一库；HTTP：api/cameras（先于 gb28181 注册 presence hook）
		{"aibiz", aibiz.Init},           // AI 业务 API：模型 / 算法 / 工作流；HTTP：api/v1/*
		{"gb28181", gb28181.Init},       // SIP + sqlite；协议面 HTTP：api/gb28181
		{"rtmp", rtmp.Init},             // API：api/stream.flv；RTMP 推/拉流
		{"webtorrent", webtorrent.Init}, // API：api/webtorrent
		{"wyoming", wyoming.Init},
		// Exec and script sources
		{"echo", echo.Init},
		{"exec", exec.Init},
		{"expr", expr.Init},
		{"ffmpeg", ffmpeg.Init}, // API：api/ffmpeg、api/ffmpeg/devices、api/ffmpeg/hardware
		// Hardware sources
		{"alsa", alsa.Init}, // API：api/alsa（linux）
		{"v4l2", v4l2.Init}, // API：api/v4l2（linux）
		// Other sources
		{"bubble", bubble.Init},
		{"doorbird", doorbird.Init},
		{"dvrip", dvrip.Init}, // API：api/dvrip
		{"eseecloud", eseecloud.Init},
		{"flussonic", flussonic.Init},
		{"gopro", gopro.Init}, // API：api/gopro
		{"isapi", isapi.Init},
		{"ivideon", ivideon.Init},
		{"kasa", kasa.Init},
		{"mpegts", mpeg.Init}, // API：api/stream.ts、api/stream.aac
		{"multitrans", multitrans.Init},
		{"nest", nest.Init},         // API：api/nest
		{"ring", ring.Init},         // API：api/ring
		{"roborock", roborock.Init}, // API：api/roborock
		{"tapo", tapo.Init},
		{"tuya", tuya.Init},     // API：api/tuya
		{"wyze", wyze.Init},     // API：api/wyze
		{"xiaomi", xiaomi.Init}, // API：api/xiaomi
		{"yandex", yandex.Init},
		// Helper modules
		{"debug", debug.Init}, // API：api/stack
		{"ngrok", ngrok.Init},
		{"pinggy", pinggy.Init},
		{"srtp", srtp.Init},
	}

	for _, m := range modules {
		if app.Modules == nil || m.name == "" || slices.Contains(app.Modules, m.name) {
			m.init()
		}
	}

	// 业务 HTTP：Provide* 集中注入后再挂载路由
	api.WireProtocols(rtmp.SettingsBackend(), rtmp.StreamFLV, rtsp.SettingsBackend(), onvif.SettingsBackend())
	api.MountCameras()
	api.MountGB28181()
	api.MountGB28181Settings()
	api.MountAIBizAPIs()
	api.MountRTMPSettings()
	api.MountRTSPSettings()
	api.MountONVIFSettings()

	shell.RunUntilSignal()
}
