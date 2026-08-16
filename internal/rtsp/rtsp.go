package rtsp

import (
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/liuhengloveyou/FrameFlow/internal/app"
	"github.com/liuhengloveyou/FrameFlow/internal/streams"
	"github.com/liuhengloveyou/FrameFlow/pkg/core"
	"github.com/liuhengloveyou/FrameFlow/pkg/rtsp"
	"github.com/liuhengloveyou/FrameFlow/pkg/tcp"
	"github.com/rs/zerolog"
)

func Init() {
	log = app.GetLogger("rtsp")
	rtsp.SetLogger(app.GetLogger("rtsp"))

	// RTSP client support（拉流能力）
	streams.HandleFunc("rtsp", rtspHandler)
	streams.HandleFunc("rtsps", rtspHandler)
	streams.HandleFunc("rtspx", rtspHandler)

	row, err := app.GetRTSPConfig()
	if err != nil {
		log.Error().Err(err).Msg("[rtsp] get config")
		row = app.DefaultRTSPConfig()
	}
	setRuntimeFromRow(row)
	if err := applyListenState(row); err != nil {
		log.Error().Err(err).
			Bool("enabled", row.Enabled).
			Int("port", row.ListenPort).
			Msg("[rtsp] apply listen on init")
	}

	app.Info["rtsp"] = map[string]any{
		"enabled": row.Enabled,
		"port":    row.ListenPort,
		"listen":  row.ListenAddr(),
	}
	log.Info().
		Bool("enabled", row.Enabled).
		Int("port", row.ListenPort).
		Str("listen", row.ListenAddr()).
		Msg("[rtsp] ready (HTTP via api.WireProtocols / MountRTSPSettings)")
}

type Handler func(conn *rtsp.Conn) bool

func HandleFunc(handler Handler) {
	handlers = append(handlers, handler)
}

// Port 返回库内已启用的 RTSP 监听端口；未启用则空字符串。
func Port() string {
	row, err := app.GetRTSPConfig()
	if err != nil || !row.Enabled || row.ListenPort <= 0 {
		return ""
	}
	return strconv.Itoa(row.ListenPort)
}

// internal

var log zerolog.Logger
var handlers []Handler
var defaultMedias []*core.Media

func rtspHandler(rawURL string) (core.Producer, error) {
	rawURL, rawQuery, _ := strings.Cut(rawURL, "#")

	conn := rtsp.NewClient(rawURL)
	conn.Backchannel = true
	conn.UserAgent = app.UserAgent

	if rawQuery != "" {
		query := streams.ParseQuery(rawQuery)
		conn.Backchannel = query.Get("backchannel") == "1"
		conn.Media = query.Get("media")
		conn.Timeout = core.Atoi(query.Get("timeout"))
		conn.Transport = query.Get("transport")
	}

	if log.Trace().Enabled() {
		conn.Listen(func(msg any) {
			switch msg := msg.(type) {
			case *tcp.Request:
				log.Trace().Msgf("[rtsp] client request:\n%s", msg)
			case *tcp.Response:
				log.Trace().Msgf("[rtsp] client response:\n%s", msg)
			case string:
				log.Trace().Msgf("[rtsp] client msg: %s", msg)
			}
		})
	}

	if err := conn.Dial(); err != nil {
		return nil, err
	}

	if err := conn.Describe(); err != nil {
		if !conn.Backchannel {
			return nil, err
		}
		log.Trace().Msgf("[rtsp] describe (backchannel=%t) err: %v", conn.Backchannel, err)

		// second try without backchannel, we need to reconnect
		conn.Backchannel = false
		if err = conn.Dial(); err != nil {
			return nil, err
		}
		if err = conn.Describe(); err != nil {
			return nil, err
		}
	}

	return conn, nil
}

