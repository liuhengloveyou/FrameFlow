package app

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/liuhengloveyou/FrameFlow/pkg/creds"
	"github.com/mattn/go-isatty"
	"github.com/rs/zerolog"
)

var MemoryLog = newBuffer()

func GetLogger(module string) zerolog.Logger {
	Logger.Trace().Str("module", module).Msgf("[log] init")

	if s, ok := modules[module]; ok {
		lvl, err := zerolog.ParseLevel(s)
		if err == nil {
			return Logger.Level(lvl)
		}
		Logger.Warn().Err(err).Caller().Send()
	}

	return Logger
}

// initLogger support:
// - output: empty (only to memory), stderr, stdout
// - format: empty (autodetect color support), color, json, text
// - time:   empty (disable timestamp), UNIXMS, UNIXMICRO, UNIXNANO
// - level:  disabled, trace, debug, info, warn, error...
func initLogger() {
	var cfg struct {
		Mod map[string]string `yaml:"log"`
	}

	cfg.Mod = modules // defaults

	LoadConfig(&cfg)

	var writer io.Writer

	switch output, path, _ := strings.Cut(modules["output"], ":"); output {
	case "stderr":
		writer = os.Stderr
	case "stdout":
		writer = os.Stdout
	case "file":
		if path == "" {
			path = "logs/frameflow.log"
		}
		if dir := filepath.Dir(path); dir != "" && dir != "." {
			_ = os.MkdirAll(dir, 0755)
		}
		// if fail - only MemoryLog will be available
		writer, _ = os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	}

	timeFormat := modules["time"]

	if writer != nil {
		if format := modules["format"]; format != "json" {
			console := &zerolog.ConsoleWriter{Out: writer}

			switch format {
			case "text":
				console.NoColor = true
			case "color":
				console.NoColor = false // useless, but anyway
			default:
				// autodetection if output support color
				// go-isatty - dependency for go-colorable - dependency for ConsoleWriter
				console.NoColor = !isatty.IsTerminal(writer.(*os.File).Fd())
			}

			// 含时间戳时 zerolog 默认 PartsOrder 不含 caller，须显式列出才能在控制台看到 file:line
			if timeFormat != "" {
				console.TimeFormat = "15:04:05.000"
				console.PartsOrder = []string{
					zerolog.TimestampFieldName,
					zerolog.LevelFieldName,
					zerolog.CallerFieldName,
					zerolog.MessageFieldName,
				}
			} else {
				console.PartsOrder = []string{
					zerolog.LevelFieldName,
					zerolog.CallerFieldName,
					zerolog.MessageFieldName,
				}
			}

			writer = console
		}

		writer = zerolog.MultiLevelWriter(writer, MemoryLog)
	} else {
		writer = MemoryLog
	}

	writer = creds.SecretWriter(writer)

	lvl, _ := zerolog.ParseLevel(modules["level"])
	base := zerolog.New(writer).Level(lvl)

	callerOn := modules["caller"] != "false"
	ctx := base.With()
	if timeFormat != "" {
		zerolog.TimeFieldFormat = timeFormat
		ctx = ctx.Timestamp()
	}
	if callerOn {
		zerolog.CallerMarshalFunc = func(_ uintptr, file string, line int) string {
			rel := file
			if wd, err := os.Getwd(); err == nil {
				if r, err := filepath.Rel(wd, file); err == nil {
					rel = r
				}
			}
			return rel + ":" + strconv.Itoa(line)
		}
		Logger = ctx.Caller().Logger()
	} else {
		Logger = ctx.Logger()
	}
}

var Logger zerolog.Logger

// modules log levels
var modules = map[string]string{
	"format": "", // useless, but anyway
	"level":  "info",
	"output": "stdout", // TODO: change to stderr someday
	"time":   zerolog.TimeFormatUnixMs,
	"caller": "true", // log.caller: false 关闭 file:line（如纯 JSON 采集）
}

const (
	chunkCount = 16
	chunkSize  = 1 << 16
)

type circularBuffer struct {
	chunks [][]byte
	r, w   int
	mu     sync.Mutex
}

func newBuffer() *circularBuffer {
	b := &circularBuffer{chunks: make([][]byte, 0, chunkCount)}
	// create first chunk
	b.chunks = append(b.chunks, make([]byte, 0, chunkSize))
	return b
}

func (b *circularBuffer) Write(p []byte) (n int, err error) {
	n = len(p)

	b.mu.Lock()
	// check if chunk has size
	if len(b.chunks[b.w])+n > chunkSize {
		// increase write chunk index
		if b.w++; b.w == chunkCount {
			b.w = 0
		}
		// check overflow
		if b.r == b.w {
			// increase read chunk index
			if b.r++; b.r == chunkCount {
				b.r = 0
			}
		}
		// check if current chunk exists
		if b.w == len(b.chunks) {
			// allocate new chunk
			b.chunks = append(b.chunks, make([]byte, 0, chunkSize))
		} else {
			// reset len of current chunk
			b.chunks[b.w] = b.chunks[b.w][:0]
		}
	}

	b.chunks[b.w] = append(b.chunks[b.w], p...)
	b.mu.Unlock()
	return
}

func (b *circularBuffer) WriteTo(w io.Writer) (n int64, err error) {
	buf := make([]byte, 0, chunkCount*chunkSize)

	// use temp buffer inside mutex because w.Write can take some time
	b.mu.Lock()
	for i := b.r; ; {
		buf = append(buf, b.chunks[i]...)
		if i == b.w {
			break
		}
		if i++; i == chunkCount {
			i = 0
		}
	}
	b.mu.Unlock()

	nn, err := w.Write(buf)
	return int64(nn), err
}

func (b *circularBuffer) Reset() {
	b.mu.Lock()
	b.chunks[0] = b.chunks[0][:0]
	b.r = 0
	b.w = 0
	b.mu.Unlock()
}
