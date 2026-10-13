package vision

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

// 调试用：FRAMEFLOW_VISION_DUMP 指向目录时，把 vision 每拍收到的原始增量码流
// 逐份落盘（000001.bin...），用于离线复现「喂给解码器的字节到底是什么」。
// 仅排障用，不设即零开销。
var visionDumpSeq atomic.Uint64

func dumpVisionSample(b []byte) {
	dir := os.Getenv("FRAMEFLOW_VISION_DUMP")
	if dir == "" || len(b) == 0 {
		return
	}
	n := visionDumpSeq.Add(1)
	name := filepath.Join(dir, fmt.Sprintf("%06d.bin", n))
	if err := os.WriteFile(name, b, 0o644); err != nil {
		log.Warn().Err(err).Str("dir", dir).Msg("[vision] dump sample")
	}
}

// dumpVisionFrame：FRAMEFLOW_VISION_DUMP_RGB 指向目录时，把「送进模型前」的解码帧
// 同时落两份：
//   - NNNNNN.ppm：RGB888 原样（P6，无编码器介入）→ 用 ffmpeg/看图工具独立核对画面；
//   - NNNNNN.jpg：走自有 EncodeJPEG（即告警证据图同款编码器）→ 两者对比即可区分
//     「解码出的帧坏了」与「证据 JPEG 编码器坏了」。
//
// 只落前 maxFrames 帧，避免长时间跑攒出大量文件。仅排障用，不设即零开销。
var visionFrameSeq atomic.Uint64

const visionFrameDumpMax = 60

func dumpVisionFrame(rgb []byte, w, h int) {
	dir := os.Getenv("FRAMEFLOW_VISION_DUMP_RGB")
	if dir == "" || w < 1 || h < 1 || len(rgb) < w*h*3 {
		return
	}
	n := visionFrameSeq.Add(1)
	if n > visionFrameDumpMax {
		return
	}
	base := filepath.Join(dir, fmt.Sprintf("%06d", n))

	f, err := os.Create(base + ".ppm")
	if err != nil {
		log.Warn().Err(err).Str("dir", dir).Msg("[vision] dump frame ppm")
		return
	}
	if _, err = f.WriteString(fmt.Sprintf("P6\n%d %d\n255\n", w, h)); err == nil {
		_, err = f.Write(rgb[:w*h*3])
	}
	f.Close()
	if err != nil {
		log.Warn().Err(err).Str("path", base+".ppm").Msg("[vision] dump frame ppm write")
	}

	if jpg, jerr := EncodeJPEG(rgb[:w*h*3], w, h); jerr == nil {
		if werr := os.WriteFile(base+".jpg", jpg, 0o644); werr != nil {
			log.Warn().Err(werr).Str("path", base+".jpg").Msg("[vision] dump frame jpg")
		}
	}
}

// FRAMEFLOW_VISION_DEBUG_STREAM=key1,key2：启动后延迟自启这些流的检测 worker，
// 绕过「必须先有 WebRTC DataChannel 订阅」的门槛（调 SAMPLE 链路时用）。
func init() {
	list := os.Getenv("FRAMEFLOW_VISION_DEBUG_STREAM")
	if list == "" {
		return
	}
	go func() {
		time.Sleep(3 * time.Second)
		for _, k := range strings.Split(list, ",") {
			k = strings.TrimSpace(k)
			if k == "" {
				continue
			}
			log.Info().Str("stream", k).Msg("[vision] debug autostart worker")
			startVisionStreamWorker(k)
		}
	}()
}
