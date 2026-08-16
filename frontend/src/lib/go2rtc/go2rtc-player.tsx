import {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useRef,
} from 'react'
import { VideoRTC } from '@/lib/go2rtc/video-rtc'
import { buildWsSrc } from '@/lib/preview-source'
import {
  humanizeStreamError,
  STREAM_CONNECT_TIMEOUT_HINT,
  StreamErrors,
} from '@/lib/stream-error'
import type { VisionFramePayload } from '@/lib/vision-types'

const ELEMENT = 'video-rtc'
/** 无首帧则视为设备/码流未就绪 */
const CONNECT_TIMEOUT_MS = 12_000

let defined = false
function ensureDefined() {
  if (defined || typeof window === 'undefined') return
  if (!customElements.get(ELEMENT)) {
    customElements.define(ELEMENT, VideoRTC)
  }
  defined = true
}

export type StreamPlayStats = {
  mode: 'WebRTC' | 'MSE'
  bitrateKbps: number
  bandwidthKBps: number
  latencySec: number | null
  framesReceived: number
  framesDecoded: number
  framesDropped: number
  dropRate: number
  width: number
  height: number
}

export type Go2rtcPlayerHandle = {
  capturePng: () => string | null
  getVideo: () => HTMLVideoElement | null
}

type Go2rtcPlayerProps = {
  streamSrc: string
  mode: 'webrtc' | 'normal'
  muted?: boolean
  className?: string
  onState?: (state: 'connecting' | 'playing' | 'error' | 'idle') => void
  /** 面向用户的失败/等待说明 */
  onError?: (message: string) => void
  onStats?: (stats: StreamPlayStats) => void
  /** WebRTC DataChannel `frameflow-vision` 推送的检测帧 */
  onVisionFrame?: (payload: VisionFramePayload) => void
}

/**
 * Thin React wrapper around go2rtc VideoRTC (WebSocket → WebRTC / MSE).
 */
