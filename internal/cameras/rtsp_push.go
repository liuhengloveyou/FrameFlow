package cameras

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/liuhengloveyou/FrameFlow/internal/api"
	"github.com/liuhengloveyou/FrameFlow/internal/rtsp"
	"github.com/liuhengloveyou/FrameFlow/internal/streams"
)

func isRTSPPush(cam api.Camera) bool {
	return cam.Protocol == api.CameraRTSP && cam.IngestMode == api.CameraIngestPush
}

func isPushIngest(cam api.Camera) bool {
	return isRTMPPush(cam) || isRTSPPush(cam)
}

func rtspListenPort() int {
	return rtsp.ListenPortForPublish()
}

func buildRTSPPublishURL(host string, port int, streamKey string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		host = "127.0.0.1"
	}
	if port <= 0 {
		port = 8554
	}
	return fmt.Sprintf("rtsp://%s:%d/%s", host, port, streamKey)
}

func enrichRTSPPushFields(cam *api.Camera, _ string) {
	if !isRTSPPush(*cam) {
		return
	}
	if cam.StreamKey == "" && cam.ID != 0 {
		cam.StreamKey = CameraStreamName(*cam)
	}
	port := rtspListenPort()
	cam.Port = port
	host := rtsp.AdvertiseHostForPublish()
	cam.IP = host
	cam.AllowAnonymous = true
	cam.PushSecretHash = ""
	cam.PushSecret = ""
	cam.PublishURL = buildRTSPPublishURL(host, port, cam.StreamKey)
}

func generateRTSPStreamKey() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "ingest_" + hex.EncodeToString(b), nil
}

func setupRTSPPushSlot(cam *api.Camera, _ api.CameraInput, _ *api.Camera) (revealSecret string, err error) {
	key := strings.TrimSpace(cam.StreamKey)
	if key == "" && cam.ID != 0 {
		key = CameraStreamName(*cam)
	}
	if key == "" {
		return "", fmt.Errorf("stream key required")
	}
	cam.StreamKey = key

	port := rtspListenPort()
	cam.Port = port
	cam.IP = rtsp.AdvertiseHostForPublish()
	cam.AllowAnonymous = true
	cam.PushSecretHash = ""

	if _, err = streams.EnsureRTSPIngestSlot(key, "", ""); err != nil {
		return "", err
	}

	cam.StreamURL = "rtsp-ingest://" + key
	enrichRTSPPushFields(cam, "")
	return "", nil
}

func teardownRTSPPushSlot(cam api.Camera) {
	key := CameraStreamName(cam)
	streams.DeleteRTSPIngestSlot(key)
	streams.Delete(key)
}

// enrichPushFields fills publish URL fields for RTMP or RTSP push cameras.
func enrichPushFields(cam *api.Camera, reveal string) {
	enrichRTMPPushFields(cam, reveal)
	enrichRTSPPushFields(cam, reveal)
	if !isPushIngest(*cam) {
		return
	}
	key := CameraStreamName(*cam)
	if streams.HasLiveIngest(key) {
		cam.Status = "online"
		cam.Online = true
	} else {
		// 推流设备在线态以当前是否有推入为准，不沿用库内旧值
		cam.Status = "offline"
		cam.Online = false
	}
}
