import {
  useEffect,
  useRef,
  useState,
  type PointerEvent as ReactPointerEvent,
} from 'react'
import type { PlaybackSegment } from './playback-utils'
import {
  dayBounds,
  findSegmentAt,
  formatClock,
  parseLocalDateTimeMs,
} from './playback-utils'

type Props = {
  day: string
  segments: PlaybackSegment[]
  cursorMs: number | null
  activeSegId?: string | null
  onSeek: (tMs: number, seg: PlaybackSegment | null) => void
}

/** 参考 vis-timeline：滚轮以指针为中心缩放、拖拽平移、游标、自适应刻度 */
const ZOOM_MIN_MS = 60_000
const ZOOM_MAX_MS = 24 * 60 * 60 * 1000
const AXIS_H = 22
const TRACK_H = 36

type Tick = { ms: number; major: boolean; label: string }

function clamp(n: number, min: number, max: number) {
  return Math.min(max, Math.max(min, n))
}

function chooseStep(spanMs: number, widthPx: number): { minor: number; major: number } {
  const target = spanMs / Math.max(1, widthPx / 90)
  const steps = [
    1_000, 2_000, 5_000, 10_000, 15_000, 30_000, 60_000, 2 * 60_000, 5 * 60_000,
    10 * 60_000, 15 * 60_000, 30 * 60_000, 60 * 60_000, 2 * 60 * 60_000,
    3 * 60 * 60_000, 6 * 60 * 60_000, 12 * 60 * 60_000,
  ]
  let minor = steps[steps.length - 1]
  for (const s of steps) {
    if (s >= target) {
      minor = s
      break
    }
  }
  const major =
    minor <= 60_000
      ? Math.max(minor * 5, 60_000)
      : minor <= 15 * 60_000
        ? Math.max(minor * 4, 60 * 60_000)
        : Math.max(minor * 2, 6 * 60 * 60_000)
  return { minor, major }
}

function buildTicks(viewStart: number, viewEnd: number, widthPx: number): Tick[] {
  const span = Math.max(1, viewEnd - viewStart)
  const { minor, major } = chooseStep(span, widthPx)
  const first = Math.floor(viewStart / minor) * minor
  const ticks: Tick[] = []
  for (let t = first; t <= viewEnd + minor; t += minor) {
    if (t < viewStart - minor || t > viewEnd + minor) continue
    const isMajor = Math.abs(t % major) < 1 || Math.abs((t % major) - major) < 1
    const d = new Date(t)
    const hh = String(d.getHours()).padStart(2, '0')
    const mm = String(d.getMinutes()).padStart(2, '0')
    const ss = String(d.getSeconds()).padStart(2, '0')
    let label = `${hh}:${mm}`
    if (minor < 60_000) label = `${hh}:${mm}:${ss}`
    else if (minor >= 60 * 60_000 && d.getMinutes() === 0) label = `${hh}:00`
    ticks.push({ ms: t, major: isMajor, label: isMajor ? label : '' })
  }
  return ticks
}

function formatWindow(startMs: number, endMs: number): string {
  return `${formatClock(startMs)} – ${formatClock(endMs)}`
}