func tcpHandler(conn *rtsp.Conn) {
	var name string
	var closer func()

	trace := log.Trace().Enabled()
	level := zerolog.WarnLevel

	conn.Listen(func(msg any) {
		if trace {
			switch msg := msg.(type) {
			case *tcp.Request:
				log.Trace().Msgf("[rtsp] server request:\n%s", msg)
			case *tcp.Response:
				log.Trace().Msgf("[rtsp] server response:\n%s", msg)
			}
		}

		switch msg {
		case rtsp.MethodDescribe:
			if len(conn.URL.Path) == 0 {
				log.Warn().Msg("[rtsp] server empty URL on DESCRIBE")
				return
			}

			name = conn.URL.Path[1:]

			stream := streams.Get(name)
			if stream == nil {
				return
			}

			log.Debug().Str("stream", name).Msg("[rtsp] new consumer")

			conn.SessionName = app.UserAgent

			query := conn.URL.Query()
			conn.Medias = ParseQuery(query)
			if conn.Medias == nil {
				for _, media := range defaultMedias {
					conn.Medias = append(conn.Medias, media.Clone())
				}
			}

			if query.Get("backchannel") == "1" {
				conn.Medias = append(conn.Medias, &core.Media{
					Kind:      core.KindAudio,
					Direction: core.DirectionRecvonly,
					Codecs: []*core.Codec{
						{Name: core.CodecOpus, ClockRate: 48000, Channels: 2},
						{Name: core.CodecPCM, ClockRate: 16000},
						{Name: core.CodecPCMA, ClockRate: 16000},
						{Name: core.CodecPCMU, ClockRate: 16000},
						{Name: core.CodecPCM, ClockRate: 8000},
						{Name: core.CodecPCMA, ClockRate: 8000},
						{Name: core.CodecPCMU, ClockRate: 8000},
						{Name: core.CodecAAC, ClockRate: 8000},
						{Name: core.CodecAAC, ClockRate: 16000},
					},
				})
			}

			if s := query.Get("pkt_size"); s != "" {
				conn.PacketSize = uint16(core.Atoi(s))
			}

			// param name like ffmpeg style https://ffmpeg.org/ffmpeg-protocols.html
			if s := query.Get("log_level"); s != "" {
				if lvl, err := zerolog.ParseLevel(s); err == nil {
					level = lvl
				}
			}

			// will help to protect looping requests to same source
			conn.Connection.Source = query.Get("source")

			if err := stream.AddConsumer(conn); err != nil {
				log.WithLevel(level).Err(err).Str("stream", name).Msg("[rtsp]")
				return
			}

			closer = func() {
				stream.RemoveConsumer(conn)
			}

		case rtsp.MethodAnnounce:
			if len(conn.URL.Path) == 0 {
				log.Warn().Msg("[rtsp] server empty URL on ANNOUNCE")
				return
			}

			name = conn.URL.Path[1:]
			// 路径中可能仍带 query（部分客户端）；优先用 URL.Query
			query := conn.URL.Query()
			token := query.Get(streams.PushTokenQueryParam)
			if err := streams.VerifyRTSPPushToken(name, token); err != nil {
				log.Warn().Str("stream", name).Msg("[rtsp] announce rejected: push secret")
				return
			}

			stream := streams.Get(name)
			if stream == nil {
				log.Warn().Str("stream", name).Msg("[rtsp] announce rejected: unknown stream (register device push slot first)")
				return
			}

			if s := query.Get("timeout"); s != "" {
				conn.Timeout = core.Atoi(s)
			}

			log.Info().Str("stream", name).Msg("[rtsp] new producer (ANNOUNCE)")

			stream.AddProducer(conn)

			closer = func() {
				stream.RemoveProducer(conn)
			}
		}
	})

	if err := conn.Accept(); err != nil {
		if errors.Is(err, rtsp.FailedAuth) {
			log.Warn().Str("remote_addr", conn.Connection.RemoteAddr).Msg("[rtsp] failed authentication")
		} else if err != io.EOF {
			log.WithLevel(level).Err(err).Caller().Send()
		}
		if closer != nil {
			closer()
		}
		_ = conn.Close()
		return
	}

	for _, handler := range handlers {
		if handler(conn) {
			return
		}
	}

	if closer != nil {
		if err := conn.Handle(); err != nil {
			log.Debug().Err(err).Msg("[rtsp] handle")
		}

		closer()

		log.Debug().Str("stream", name).Msg("[rtsp] disconnect")
	}

	_ = conn.Close()
}

func ParseQuery(query map[string][]string) []*core.Media {
	if v := query["mp4"]; v != nil {
		return []*core.Media{
			{
				Kind:      core.KindVideo,
				Direction: core.DirectionSendonly,
				Codecs: []*core.Codec{
					{Name: core.CodecH264},
					{Name: core.CodecH265},
				},
			},
			{
				Kind:      core.KindAudio,
				Direction: core.DirectionSendonly,
				Codecs: []*core.Codec{
					{Name: core.CodecAAC},
				},
			},
		}
	}

	return core.ParseQuery(query)
}
