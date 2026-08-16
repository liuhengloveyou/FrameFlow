package rtmp

import (
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/liuhengloveyou/FrameFlow/internal/api"
	"github.com/liuhengloveyou/FrameFlow/internal/app"
	"github.com/liuhengloveyou/FrameFlow/internal/streams"
	"github.com/liuhengloveyou/FrameFlow/pkg/core"
	"github.com/liuhengloveyou/FrameFlow/pkg/fferrors"
	"github.com/liuhengloveyou/FrameFlow/pkg/flv"
	"github.com/liuhengloveyou/FrameFlow/pkg/rtmp"
	"github.com/rs/zerolog"
)

// pushTokenQuery connect.tcUrl 中用于校验推流密钥的查询参数名（与 streams.PushTokenQueryParam 默认一致，可由库内 push_token_query 覆盖）。
var pushTokenQuery = streams.PushTokenQueryParam

var log zerolog.Logger

// Init 注册协议处理能力，并按数据库配置启动 RTMP 监听。
func Init() {
	log = app.GetLogger("rtmp")
	pushTokenQuery = streams.PushTokenQueryParam

	streams.HandleFunc("rtmp", streamsHandle)
	streams.HandleFunc("rtmps", streamsHandle)
	streams.HandleFunc("rtmpx", streamsHandle)

	streams.HandleConsumerFunc("rtmp", streamsConsumerHandle)
	streams.HandleConsumerFunc("rtmps", streamsConsumerHandle)
	streams.HandleConsumerFunc("rtmpx", streamsConsumerHandle)

	row, err := app.GetRTMPConfig()
	if err != nil {
		log.Error().Err(err).Msg("[rtmp] get config")
		row = app.DefaultRTMPConfig()
	}
	if tq := strings.TrimSpace(row.PushTokenQuery); tq != "" {
		pushTokenQuery = tq
	}
	if err := applyListenState(row.ListenAddr(), row.PushTokenQuery); err != nil {
		log.Error().Err(err).
			Bool("enabled", row.Enabled).
			Int("port", row.ListenPort).
			Msg("[rtmp] apply listen on init")
	}

	log.Info().
		Bool("enabled", row.Enabled).
		Int("port", row.ListenPort).
		Str("listen", row.ListenAddr()).
		Msg("[rtmp] ready (HTTP via api.WireProtocols / MountRTMPSettings)")
}

// StreamFLV 供 api.WireProtocols / MountRTMPSettings 挂载 api/stream.flv。
func StreamFLV(w http.ResponseWriter, r *http.Request) { apiHandle(w, r) }

func rtmpPublishResolve(c *rtmp.Conn) (*streams.Stream, string) {
	k := streams.RtmpResolveStreamKey(c.App, c.Stream)
	if s := streams.Get(k); s != nil {
		log.Debug().Str("resolved", k).Str("match", "composite").Msg("[rtmp] stream lookup hit")
		return s, k
	}
	if s := streams.Get(c.App); s != nil {
		log.Debug().Str("app", c.App).Str("match", "app_only").Msg("[rtmp] stream lookup hit")
		return s, c.App
	}
	log.Debug().
		Str("app", c.App).
		Str("publish_stream", c.Stream).
		Str("resolved", k).
		Msg("[rtmp] stream lookup miss")
	return nil, k
}

