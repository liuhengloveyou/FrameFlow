package cameras

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/liuhengloveyou/FrameFlow/internal/api"
	"github.com/liuhengloveyou/FrameFlow/internal/rtmp"
	"github.com/liuhengloveyou/FrameFlow/internal/streams"
)

func isRTMPPush(cam api.Camera) bool {
	return cam.Protocol == api.CameraRTMP && cam.IngestMode == api.CameraIngestPush
}

func rtmpListenPort() int {
	return rtmp.ListenPortForPublish()
}

func buildPublishURL(host string, port int, streamKey string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		host = "127.0.0.1"
	}
	if port <= 0 {
		port = 1935
	}
	return fmt.Sprintf("rtmp://%s:%d/%s", host, port, streamKey)
}

func enrichRTMPPushFields(cam *api.Camera, _ string) {
	if !isRTMPPush(*cam) {
		return
	}
	if cam.StreamKey == "" && cam.ID != 0 {
		cam.StreamKey = CameraStreamName(*cam)
	}
	port := rtmpListenPort()
	cam.Port = port
	host := rtmp.AdvertiseHostForPublish()
	cam.IP = host
	cam.AllowAnonymous = true
	cam.PushSecretHash = ""
	cam.PushSecret = ""
	cam.PublishURL = buildPublishURL(host, port, cam.StreamKey)
}

// generateRTMPStreamKey 系统生成推流槽位名（用户不可填）。
func generateRTMPStreamKey() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "ingest_" + hex.EncodeToString(b), nil
}

// setupRTMPPushSlot 创建/更新推流槽位（无鉴权）。
func setupRTMPPushSlot(cam *api.Camera, _ api.CameraInput, _ *api.Camera) (revealSecret string, err error) {
	key := strings.TrimSpace(cam.StreamKey)
	if key == "" && cam.ID != 0 {
		key = CameraStreamName(*cam)
	}
	if key == "" {
		return "", fmt.Errorf("stream key required")
	}
	cam.StreamKey = key

	port := rtmpListenPort()
	cam.Port = port
	cam.IP = rtmp.AdvertiseHostForPublish()
	cam.AllowAnonymous = true
	cam.PushSecretHash = ""

	if _, err = streams.EnsureRTMPIngestSlot(key, "", ""); err != nil {
		return "", err
	}

	cam.StreamURL = "rtmp-ingest://" + key
	enrichRTMPPushFields(cam, "")
	return "", nil
}

func teardownRTMPPushSlot(cam api.Camera) {
	key := CameraStreamName(cam)
	streams.DeleteRTMPIngestSlot(key)
	streams.Delete(key)
}
