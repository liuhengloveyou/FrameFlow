package vision

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"sync"
	"time"

	"github.com/liuhengloveyou/FrameFlow/internal/streams"
	"github.com/liuhengloveyou/FrameFlow/pkg/core"
	"github.com/liuhengloveyou/FrameFlow/pkg/fferrors"
	"github.com/liuhengloveyou/FrameFlow/pkg/magic"
	"github.com/liuhengloveyou/FrameFlow/pkg/mjpeg"
)

// 取帧 / 检测节拍由绑定规则提供，见 resolvePullSettings。

var (
	workerMu   sync.Mutex
	workerStop = map[string]context.CancelFunc{} // streamKey -> cancel stream loop
)

func visionPullWaitTimeout(interval time.Duration) time.Duration {
	if interval <= 0 {
		interval = time.Duration(DefaultPullSampleMS) * time.Millisecond
	}
	d := interval * 8
	if d < 5*time.Second {
		d = 5 * time.Second
	}
	if d > 30*time.Second {
		d = 30 * time.Second
	}
	return d
}

// visionIntervalSampleWaitTimeout：interval 模式等首帧/下一快照；须覆盖慢 GOP（首 IDR 可能 >5s）。
func visionIntervalSampleWaitTimeout(interval time.Duration) time.Duration {
	if interval <= 0 {
		interval = time.Duration(DefaultPullSampleMS) * time.Millisecond
	}
	d := interval * 30
	if d < 15*time.Second {
		d = 15 * time.Second
	}
	if d > 60*time.Second {
		d = 60 * time.Second
	}
	return d
}

func startVisionStreamWorker(streamKey string) {
	workerMu.Lock()
	defer workerMu.Unlock()
	if _, ok := workerStop[streamKey]; ok {
		log.Debug().Str("stream", streamKey).Msg("[vision] worker already running")
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	workerStop[streamKey] = cancel
	pull := resolvePullSettings(streamKey)
	log.Info().
		Str("stream", streamKey).
		Dur("interval", pull.Interval).
		Bool("pull_keyframe_only", pull.KeyframeOnly).
		Int("pull_max_gop_mb", pull.MaxGOPBytes>>20).
		Bool("libav_decode", visionLibavDecode).
		Msg("[vision] worker started (interval from DB binding)")

	logAIPipeline(streamKey, "worker_start", true, map[string]any{
		"pull_sample_ms":     int(pull.Interval / time.Millisecond),
		"pull_keyframe_only": pull.KeyframeOnly,
		"pull_max_gop_mb":    pull.MaxGOPBytes >> 20,
		"libav_decode":       visionLibavDecode,
	})

	go visionStreamLoop(ctx, streamKey)
}

func stopVisionStreamWorker(streamKey string) {
	workerMu.Lock()
	cancel := workerStop[streamKey]
	delete(workerStop, streamKey)
	workerMu.Unlock()
	if cancel != nil {
		cancel()
		log.Info().Str("stream", streamKey).Msg("[vision] worker stopped")
	}
}

type pullSettings struct {
	KeyframeOnly bool
	Interval     time.Duration // 取帧节流 + 检测循环最小间隔（同一配置）
	MaxGOPBytes  int
}

// resolvePullSettings 从绑定 pull_sample_ms 读取检测间隔；无绑定则用默认。
func resolvePullSettings(streamKey string) pullSettings {
	d := DefaultStreamAIPolicy()
	out := pullSettings{
		KeyframeOnly: d.PullKeyframeOnly,
		Interval:     time.Duration(d.PullSampleMS) * time.Millisecond,
		MaxGOPBytes:  d.PullMaxGOPBytes,
	}
	p, ok, has := LookupStreamAIPolicy(streamKey)
	if !has || !ok || !p.Enabled {
		return out
	}
	out.KeyframeOnly = p.PullKeyframeOnly
	if p.PullSampleMS >= 50 {
		out.Interval = time.Duration(p.PullSampleMS) * time.Millisecond
	}
	if p.PullMaxGOPBytes >= 1<<20 {
		out.MaxGOPBytes = p.PullMaxGOPBytes
	}
	return out
}

func visionStreamLoop(ctx context.Context, streamKey string) {
	var lastIterStart time.Time
	var ivCons *magic.IntervalSampleConsumer
	var ivStream *streams.Stream
	var ivSample time.Duration
	var ivMaxGOP int
	var lav *libavVisionSession
	if visionLibavDecode {
		lav = newLibavVisionSession()
	}
	defer func() {
		if lav != nil {
			lav.close()
		}
		if ivCons != nil && ivStream != nil {
			ivStream.RemoveConsumer(ivCons)
		}
		log.Info().Str("stream", streamKey).Msg("[vision] stream loop exit")
	}()

	pull0 := resolvePullSettings(streamKey)
	mode := "interval"
	if pull0.KeyframeOnly {
		mode = "keyframe"
	}
	log.Info().
		Str("stream", streamKey).
		Str("mode", mode).
		Dur("interval", pull0.Interval).
		Int("pull_max_gop_mb", pull0.MaxGOPBytes>>20).
		Bool("libav", lav != nil).
		Msg("[vision] stream loop enter")

	var iter uint64
	for {
		select {
		case <-ctx.Done():
			log.Info().Str("stream", streamKey).Uint64("iters", iter).Msg("[vision] stream loop cancelled")
			return
		default:
		}

		iter++
		iterStart := time.Now()
		var sinceLast time.Duration
		haveSince := false
		if !lastIterStart.IsZero() {
			sinceLast = iterStart.Sub(lastIterStart)
			haveSince = true
		}
		lastIterStart = iterStart

		pull := resolvePullSettings(streamKey)
		mode = "interval"
		if pull.KeyframeOnly {
			mode = "keyframe"
		}

		t0 := time.Now()
		var err error
		if pull.KeyframeOnly {
			if ivCons != nil && ivStream != nil {
				ivStream.RemoveConsumer(ivCons)
				ivCons = nil
				ivStream = nil
			}
			err = processAndPublishKeyframe(streamKey, lav)
		} else {
			err = processAndPublishInterval(streamKey, ctx, &ivCons, &ivStream, &ivSample, &ivMaxGOP, pull.Interval, pull.MaxGOPBytes, lav)
		}
		processDur := time.Since(t0)
		logKeyframeOutcome(streamKey, err)

		wait := pull.Interval - processDur
		if wait < 0 {
			wait = 0
		}
		// 拉不到关键帧或其它失败时至少隔一整拍再试，避免空转占满 CPU。
		if err != nil && wait < pull.Interval {
			wait = pull.Interval
		}

		ev := log.Info().
			Str("stream", streamKey).
			Str("mode", mode).
			Uint64("iter", iter).
			Dur("work", processDur).
			Dur("sleep", wait).
			Dur("interval", pull.Interval)
		if haveSince {
			ev = ev.Dur("period", sinceLast)
		}
		if err != nil {
			ev = ev.Err(err).Bool("ok", false)
		} else {
			ev = ev.Bool("ok", true)
		}
		ev.Msg("[vision] tick")

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			log.Info().Str("stream", streamKey).Uint64("iters", iter).Msg("[vision] stream loop cancelled")
			return
		case <-timer.C:
		}
	}
}

