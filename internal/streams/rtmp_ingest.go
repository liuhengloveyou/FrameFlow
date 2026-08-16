package streams

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/liuhengloveyou/FrameFlow/pkg/fferrors"
)

// RTMP 推流接入：占位流 + 可选推流密钥（见 internal/rtmp 从 tcUrl 读取）。

// PushTokenQueryParam connect 参数 tcUrl 上用于校验推流密钥的查询参数名。
const PushTokenQueryParam = "push_key"

var ingestKeyPattern = regexp.MustCompile(`^[\p{L}\p{N}][\p{L}\p{N}_.-]{1,127}$`)

var (
	pushSecretMu sync.RWMutex
	// streamKey -> sha256(push_secret)；仅对通过本 API 创建且下发了密钥的槽位写入
	pushSecretHash = map[string][sha256.Size]byte{}
)

// RtmpResolveStreamKey 将 RTMP connect 的 app 与 publish 的 stream 名映射为 streams 中的键。
// 与 AINVR internal/go2rtcmedia.StreamKey 一致（仅把路径中的「/」换成「_」，其余字符原样保留），
// 便于大华等平台「app=live + 子流名」与 WebRTC src、HTTP 拉流名一致。
func RtmpResolveStreamKey(app, publishStream string) string {
	app = strings.TrimSpace(strings.ReplaceAll(app, "/", "_"))
	publishStream = strings.TrimSpace(strings.ReplaceAll(publishStream, "/", "_"))
	if app == "" {
		return ""
	}
	if publishStream == "" {
		return app
	}
	return app + "_" + publishStream
}

// CreateRTMPIngestSlot 创建仅用于 RTMP 推入的占位流（无固定拉流源）。
// streamKey 为空则自动生成 ingest_<随机16hex>。若 pushSecret 非空，推流须在 connect 的 tcUrl 上携带对应查询参数（默认 push_key）。
func CreateRTMPIngestSlot(streamKey, pushSecret string) (key string, err error) {
	if streamKey == "" {
		var b [8]byte
		if _, err = rand.Read(b[:]); err != nil {
			return "", err
		}
		streamKey = "ingest_" + hex.EncodeToString(b[:])
	}
	if !ingestKeyPattern.MatchString(streamKey) {
		return "", fferrors.ErrStreamsStreamKeyInvalid
	}

	streamsMu.Lock()
	if _, exists := streams[streamKey]; exists {
		streamsMu.Unlock()
		return "", fferrors.ErrStreamsStreamKeyExists
	}
	streams[streamKey] = NewStream(nil)
	streamsMu.Unlock()

	if pushSecret != "" {
		setPushSecretHash(streamKey, pushSecret)
	} else {
		clearPushSecretHash(streamKey)
	}

	log.Info().
		Str("stream_key", streamKey).
		Bool("push_secret_registered", pushSecret != "").
		Msg("[streams] rtmp ingest slot created")
	return streamKey, nil
}

// EnsureRTMPIngestSlot 确保推流占位流存在；可按明文密钥或已持久化的 sha256 hex 恢复鉴权。
// 若流已存在则只更新密钥登记，不报「已存在」错误。
func EnsureRTMPIngestSlot(streamKey, pushSecret, pushSecretHashHex string) (key string, err error) {
	if streamKey == "" {
		return CreateRTMPIngestSlot("", pushSecret)
	}
	if !ingestKeyPattern.MatchString(streamKey) {
		return "", fferrors.ErrStreamsStreamKeyInvalid
	}

	streamsMu.Lock()
	if _, exists := streams[streamKey]; !exists {
		streams[streamKey] = NewStream(nil)
	}
	streamsMu.Unlock()

	switch {
	case pushSecret != "":
		setPushSecretHash(streamKey, pushSecret)
	case pushSecretHashHex != "":
		if err = restorePushSecretHash(streamKey, pushSecretHashHex); err != nil {
			return "", err
		}
	default:
		clearPushSecretHash(streamKey)
	}

	return streamKey, nil
}

// DeleteRTMPIngestSlot 删除推流占位流与密钥登记。
func DeleteRTMPIngestSlot(streamKey string) {
	if streamKey == "" {
		return
	}
	streamsMu.Lock()
	delete(streams, streamKey)
	streamsMu.Unlock()
	clearPushSecretHash(streamKey)
	log.Info().Str("stream_key", streamKey).Msg("[streams] rtmp ingest slot deleted")
}

// HashPushSecret 返回推流密钥的 sha256 hex（持久化用）。
func HashPushSecret(secret string) string {
	if secret == "" {
		return ""
	}
	h := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(h[:])
}

// VerifyRTMPPushToken 在推流建立前调用：若该 streamKey 未登记密钥则放行；已登记则 token 必须匹配。
func VerifyRTMPPushToken(streamKey, token string) error {
	pushSecretMu.RLock()
	want, ok := pushSecretHash[streamKey]
	pushSecretMu.RUnlock()
	if !ok {
		log.Debug().Str("stream_key", streamKey).Msg("[streams] rtmp push token: no secret registered, allow")
		return nil
	}
	got := sha256.Sum256([]byte(token))
	if subtle.ConstantTimeCompare(want[:], got[:]) != 1 {
		log.Warn().Str("stream_key", streamKey).Msg("[streams] rtmp push token: mismatch or missing in tcUrl")
		return fferrors.ErrRtmpPushSecretInvalid
	}
	log.Debug().Str("stream_key", streamKey).Msg("[streams] rtmp push token: ok")
	return nil
}

func setPushSecretHash(streamKey, secret string) {
	h := sha256.Sum256([]byte(secret))
	pushSecretMu.Lock()
	pushSecretHash[streamKey] = h
	pushSecretMu.Unlock()
}

func restorePushSecretHash(streamKey, hashHex string) error {
	b, err := hex.DecodeString(strings.TrimSpace(hashHex))
	if err != nil || len(b) != sha256.Size {
		return fmt.Errorf("invalid push secret hash")
	}
	var h [sha256.Size]byte
	copy(h[:], b)
	pushSecretMu.Lock()
	pushSecretHash[streamKey] = h
	pushSecretMu.Unlock()
	return nil
}

func clearPushSecretHash(streamKey string) {
	pushSecretMu.Lock()
	delete(pushSecretHash, streamKey)
	pushSecretMu.Unlock()
}