func tcpHandle(netConn net.Conn) error {
	rtmpConn, err := rtmp.NewServer(netConn)
	if err != nil {
		return err
	}

	if err = rtmpConn.ReadCommands(); err != nil {
		return err
	}

	remote := netConn.RemoteAddr().String()
	resolved := streams.RtmpResolveStreamKey(rtmpConn.App, rtmpConn.Stream)
	log.Info().
		Str("remote", remote).
		Str("intent", rtmpConn.Intent).
		Str("app", rtmpConn.App).
		Str("publish_stream", rtmpConn.Stream).
		Str("resolved", resolved).
		Msg("[rtmp] command")

	switch rtmpConn.Intent {
	case rtmp.CommandPlay:
		stream, _ := rtmpPublishResolve(rtmpConn)
		if stream == nil {
			log.Warn().
				Str("remote", remote).
				Str("resolved", resolved).
				Msg("[rtmp] play rejected: stream not found")
			return fferrors.StreamNotFoundKey(streams.RtmpResolveStreamKey(rtmpConn.App, rtmpConn.Stream))
		}

		cons := flv.NewConsumer()
		if err = stream.AddConsumer(cons); err != nil {
			log.Error().Err(err).Str("resolved", resolved).Msg("[rtmp] play AddConsumer failed")
			return err
		}

		defer stream.RemoveConsumer(cons)

		if err = rtmpConn.WriteStart(); err != nil {
			return err
		}

		log.Debug().Str("remote", remote).Str("resolved", resolved).Int("senders", len(cons.Senders)).Msg("[rtmp] play WriteTo begin")
		_, _ = cons.WriteTo(rtmpConn)

		log.Debug().Str("remote", remote).Str("resolved", resolved).Msg("[rtmp] play session end")
		return nil

	case rtmp.CommandPublish:
		stream, authKey := rtmpPublishResolve(rtmpConn)
		if stream == nil {
			log.Warn().
				Str("remote", remote).
				Str("resolved", resolved).
				Msg("[rtmp] publish rejected: stream not found")
			return fferrors.StreamNotFoundKey(streams.RtmpResolveStreamKey(rtmpConn.App, rtmpConn.Stream))
		}

		token := rtmpConn.PushTokenFromTcUrl(pushTokenQuery)
		log.Debug().
			Str("remote", remote).
			Str("auth_key", authKey).
			Str("query_param", pushTokenQuery).
			Bool("token_present", token != "").
			Msg("[rtmp] publish auth")
		if err := streams.VerifyRTMPPushToken(authKey, token); err != nil {
			log.Warn().Str("remote", remote).Str("auth_key", authKey).Err(err).Msg("[rtmp] publish rejected")
			return err
		}

		if err = rtmpConn.WriteStart(); err != nil {
			return err
		}

		prod, err := rtmpConn.Producer()
		if err != nil {
			return err
		}

		stream.AddProducer(prod)

		defer stream.RemoveProducer(prod)

		log.Info().Str("remote", remote).Str("auth_key", authKey).Msg("[rtmp] publish start")
		_ = prod.Start()

		log.Debug().Str("remote", remote).Str("auth_key", authKey).Msg("[rtmp] publish session end")
		return nil
	}

	log.Warn().Str("remote", remote).Str("intent", rtmpConn.Intent).Msg("[rtmp] unknown intent")
	return fferrors.ErrRtmpUnknownCommand(rtmpConn.Intent)
}

func streamsHandle(url string) (core.Producer, error) {
	return rtmp.DialPlay(url)
}

func streamsConsumerHandle(url string) (core.Consumer, func(), error) {
	cons := flv.NewConsumer()
	run := func() {
		wr, err := rtmp.DialPublish(url, cons)
		if err != nil {
			return
		}
		_, err = cons.WriteTo(wr)
	}

	return cons, run, nil
}

func apiHandle(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		outputFLV(w, r)
	} else {
		inputFLV(w, r)
	}
}

func outputFLV(w http.ResponseWriter, r *http.Request) {
	src := r.URL.Query().Get("src")
	stream := streams.Get(src)
	if stream == nil {
		log.Warn().Str("src", src).Msg("[rtmp] stream.flv not found")
		http.Error(w, api.StreamNotFound, http.StatusNotFound)
		return
	}

	cons := flv.NewConsumer()
	cons.WithRequest(r)

	if err := stream.AddConsumer(cons); err != nil {
		log.Error().Err(err).Str("src", src).Msg("[rtmp] stream.flv AddConsumer failed")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h := w.Header()
	h.Set("Content-Type", "video/x-flv")

	log.Debug().Str("src", src).Int("senders", len(cons.Senders)).Msg("[rtmp] stream.flv WriteTo begin")
	_, _ = cons.WriteTo(w)

	stream.RemoveConsumer(cons)
}

func inputFLV(w http.ResponseWriter, r *http.Request) {
	dst := r.URL.Query().Get("dst")
	stream := streams.Get(dst)
	if stream == nil {
		http.Error(w, api.StreamNotFound, http.StatusNotFound)
		return
	}

	client, err := flv.Open(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	stream.AddProducer(client)

	if err = client.Start(); err != nil && err != io.EOF {
		log.Warn().Err(err).Caller().Send()
	}

	stream.RemoveProducer(client)
}
