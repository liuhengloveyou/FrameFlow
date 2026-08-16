package cameras

import (
	"strings"
	"time"

	"github.com/liuhengloveyou/FrameFlow/internal/api"
	"github.com/liuhengloveyou/FrameFlow/internal/app"
	"github.com/liuhengloveyou/FrameFlow/internal/gb28181"
	"github.com/liuhengloveyou/FrameFlow/internal/streams"
)

// ChannelStreamName returns the go2rtc stream key for a device channel.
// Default / only channel keeps the base stream name so existing links still work.
func ChannelStreamName(cameraStreamName, channelID string) string {
	cameraStreamName = strings.TrimSpace(cameraStreamName)
	channelID = strings.TrimSpace(channelID)
	if cameraStreamName == "" {
		return channelID
	}
	if channelID == "" || channelID == cameraStreamName {
		return cameraStreamName
	}
	return cameraStreamName + "/" + channelID
}

func (s *service) syncStream(cam api.Camera) {
	base := CameraStreamName(cam)
	if isRTMPPush(cam) {
		key := base
		if _, err := streams.EnsureRTMPIngestSlot(key, "", cam.PushSecretHash); err != nil {
			app.Logger.Warn().Err(err).Uint("id", cam.ID).Str("streamKey", key).Msg("[cameras] ensure rtmp push slot")
		}
		app.Logger.Debug().
			Uint("id", cam.ID).
			Str("streamKey", key).
			Msg("[cameras] rtmp push stream synced")
		return
	}
	if isRTSPPush(cam) {
		key := base
		if _, err := streams.EnsureRTSPIngestSlot(key, "", cam.PushSecretHash); err != nil {
			app.Logger.Warn().Err(err).Uint("id", cam.ID).Str("streamKey", key).Msg("[cameras] ensure rtsp push slot")
		}
		app.Logger.Debug().
			Uint("id", cam.ID).
			Str("streamKey", key).
			Msg("[cameras] rtsp push stream synced")
		return
	}
	if cam.StreamURL != "" && base != "" {
		if _, err := streams.Patch(base, cam.StreamURL); err != nil {
			app.Logger.Warn().Err(err).Uint("id", cam.ID).Str("streamUrl", cam.StreamURL).Msg("[cameras] streams.Patch")
		}
	}
	for _, ch := range cam.Channels {
		if ch.StreamURL == "" || ch.ChannelID == "" {
			continue
		}
		name := ChannelStreamName(base, ch.ChannelID)
		if name == base {
			continue
		}
		if _, err := streams.Patch(name, ch.StreamURL); err != nil {
			app.Logger.Warn().Err(err).Str("name", name).Str("streamUrl", ch.StreamURL).Msg("[cameras] channel stream")
			continue
		}
	}
	app.Logger.Debug().
		Uint("id", cam.ID).
		Str("streamUrl", cam.StreamURL).
		Int("channels", len(cam.Channels)).
		Msg("[cameras] stream synced")
}

func (s *service) syncAllStreams() {
	page, err := s.store.List(api.CameraQuery{Page: 1, PageSize: 5000})
	if err != nil {
		app.Logger.Warn().Err(err).Msg("[cameras] syncAllStreams list")
		return
	}
	gb28181.AttachCameraChannels(page.Items)
	for i := range page.Items {
		s.syncStream(page.Items[i])
	}
	app.Logger.Info().Int("count", len(page.Items)).Msg("[cameras] streams synced")
}

// scheduleStreamSync waits for gb28181 store, then registers all camera/channel streams.
func scheduleStreamSync() {
	go func() {
		deadline := time.Now().Add(15 * time.Second)
		for time.Now().Before(deadline) {
			if gb28181.DefaultStore() != nil && svc != nil {
				svc.syncAllStreams()
				return
			}
			time.Sleep(200 * time.Millisecond)
		}
		if svc != nil {
			svc.syncAllStreams()
		}
	}()
}