export const Go2rtcPlayer = forwardRef<Go2rtcPlayerHandle, Go2rtcPlayerProps>(
  function Go2rtcPlayer(
    {
      streamSrc,
      mode,
      muted = true,
      className,
      onState,
      onError,
      onStats,
      onVisionFrame,
    },
    ref,
  ) {
    const hostRef = useRef<HTMLDivElement>(null)
    const playerRef = useRef<VideoRTC | null>(null)
    const mutedRef = useRef(muted)
    const onStateRef = useRef(onState)
    const onErrorRef = useRef(onError)
    const onStatsRef = useRef(onStats)
    const onVisionRef = useRef(onVisionFrame)
    mutedRef.current = muted
    onStateRef.current = onState
    onErrorRef.current = onError
    onStatsRef.current = onStats
    onVisionRef.current = onVisionFrame

    useImperativeHandle(ref, () => ({
      getVideo: () => playerRef.current?.video ?? null,
      capturePng: () => {
        const v = playerRef.current?.video
        if (!v || v.videoWidth <= 0 || v.videoHeight <= 0) return null
        try {
          const canvas = document.createElement('canvas')
          canvas.width = v.videoWidth
          canvas.height = v.videoHeight
          const ctx = canvas.getContext('2d')
          if (!ctx) return null
          ctx.drawImage(v, 0, 0)
          return canvas.toDataURL('image/png')
        } catch {
          return null
        }
      },
    }))

    useEffect(() => {
      ensureDefined()
      const host = hostRef.current
      if (!host || !streamSrc) return

      const el = document.createElement(ELEMENT) as VideoRTC
      el.className = 'ff-go2rtc-player'
      el.background = true
      el.visibilityCheck = false
      el.mode = mode === 'webrtc' ? 'webrtc,mse' : 'mse,webrtc,hls'
      el.media = 'video,audio'
      host.replaceChildren(el)
      playerRef.current = el

      let gotPlaying = false
      const reportError = (raw: string) => {
        const msg = humanizeStreamError(raw)
        onErrorRef.current?.(msg)
        onStateRef.current?.('error')
      }

      onStateRef.current?.('connecting')
      const elVision = el as VideoRTC & {
        onVisionFrame?: (payload: VisionFramePayload) => void
      }
      elVision.onVisionFrame = (payload) => {
        onVisionRef.current?.(payload)
      }
      el.src = buildWsSrc(streamSrc)

      // 拦截服务端 error（如 device offline）
      const origOnopen = el.onopen.bind(el)
      el.onopen = () => {
        const modes = origOnopen()
        const bag = el as VideoRTC & {
          onmessage?: Record<
            string,
            (msg: { type?: string; value?: unknown }) => void
          >
        }
        if (!bag.onmessage) bag.onmessage = {}
        bag.onmessage['ff-user-error'] = (msg) => {
          if (msg?.type !== 'error') return
          const raw = String(msg.value ?? '')
          if (!gotPlaying) reportError(raw)
        }
        return modes
      }

      const applyVideo = () => {
        const v = el.video
        if (!v) return
        v.controls = false
        v.muted = true
        v.playsInline = true
        v.autoplay = true
        v.style.display = 'block'
        v.style.width = '100%'
        v.style.height = '100%'
        v.style.objectFit = 'contain'
        v.style.objectPosition = 'center'
        v.style.background = '#0a0a0a'
      }

      const mo = new MutationObserver(() => applyVideo())
      mo.observe(el, { childList: true })
      applyVideo()

      const onPlaying = () => {
        gotPlaying = true
        const v = el.video
        if (v) v.muted = mutedRef.current
        onStateRef.current?.('playing')
      }
      const onWaiting = () => {
        const v = el.video
        if (v && v.videoWidth > 0 && !v.paused) return
        if (!gotPlaying) onStateRef.current?.('connecting')
      }
      const onVideoError = () => {
        const v = el.video
        const err = v?.error
        if (err?.code === MediaError.MEDIA_ERR_SRC_NOT_SUPPORTED) {
          if (!v?.src && !v?.srcObject) return
        }
        if (!gotPlaying) {
          reportError(`${StreamErrors.PLAY_FAILED.code} ${StreamErrors.PLAY_FAILED.message}`)
        }
      }

      const bindVideoEvents = () => {
        const v = el.video
        if (!v) return
        v.addEventListener('playing', onPlaying)
        v.addEventListener('waiting', onWaiting)
        v.addEventListener('error', onVideoError)
      }
      const bindMo = new MutationObserver(() => bindVideoEvents())
      bindMo.observe(el, { childList: true })
      bindVideoEvents()

      const connectTimer = window.setTimeout(() => {
        if (gotPlaying) return
        const v = el.video
        if (v && v.videoWidth > 0) return
        reportError(
          `${StreamErrors.CONNECT_TIMEOUT.code} ${STREAM_CONNECT_TIMEOUT_HINT}`,
        )
      }, CONNECT_TIMEOUT_MS)

      const kickTimer = window.setInterval(() => {
        const v = el.video
        if (!v) return
        if ((v.srcObject || v.src) && v.paused && v.readyState >= 2) {
          void v.play().catch(() => {
            v.muted = true
            void v.play().catch(() => {})
          })
        }
      }, 1500)

      let prevBytes = 0
      let prevAt = 0
      const statsTimer = window.setInterval(() => {
        void (async () => {
          const v = el.video
          const width = v?.videoWidth ?? 0
          const height = v?.videoHeight ?? 0
          if (v && width > 0 && !v.paused && !v.ended) {
            gotPlaying = true
            onStateRef.current?.('playing')
          }
          const playMode: 'WebRTC' | 'MSE' = el.pc ? 'WebRTC' : 'MSE'

          let bitrateKbps = 0
          let bandwidthKBps = 0
          let latencySec: number | null = null
          let framesReceived = 0
          let framesDecoded = 0
          let framesDropped = 0

          const pc = el.pc
          if (pc) {
            try {
              const report = await pc.getStats()
              let bytes = 0
              let rttSec: number | null = null
              report.forEach((r) => {
                if (r.type === 'inbound-rtp') {
                  const s = r as RTCInboundRtpStreamStats & {
                    mediaType?: string
                  }
                  const isVideo =
                    s.kind === 'video' ||
                    s.mediaType === 'video' ||
                    ((s.kind == null || s.kind === '') &&
                      (s.framesDecoded != null || s.framesReceived != null))
                  if (!isVideo) return
                  bytes += s.bytesReceived ?? 0
                  framesReceived += s.framesReceived ?? 0
                  framesDecoded += s.framesDecoded ?? 0
                  framesDropped += s.framesDropped ?? 0
                  const delay = s.jitterBufferDelay
                  const emitted = s.jitterBufferEmittedCount
                  if (
                    delay != null &&
                    emitted != null &&
                    emitted > 0 &&
                    latencySec == null
                  ) {
                    latencySec = delay / emitted
                  }
                }
                if (r.type === 'candidate-pair') {
                  const pair = r as RTCIceCandidatePairStats & {
                    selected?: boolean
                  }
                  if (
                    pair.state === 'succeeded' &&
                    pair.currentRoundTripTime != null &&
                    (pair.selected || pair.nominated || rttSec == null)
                  ) {
                    rttSec = pair.currentRoundTripTime
                  }
                }
              })
              if (latencySec == null && rttSec != null) latencySec = rttSec
              const now = performance.now()
              if (prevAt > 0 && now > prevAt && bytes >= prevBytes) {
                const deltaBytes = bytes - prevBytes
                const dtMs = now - prevAt
                bitrateKbps = Math.round((deltaBytes * 8) / dtMs)
                bandwidthKBps = deltaBytes / dtMs
              }
              prevBytes = bytes
              prevAt = now
            } catch {
              /* ignore stats errors */
            }
          }

          const dropRate =
            framesReceived > 0
              ? (framesDropped / framesReceived) * 100
              : 0
          onStatsRef.current?.({
            mode: playMode,
            bitrateKbps,
            bandwidthKBps,
            latencySec,
            framesReceived,
            framesDecoded,
            framesDropped,
            dropRate,
            width,
            height,
          })
        })()
      }, 1000)

      return () => {
        window.clearTimeout(connectTimer)
        window.clearInterval(kickTimer)
        window.clearInterval(statsTimer)
        mo.disconnect()
        bindMo.disconnect()
        const v = el.video
        if (v) {
          v.removeEventListener('playing', onPlaying)
          v.removeEventListener('waiting', onWaiting)
          v.removeEventListener('error', onVideoError)
        }
        try {
          el.ondisconnect?.()
        } catch {
          /* ignore */
        }
        host.replaceChildren()
        playerRef.current = null
        onStateRef.current?.('idle')
      }
    }, [streamSrc, mode])

    useEffect(() => {
      const v = playerRef.current?.video
      if (v) v.muted = muted
    }, [muted])

    return <div ref={hostRef} className={className} />
  },
)