export function PlaybackTimeline({
  day,
  segments,
  cursorMs,
  activeSegId,
  onSeek,
}: Props) {
  const rootRef = useRef<HTMLDivElement>(null)
  const bounds = dayBounds(day)
  const dayStart = parseLocalDateTimeMs(bounds.start)
  const dayEnd = parseLocalDateTimeMs(bounds.end) + 999
  const daySpan = Math.max(1, dayEnd - dayStart)

  const [viewStart, setViewStart] = useState(dayStart)
  const [viewEnd, setViewEnd] = useState(dayEnd)
  const [width, setWidth] = useState(640)
  const [hoverMs, setHoverMs] = useState<number | null>(null)

  const viewRef = useRef({ start: dayStart, end: dayEnd, width: 640 })
  viewRef.current = { start: viewStart, end: viewEnd, width }

  const dragRef = useRef<{
    kind: 'pan' | 'cursor'
    originX: number
    originStart: number
    originEnd: number
    moved: boolean
  } | null>(null)

  useEffect(() => {
    setViewStart(dayStart)
    setViewEnd(dayEnd)
  }, [dayStart, dayEnd])

  useEffect(() => {
    const el = rootRef.current
    if (!el) return
    const ro = new ResizeObserver((entries) => {
      const w = entries[0]?.contentRect.width
      if (w && w > 0) setWidth(Math.floor(w))
    })
    ro.observe(el)
    setWidth(Math.floor(el.clientWidth) || 640)
    return () => ro.disconnect()
  }, [])

  function applyWindow(nextStart: number, nextEnd: number) {
    const { start: ds, end: de } = { start: dayStart, end: dayEnd }
    const maxSpan = Math.min(ZOOM_MAX_MS, daySpan)
    let s = nextStart
    let e = nextEnd
    let w = clamp(e - s, ZOOM_MIN_MS, maxSpan)
    const mid = (s + e) / 2
    s = mid - w / 2
    e = mid + w / 2
    if (s < ds) {
      e += ds - s
      s = ds
    }
    if (e > de) {
      s -= e - de
      e = de
    }
    s = clamp(s, ds, de - ZOOM_MIN_MS)
    e = clamp(e, s + ZOOM_MIN_MS, de)
    setViewStart(s)
    setViewEnd(e)
  }

  function msToX(ms: number, start = viewStart, end = viewEnd, w = width) {
    return ((ms - start) / Math.max(1, end - start)) * w
  }

  function xToMs(x: number, start = viewStart, end = viewEnd, w = width) {
    return start + (x / Math.max(1, w)) * Math.max(1, end - start)
  }

  function zoomAtRatio(ratio: number, factor: number) {
    const { start, end } = viewRef.current
    const span = Math.max(1, end - start)
    const anchor = start + clamp(ratio, 0, 1) * span
    const nextSpan = clamp(
      span * factor,
      ZOOM_MIN_MS,
      Math.min(ZOOM_MAX_MS, daySpan),
    )
    applyWindow(anchor - ratio * nextSpan, anchor - ratio * nextSpan + nextSpan)
  }

  useEffect(() => {
    const el = rootRef.current
    if (!el) return
    const onWheel = (e: WheelEvent) => {
      e.preventDefault()
      const rect = el.getBoundingClientRect()
      const { start, end, width: w } = viewRef.current
      const span = Math.max(1, end - start)
      if (e.shiftKey) {
        const delta = (e.deltaY || e.deltaX) * span * 0.0015
        applyWindow(start + delta, end + delta)
        return
      }
      const ratio = clamp((e.clientX - rect.left) / Math.max(1, w), 0, 1)
      zoomAtRatio(ratio, e.deltaY > 0 ? 1.15 : 1 / 1.15)
    }
    el.addEventListener('wheel', onWheel, { passive: false })
    return () => el.removeEventListener('wheel', onWheel)
  }, [dayStart, dayEnd, daySpan])

  function seekAtClientX(clientX: number) {
    const el = rootRef.current
    if (!el) return
    const rect = el.getBoundingClientRect()
    const { start, end, width: w } = viewRef.current
    const tMs = clamp(xToMs(clientX - rect.left, start, end, w), dayStart, dayEnd)
    onSeek(tMs, findSegmentAt(segments, tMs))
  }

  function onPointerDown(e: ReactPointerEvent<HTMLDivElement>) {
    if (e.button !== 0) return
    const el = rootRef.current
    if (!el) return
    const rect = el.getBoundingClientRect()
    const x = e.clientX - rect.left
    const nearCursor =
      cursorMs != null && Math.abs(msToX(cursorMs) - x) <= 6
    el.setPointerCapture(e.pointerId)
    dragRef.current = {
      kind: nearCursor ? 'cursor' : 'pan',
      originX: e.clientX,
      originStart: viewStart,
      originEnd: viewEnd,
      moved: false,
    }
  }

  function onPointerMove(e: ReactPointerEvent<HTMLDivElement>) {
    const el = rootRef.current
    if (!el) return
    const rect = el.getBoundingClientRect()
    const { start, end, width: w } = viewRef.current
    const tMs = clamp(xToMs(e.clientX - rect.left, start, end, w), dayStart, dayEnd)
    setHoverMs(tMs)

    const drag = dragRef.current
    if (!drag) return
    const dx = e.clientX - drag.originX
    if (Math.abs(dx) > 3) drag.moved = true

    if (drag.kind === 'pan') {
      const dt =
        -(dx / Math.max(1, w)) * (drag.originEnd - drag.originStart)
      applyWindow(drag.originStart + dt, drag.originEnd + dt)
    }
  }

  function onPointerUp(e: ReactPointerEvent<HTMLDivElement>) {
    const drag = dragRef.current
    dragRef.current = null
    try {
      e.currentTarget.releasePointerCapture(e.pointerId)
    } catch {
      /* ignore */
    }
    if (!drag) return
    if (drag.kind === 'cursor' || !drag.moved) {
      seekAtClientX(e.clientX)
    }
  }

  const ticks = buildTicks(viewStart, viewEnd, width)
  const visibleSegs = segments.filter(
    (s) => s.endMs >= viewStart && s.startMs <= viewEnd,
  )

  return (
    <div className="ff-playback-timeline">
      <div className="ff-playback-timeline-toolbar">
        <span className="ff-playback-timeline-hint">
          {hoverMs != null
            ? `指针 ${formatClock(hoverMs)}`
            : cursorMs != null
              ? `定位 ${formatClock(cursorMs)}`
              : segments.length
                ? `共 ${segments.length} 段 · 滚轮缩放 · 拖拽平移`
                : '当日无录像'}
        </span>
      </div>

      <div
        ref={rootRef}
        className="ff-playback-timeline-canvas"
        role="slider"
        aria-label="回放时间轴"
        aria-valuemin={0}
        aria-valuemax={24}
        tabIndex={0}
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={onPointerUp}
        onPointerCancel={onPointerUp}
        onPointerLeave={() => {
          if (!dragRef.current) setHoverMs(null)
        }}
        onDoubleClick={() => applyWindow(dayStart, dayEnd)}
      >
        <div className="ff-playback-timeline-axis" style={{ height: AXIS_H }}>
          {ticks.map((t) => {
            const left = msToX(t.ms)
            if (left < -20 || left > width + 20) return null
            return (
              <div
                key={t.ms}
                className="ff-playback-timeline-tick"
                data-major={t.major ? 'true' : undefined}
                style={{ left }}
              >
                <i />
                {t.label ? <span>{t.label}</span> : null}
              </div>
            )
          })}
        </div>

        <div className="ff-playback-timeline-track" style={{ height: TRACK_H }}>
          {visibleSegs.map((s) => {
            const left = msToX(s.startMs)
            const right = msToX(s.endMs)
            return (
              <div
                key={s.id}
                className="ff-playback-timeline-seg"
                data-active={activeSegId === s.id ? 'true' : undefined}
                style={{ left, width: Math.max(2, right - left) }}
                title={`${formatClock(s.startMs)} – ${formatClock(s.endMs)}`}
              />
            )
          })}

          {hoverMs != null ? (
            <div
              className="ff-playback-timeline-hover"
              style={{ left: msToX(hoverMs) }}
            />
          ) : null}

          {cursorMs != null ? (
            <div
              className="ff-playback-timeline-cursor"
              style={{ left: msToX(cursorMs) }}
            />
          ) : null}
        </div>
      </div>

      <div className="ff-playback-timeline-footer">
        <span>{formatWindow(viewStart, viewEnd)}</span>
        <span>Shift+滚轮平移 · 双击复位全天</span>
      </div>
    </div>
  )
}