func logKeyframeOutcome(streamKey string, err error) {
	if err == nil {
		return
	}
	if errors.Is(err, fferrors.ErrVisionNoKeyframe) {
		log.Debug().Err(err).Str("stream", streamKey).Msg("[vision] keyframe")
		return
	}
	log.Warn().Err(err).Str("stream", streamKey).Msg("[vision] keyframe")
}

type visionCodecSource interface {
	CodecName() string
	LastRTP() (ts uint32, ok bool)
}

func visionPublishDetected(streamKey string, src visionCodecSource, boxes []VisionBBox, frameText string,
	keyframeUnixMS int64, durGet, durPull, durJpeg, durDet time.Duration, frameBytes int) error {
	payload := VisionFramePayload{
		Type:           streams.VisionDataChannelLabel,
		V:              1,
		Stream:         streamKey,
		Event:          "frame",
		UnixMS:         time.Now().UnixMilli(),
		KeyframeUnixMS: keyframeUnixMS,
		Text:           frameText,
		Boxes:          boxes,
	}
	if ts, ok := src.LastRTP(); ok {
		v := int64(ts)
		payload.RtpTimestamp = &v
	}
	t := time.Now()
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Error().Err(err).Str("stream", streamKey).Msg("[vision] marshal payload")
		return err
	}
	streams.BroadcastStreamData(streamKey, payloadBytes)
	durPub := time.Since(t)

	if log.Debug().Enabled() {
		sum := durGet + durPull + durJpeg + durDet + durPub
		log.Debug().
			Str("stream", streamKey).
			Dur("get", durGet).
			Dur("kf", durPull).
			Dur("jpeg", durJpeg).
			Dur("det", durDet).
			Dur("pub", durPub).
			Dur("sum", sum).
			Int("jpeg_bytes", frameBytes).
			Msg("[vision] stages")
	}
	if len(boxes) > 0 {
		log.Info().
			Str("stream", streamKey).
			Int("boxes", len(boxes)).
			Str("text", frameText).
			Int("payload_bytes", len(payloadBytes)).
			Msg("[vision] broadcast frame (check WebRTC DataChannel label=frameflow-vision)")
	} else {
		log.Debug().
			Str("stream", streamKey).
			Str("text", frameText).
			Int("payload_bytes", len(payloadBytes)).
			Msg("[vision] broadcast frame (empty)")
	}
	return nil
}

