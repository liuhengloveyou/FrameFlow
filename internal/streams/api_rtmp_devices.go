package streams

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/liuhengloveyou/FrameFlow/internal/api"
	"github.com/liuhengloveyou/FrameFlow/pkg/creds"
)

// apiDevicesRTMP 管理 RTMP IPC 推流槽位：标识 stream_key、可选推流密钥（见 internal/rtmp README）。
func apiDevicesRTMP(w http.ResponseWriter, r *http.Request) {
	w = creds.SecretResponse(w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	switch r.Method {
	case http.MethodGet:
		keys := GetAllNames()
		log.Debug().Int("count", len(keys)).Msg("[streams] api devices/rtmp GET")
		api.ResponseJSON(w, map[string]any{
			"stream_keys":      keys,
			"push_token_query": PushTokenQueryParam,
		})

	case http.MethodPost:
		var in struct {
			StreamKey      string `json:"stream_key"`
			PushSecret     string `json:"push_secret"`
			AllowAnonymous bool   `json:"allow_anonymous"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		secret := in.PushSecret
		if !in.AllowAnonymous && secret == "" {
			b := make([]byte, 16)
			if _, err := rand.Read(b); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			secret = hex.EncodeToString(b)
		}
		if in.AllowAnonymous {
			secret = ""
		}

		key, err := CreateRTMPIngestSlot(in.StreamKey, secret)
		if err != nil {
			log.Warn().Err(err).Str("stream_key", in.StreamKey).Bool("allow_anonymous", in.AllowAnonymous).Msg("[streams] api devices/rtmp POST failed")
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		log.Info().
			Str("stream_key", key).
			Bool("allow_anonymous", in.AllowAnonymous).
			Bool("response_includes_secret", secret != "").
			Msg("[streams] api devices/rtmp POST ok")
		out := map[string]any{
			"stream_key":       key,
			"push_token_query": PushTokenQueryParam,
			"hint":             "推流 URL 一般为 rtmp://<服务器>:1935/<stream_key>；鉴权时 encoder 的 tcUrl 须带 ?" + PushTokenQueryParam + "=<push_secret>（见 internal/rtmp README）。",
		}
		if secret != "" {
			out["push_secret"] = secret
		}
		api.ResponseJSON(w, out)

	case http.MethodDelete:
		key := r.URL.Query().Get("stream_key")
		if key == "" {
			http.Error(w, "missing stream_key", http.StatusBadRequest)
			return
		}
		log.Info().Str("stream_key", key).Msg("[streams] api devices/rtmp DELETE")
		DeleteRTMPIngestSlot(key)
		log.Info().Str("stream_key", key).Msg("[streams] api devices/rtmp DELETE ok")
		api.ResponseJSON(w, map[string]any{"ok": true, "stream_key": key})

	default:
		http.Error(w, "", http.StatusMethodNotAllowed)
	}
}
