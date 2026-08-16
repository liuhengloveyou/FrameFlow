package yandex

import (
	"net/url"

	"github.com/liuhengloveyou/FrameFlow/internal/streams"
	"github.com/liuhengloveyou/FrameFlow/pkg/core"
	"github.com/liuhengloveyou/FrameFlow/pkg/yandex"
)

func Init() {
	streams.HandleFunc("yandex", func(source string) (core.Producer, error) {
		u, err := url.Parse(source)
		if err != nil {
			return nil, err
		}

		query := u.Query()
		token := query.Get("x_token")

		session, err := yandex.GetSession(token)
		if err != nil {
			return nil, err
		}

		deviceID := query.Get("device_id")

		if query.Has("snapshot") {
			rawURL, err := session.GetSnapshotURL(deviceID)
			if err != nil {
				return nil, err
			}
			rawURL += "/current.jpg?" + query.Get("snapshot") + "#header=Cookie:" + session.GetCookieString(rawURL)
			return streams.GetProducer(rawURL)
		}

		room, err := session.WebrtcCreateRoom(deviceID)
		if err != nil {
			return nil, err
		}

		return goloomClient(room.ServiceUrl, room.ServiceName, room.RoomId, room.ParticipantId, room.Credentials)
	})
}