func visionFinishSample(streamKey string, src visionCodecSource, b []byte, durGet, durPull time.Duration, lav *libavVisionSession) error {
	t0 := time.Now()
	logAIPipeline(streamKey, "0_sample", false, map[string]any{
		"codec":      src.CodecName(),
		"sample_len": len(b),
		"dur_get":    durGet,
		"dur_pull":   durPull,
	})
	switch src.CodecName() {
	case core.CodecH264, core.CodecH265:
		if !visionLibavDecode {
			log.Warn().Str("stream", streamKey).Str("codec", src.CodecName()).Msg("[vision] H264/H265 requires CGO+libavcodec build")
			return fferrors.ErrVisionLibavRequired
		}
		if lav == nil {
			return fferrors.ErrVisionLibavRequired
		}
		rgb, w, h, ok, errL := lav.tryDecodeAnnexB(src.CodecName(), b)
		if errL != nil {
			logAIPipeline(streamKey, "0_decode_err", true, map[string]any{"err": errL.Error(), "codec": src.CodecName()})
			log.Warn().Err(errL).Str("stream", streamKey).Str("codec", src.CodecName()).Msg("[vision] libav decode")
			return fferrors.ErrVisionLibavDecode
		}
		if !ok || len(rgb) == 0 {
			logAIPipeline(streamKey, "0_decode_no_frame", false, map[string]any{"codec": src.CodecName(), "sample_len": len(b)})
			log.Warn().Str("stream", streamKey).Str("codec", src.CodecName()).Msg("[vision] libav no frame")
			return fferrors.ErrVisionLibavNoFrame
		}
		logAIPipeline(streamKey, "0_decode_ok", false, map[string]any{
			"w": w, "h": h, "rgb_bytes": len(rgb), "dur_decode": time.Since(t0),
		})
		durJpeg := time.Since(t0)
		keyframeUnixMS := time.Now().UnixMilli()
		t1 := time.Now()
		boxes, frameText, err := boxesFromKeyframeRGBStream(streamKey, rgb, w, h)
		durDet := time.Since(t1)
		if err != nil {
			if errors.Is(err, fferrors.ErrVisionInferBusy) {
				log.Debug().Str("stream", streamKey).Dur("det_wait", durDet).
					Msg("[vision] skip frame: detector busy")
				return nil
			}
			log.Warn().Err(err).Str("stream", streamKey).Msg("[vision] detect boxes")
			return err
		}
		if len(boxes) > 0 {
			pol, _, _ := LookupStreamAIPolicy(streamKey)
			var jpegBytes []byte
			if img, errEnc := rgbToJPEG(rgb, w, h); errEnc == nil {
				jpegBytes = img
			}
			logAIPipeline(streamKey, "4_notify", true, map[string]any{
				"boxes":   len(boxes),
				"text":    frameText,
				"bind_id": pol.BindID,
			})
			NotifyDetection(DetectionEvent{
				StreamKey: streamKey,
				Boxes:     boxes,
				JPEG:      jpegBytes,
				Policy:    pol,
			})
		} else {
			logAIPipeline(streamKey, "4_publish_empty", false, map[string]any{
				"text":    frameText,
				"dur_det": durDet,
			})
		}
		return visionPublishDetected(streamKey, src, boxes, frameText, keyframeUnixMS, durGet, durPull, durJpeg, durDet, len(rgb))
	case core.CodecJPEG:
		b = mjpeg.FixJPEG(b)
	default:
		log.Warn().Str("stream", streamKey).Str("codec", src.CodecName()).Msg("[vision] unsupported codec")
		return fferrors.ErrVisionCodec
	}
	durJpeg := time.Since(t0)

	keyframeUnixMS := time.Now().UnixMilli()
	t := time.Now()
	boxes, frameText, err := boxesFromKeyframeJPEGStream(streamKey, b)
	durDet := time.Since(t)
	if err != nil {
		if errors.Is(err, fferrors.ErrVisionInferBusy) {
			log.Debug().Str("stream", streamKey).Dur("det_wait", durDet).
				Msg("[vision] skip frame: detector busy")
			return nil
		}
		log.Warn().Err(err).Str("stream", streamKey).Msg("[vision] detect boxes")
		return err
	}
	if len(boxes) > 0 {
		pol, _, _ := LookupStreamAIPolicy(streamKey)
		logAIPipeline(streamKey, "4_notify", true, map[string]any{
			"boxes":   len(boxes),
			"text":    frameText,
			"bind_id": pol.BindID,
		})
		NotifyDetection(DetectionEvent{
			StreamKey: streamKey,
			Boxes:     append([]VisionBBox(nil), boxes...),
			JPEG:      append([]byte(nil), b...),
			Policy:    pol,
		})
	}

	return visionPublishDetected(streamKey, src, boxes, frameText, keyframeUnixMS, durGet, durPull, durJpeg, durDet, len(b))
}

