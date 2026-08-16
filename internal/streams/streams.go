package streams

import (
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/liuhengloveyou/FrameFlow/internal/api"
	"github.com/liuhengloveyou/FrameFlow/internal/app"
	"github.com/liuhengloveyou/FrameFlow/pkg/core"
	"github.com/liuhengloveyou/FrameFlow/pkg/fferrors"
	"github.com/rs/zerolog"
)

func Init() {
	var cfg struct {
		Streams map[string]any    `yaml:"streams"`
		Publish map[string]any    `yaml:"publish"`
		Preload map[string]string `yaml:"preload"`
	}

	app.LoadConfig(&cfg)

	log = app.GetLogger("streams")
	core.SetLogger(app.GetLogger("core"))

	for name, item := range cfg.Streams {
		streams[name] = NewStream(item)
	}

	api.RegisterBizAPI("", "api/streams", false, "", apiStreams)
	api.RegisterBizAPI("", "api/streams.dot", false, "", apiStreamsDOT)
	api.RegisterBizAPI("", "api/devices/rtmp", false, "", apiDevicesRTMP)
	api.RegisterBizAPI("", "api/devices/rtsp", false, "", apiDevicesRTSP)
	api.RegisterBizAPI("", "api/preload", false, "", apiPreload)
	api.RegisterBizAPI("", "api/schemes", false, "", apiSchemes)

	if cfg.Publish == nil && cfg.Preload == nil {
		return
	}

	time.AfterFunc(time.Second, func() {
		// range for nil map is OK
		for name, dst := range cfg.Publish {
			if stream := Get(name); stream != nil {
				Publish(stream, dst)
			}
		}
		for name, rawQuery := range cfg.Preload {
			if err := AddPreload(name, rawQuery); err != nil {
				log.Error().Err(err).Caller().Send()
			}
		}
	})
}

func New(name string, sources ...string) (*Stream, error) {
	for _, source := range sources {
		if !HasProducer(source) {
			return nil, fferrors.ErrStreamsSourceNotSupported
		}

		if err := Validate(source); err != nil {
			return nil, err
		}
	}

	stream := NewStream(sources)

	streamsMu.Lock()
	streams[name] = stream
	streamsMu.Unlock()

	return stream, nil
}

func Patch(name string, source string) (*Stream, error) {
	streamsMu.Lock()
	defer streamsMu.Unlock()

	// check if source links to some stream name from go2rtc
	if u, err := url.Parse(source); err == nil && u.Scheme == "rtsp" && len(u.Path) > 1 {
		rtspName := u.Path[1:]
		if stream, ok := streams[rtspName]; ok {
			if streams[name] != stream {
				// link (alias) streams[name] to streams[rtspName]
				streams[name] = stream
			}
			return stream, nil
		}
	}

	if stream, ok := streams[source]; ok {
		if name != source {
			// link (alias) streams[name] to streams[source]
			streams[name] = stream
		}
		return stream, nil
	}

	// check if src has supported scheme
	if !HasProducer(source) {
		return nil, fferrors.ErrStreamsSourceNotSupported
	}

	if err := Validate(source); err != nil {
		return nil, err
	}

	// check an existing stream with this name
	if stream, ok := streams[name]; ok {
		stream.SetSource(source)
		return stream, nil
	}

	// create new stream with this name
	stream := NewStream(source)
	streams[name] = stream
	return stream, nil
}

func GetOrPatch(query url.Values) (*Stream, error) {
	// check if src param exists
	source := query.Get("src")
	if source == "" {
		return nil, fferrors.ErrStreamsSourceEmpty
	}

	// check if src is stream name
	if stream := Get(source); stream != nil {
		return stream, nil
	}

	// RTMP 推流占位：前端/DB 可能带 rtmp-ingest://<stream_key>，实际注册名是 stream_key
	if key := strings.TrimPrefix(source, "rtmp-ingest://"); key != source && key != "" {
		if stream := Get(key); stream != nil {
			return stream, nil
		}
	}

	// check if name param provided
	if name := query.Get("name"); name != "" {
		return Patch(name, source)
	}

	// return new stream with src as name
	return Patch(source, source)
}

var log zerolog.Logger

// streams map

var streams = map[string]*Stream{}
var streamsMu sync.Mutex

func Get(name string) *Stream {
	streamsMu.Lock()
	defer streamsMu.Unlock()
	return streams[name]
}

// HasLiveIngest returns true when stream_key currently has an active push producer
// (RTMP publish / RTSP ANNOUNCE).
func HasLiveIngest(streamKey string) bool {
	streamKey = strings.TrimSpace(streamKey)
	if streamKey == "" {
		return false
	}
	return Get(streamKey).HasLiveExternalProducer()
}

func Delete(name string) {
	streamsMu.Lock()
	defer streamsMu.Unlock()
	delete(streams, name)
}

// LinkAlias 让 alias 名称指向与 target 相同的 Stream（推流槽位与 camera id 不一致时用）。
func LinkAlias(alias, target string) error {
	if alias == "" || target == "" {
		return nil
	}
	streamsMu.Lock()
	defer streamsMu.Unlock()
	st, ok := streams[target]
	if !ok || st == nil {
		return fferrors.ErrStreamsSourceEmpty
	}
	streams[alias] = st
	return nil
}

func GetAllNames() []string {
	streamsMu.Lock()
	names := make([]string, 0, len(streams))
	for name := range streams {
		names = append(names, name)
	}
	streamsMu.Unlock()
	return names
}

func GetAllSources() map[string][]string {
	streamsMu.Lock()
	sources := make(map[string][]string, len(streams))
	for name, stream := range streams {
		sources[name] = stream.Sources()
	}
	streamsMu.Unlock()
	return sources
}