func processAndPublishKeyframe(streamKey string, lav *libavVisionSession) error {
	q := url.Values{"src": {streamKey}}
	t := time.Now()
	stream, err := streams.GetOrPatch(q)
	durGet := time.Since(t)
	if err != nil {
		log.Warn().Err(err).Str("stream", streamKey).Msg("[vision] get or patch stream")
		return err
	}
	if stream == nil {
		log.Warn().Str("stream", streamKey).Msg("[vision] stream is nil")
		return fferrors.ErrVisionStreamNil
	}

	kfCons := magic.NewVideoKeyframeConsumer()
	if err := stream.AddConsumer(kfCons); err != nil {
		log.Warn().Err(err).Str("stream", streamKey).Msg("[vision] add keyframe consumer")
		return err
	}

	t = time.Now()
	once := &core.OnceBuffer{}
	_, _ = kfCons.WriteTo(once)
	b := once.Buffer()
	stream.RemoveConsumer(kfCons)
	durKf := time.Since(t)

	if len(b) == 0 {
		return fferrors.ErrVisionNoKeyframe
	}

	return visionFinishSample(streamKey, kfCons, b, durGet, durKf, lav)
}

func processAndPublishInterval(streamKey string, ctx context.Context, ivCons **magic.IntervalSampleConsumer, ivStream **streams.Stream, ivSample *time.Duration, ivMaxGOP *int, interval time.Duration, maxGOPBytes int, lav *libavVisionSession) error {
	q := url.Values{"src": {streamKey}}
	t := time.Now()
	stream, err := streams.GetOrPatch(q)
	durGet := time.Since(t)
	if err != nil {
		log.Warn().Err(err).Str("stream", streamKey).Msg("[vision] get or patch stream")
		return err
	}
	if stream == nil {
		log.Warn().Str("stream", streamKey).Msg("[vision] stream is nil")
		return fferrors.ErrVisionStreamNil
	}

	needNew := *ivCons == nil || *ivSample != interval || *ivMaxGOP != maxGOPBytes
	if needNew {
		if *ivCons != nil && *ivStream != nil {
			(*ivStream).RemoveConsumer(*ivCons)
			*ivCons = nil
			*ivStream = nil
		}
		c := magic.NewIntervalSampleConsumer(interval, maxGOPBytes)
		if err := stream.AddConsumer(c); err != nil {
			log.Warn().Err(err).Str("stream", streamKey).Msg("[vision] add interval consumer")
			return err
		}
		*ivCons = c
		*ivStream = stream
		*ivSample = interval
		*ivMaxGOP = maxGOPBytes
	}

	waitCtx, cancel := context.WithTimeout(ctx, visionIntervalSampleWaitTimeout(interval))
	defer cancel()

	t = time.Now()
	b, err := (*ivCons).WaitSample(waitCtx)
	durPull := time.Since(t)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			if errors.Is(err, context.DeadlineExceeded) && len((*ivCons).Senders) > 0 {
				drops := (*ivCons).Senders[0].Drops
				if drops > 0 {
					log.Warn().
						Str("stream", streamKey).
						Int("sender_drops", drops).
						Msg("[vision] interval sample timeout (upstream may be dropping packets; see core.Sender buffer)")
				}
			}
			return fferrors.ErrVisionNoKeyframe
		}
		log.Warn().Err(err).Str("stream", streamKey).Msg("[vision] interval sample")
		return err
	}
	if len(b) == 0 {
		return fferrors.ErrVisionNoKeyframe
	}

	return visionFinishSample(streamKey, *ivCons, b, durGet, durPull, lav)
}
