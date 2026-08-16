import { useEffect, useMemo, useRef, useState } from 'react'
import type { ReviewEvent, ReviewEventKind } from '@/types/api'
import { formatAbsoluteShort, formatRelativeTime } from './review-time'
import {
  REVIEW_ZOOM_LEVELS,
  SEGMENT_PX,
  buildTimelineSegments,
  nearestSegmentStart,
  type TimelineSegment,
} from './review-timeline-utils'

const OVERSCAN = 24

function formatHandlebar(sec: number) {
  const d = new Date(sec * 1000)
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

/**
 * Frigate 式核查时间轴：事件密度色条、悬停预览、可拖游标、缩放。
 */
export function ReviewTimeline({
  events,
  preferKind,
  startMs,
  endMs,
  activeTimeMs,
  onSeek,
}: {
  events: ReviewEvent[]
  preferKind?: ReviewEventKind
  startMs: number
  endMs: number
  activeTimeMs: number | null
  onSeek: (payload: {
    timeMs: number
    eventId: string | null
    scrubbing?: boolean
  }) => void
}) {
  const trackRef = useRef<HTMLDivElement>(null)
  const [zoomIndex, setZoomIndex] = useState(0)
  const [dragging, setDragging] = useState(false)
  const [visible, setVisible] = useState({ start: 0, end: 120 })
  const [hover, setHover] = useState<{
    seg: TimelineSegment
    y: number
  } | null>(null)
  const dragRef = useRef(false)
  const syncLock = useRef(false)
  const hoverTimer = useRef<number | null>(null)

  const zoom = REVIEW_ZOOM_LEVELS[zoomIndex] ?? REVIEW_ZOOM_LEVELS[0]

  const segments = useMemo(
    () =>
      buildTimelineSegments({
        events,
        segmentDuration: zoom.segmentDuration,
        timestampSpread: zoom.timestampSpread,
        startMs,
        endMs,
        preferKind,
      }),
    [events, zoom.segmentDuration, zoom.timestampSpread, startMs, endMs, preferKind],
  )

  const handlebarSec = useMemo(() => {
    if (activeTimeMs == null) return null
    return nearestSegmentStart(activeTimeMs, zoom.segmentDuration)
  }, [activeTimeMs, zoom.segmentDuration])

  const handlebarIndex = useMemo(() => {
    if (handlebarSec == null) return -1
    return segments.findIndex((s) => s.startSec === handlebarSec)
  }, [handlebarSec, segments])

  const handlebarTop =
    handlebarIndex >= 0 ? handlebarIndex * SEGMENT_PX + SEGMENT_PX / 2 : null

  const updateVisible = () => {
    const el = trackRef.current
    if (!el) return
    const start = Math.max(0, Math.floor(el.scrollTop / SEGMENT_PX) - OVERSCAN)
    const end = Math.min(
      segments.length,
      Math.ceil((el.scrollTop + el.clientHeight) / SEGMENT_PX) + OVERSCAN,
    )
    setVisible((prev) =>
      prev.start === start && prev.end === end ? prev : { start, end },
    )
  }

  useEffect(() => {
    updateVisible()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [segments.length, zoomIndex, startMs, endMs])

  useEffect(() => {
    if (dragging || syncLock.current || handlebarTop == null) return
    const el = trackRef.current
    if (!el) return
    const viewTop = el.scrollTop
    const viewBottom = viewTop + el.clientHeight
    if (handlebarTop < viewTop + 24 || handlebarTop > viewBottom - 24) {
      el.scrollTo({
        top: Math.max(0, handlebarTop - el.clientHeight / 2),
        behavior: 'smooth',
      })
    }
  }, [handlebarTop, dragging])

  const seekFromY = (clientY: number, commit: boolean) => {
    const el = trackRef.current
    if (!el) return
    const rect = el.getBoundingClientRect()
    const y = clientY - rect.top + el.scrollTop
    const idx = Math.min(
      segments.length - 1,
      Math.max(0, Math.floor(y / SEGMENT_PX)),
    )
    const seg = segments[idx]
    if (!seg) return
    const timeMs = seg.startSec * 1000
    if (commit) {
      onSeek({ timeMs, eventId: seg.jumpEventId, scrubbing: false })
    } else {
      onSeek({ timeMs, eventId: null, scrubbing: true })
    }
  }

  useEffect(() => {
    if (!dragging) return
    const onMove = (e: MouseEvent | TouchEvent) => {
      const clientY = 'touches' in e ? e.touches[0]?.clientY : e.clientY
      if (clientY == null) return
      seekFromY(clientY, false)
    }
    const onUp = (e: MouseEvent | TouchEvent) => {
      const clientY =
        'changedTouches' in e ? e.changedTouches[0]?.clientY : e.clientY
      if (clientY != null) seekFromY(clientY, true)
      dragRef.current = false
      setDragging(false)
      syncLock.current = true
      window.setTimeout(() => {
        syncLock.current = false
      }, 280)
    }
    window.addEventListener('mousemove', onMove)
    window.addEventListener('mouseup', onUp)
    window.addEventListener('touchmove', onMove, { passive: true })
    window.addEventListener('touchend', onUp)
    return () => {
      window.removeEventListener('mousemove', onMove)
      window.removeEventListener('mouseup', onUp)
      window.removeEventListener('touchmove', onMove)
      window.removeEventListener('touchend', onUp)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dragging, segments])

  const showHover = (seg: TimelineSegment, el: HTMLElement) => {
    if (!seg.previewEvent || dragging) return
    if (hoverTimer.current) window.clearTimeout(hoverTimer.current)
    hoverTimer.current = window.setTimeout(() => {
      const wrap = trackRef.current?.parentElement
      if (!wrap) return
      const wrapBox = wrap.getBoundingClientRect()
      const segBox = el.getBoundingClientRect()
      setHover({
        seg,
        y: segBox.top + segBox.height / 2 - wrapBox.top,
      })
    }, 160)
  }

  const hideHover = () => {
    if (hoverTimer.current) window.clearTimeout(hoverTimer.current)
    hoverTimer.current = window.setTimeout(() => setHover(null), 80)
  }

  const totalHeight = segments.length * SEGMENT_PX
  const slice = segments.slice(visible.start, visible.end)
  const preview = hover?.seg.previewEvent ?? null

  return (
    <aside className="ff-review-timeline" aria-label="活动时间轴">
      <div className="ff-review-timeline-legend" aria-hidden>
        <span data-tone="alert" title="警报" />
        <span data-tone="detection" title="检测" />
        <span data-tone="motion" title="变动" />
      </div>

      <div className="ff-review-timeline-track-wrap">
        <div className="ff-review-timeline-fade ff-review-timeline-fade-top" />
        <div className="ff-review-timeline-fade ff-review-timeline-fade-bottom" />

        {preview && hover ? (
          <div
            className="ff-review-timeline-preview"
            style={{ top: Math.max(8, Math.min(hover.y - 70, 9999)) }}
            onMouseEnter={() => {
              if (hoverTimer.current) window.clearTimeout(hoverTimer.current)
            }}
            onMouseLeave={hideHover}
            onClick={() => {
              onSeek({
                timeMs: hover.seg.startSec * 1000,
                eventId: preview.id,
              })
              setHover(null)
            }}
          >
            <div className="ff-review-timeline-preview-media">
              {preview.thumbUrl ? (
                <img src={preview.thumbUrl} alt="" />
              ) : (
                <div className="ff-review-card-thumb-fallback" />
              )}
              {preview.boxes?.length ? (
                <svg
                  className="ff-review-card-boxes"
                  viewBox="0 0 1 1"
                  preserveAspectRatio="none"
                >
                  {preview.boxes.map((b, i) => (
                    <rect
                      key={i}
                      x={b.x}
                      y={b.y}
                      width={b.w}
                      height={b.h}
                      className="ff-review-card-box"
                    />
                  ))}
                </svg>
              ) : null}
            </div>
            <div className="ff-review-timeline-preview-meta">
              <div className="ff-review-timeline-preview-title">{preview.title}</div>
              <div className="ff-review-timeline-preview-sub">
                {preview.cameraName} · {formatRelativeTime(preview.occurredAt)}
              </div>
              <div className="ff-review-timeline-preview-time">
                {formatAbsoluteShort(preview.occurredAt)}
              </div>
            </div>
          </div>
        ) : null}

        <div
          ref={trackRef}
          className="ff-review-timeline-track"
          data-dragging={dragging ? 'true' : undefined}
          onScroll={() => {
            updateVisible()
            setHover(null)
          }}
          onMouseDown={(e) => {
            if (e.button !== 0) return
            // 窄栏：整轨可拖，不只依赖细红线
            e.preventDefault()
            dragRef.current = true
            setDragging(true)
            setHover(null)
            seekFromY(e.clientY, false)
          }}
          onTouchStart={(e) => {
            const y = e.touches[0]?.clientY
            if (y == null) return
            dragRef.current = true
            setDragging(true)
            setHover(null)
            seekFromY(y, false)
          }}
          onClick={(e) => {
            // 拖拽结束后由 mouseup 提交；纯点击也再提交一次
            if (dragRef.current) return
            seekFromY(e.clientY, true)
          }}
        >
          <div className="ff-review-timeline-inner" style={{ height: totalHeight }}>
            {visible.start > 0 ? (
              <div aria-hidden style={{ height: visible.start * SEGMENT_PX }} />
            ) : null}

            {slice.map((seg) => (
              <SegmentRow
                key={seg.startSec}
                seg={seg}
                active={handlebarSec === seg.startSec}
                onJump={() =>
                  onSeek({
                    timeMs: seg.startSec * 1000,
                    eventId: seg.jumpEventId,
                  })
                }
                onHoverEnter={(el) => showHover(seg, el)}
                onHoverLeave={hideHover}
              />
            ))}

            {handlebarTop != null && handlebarSec != null ? (
              <div
                className="ff-review-timeline-handlebar"
                style={{ top: handlebarTop }}
                role="slider"
                aria-valuetext={formatHandlebar(handlebarSec)}
                aria-label="时间游标"
                onMouseDown={(e) => {
                  e.preventDefault()
                  e.stopPropagation()
                  dragRef.current = true
                  setDragging(true)
                  setHover(null)
                }}
                onTouchStart={(e) => {
                  e.preventDefault()
                  e.stopPropagation()
                  dragRef.current = true
                  setDragging(true)
                  setHover(null)
                }}
              >
                <div className="ff-review-timeline-handlebar-hit" />
                <div className="ff-review-timeline-handlebar-line" />
                <div className="ff-review-timeline-handlebar-pill">
                  {formatHandlebar(handlebarSec)}
                </div>
              </div>
            ) : null}
          </div>
        </div>

        <div className="ff-review-timeline-zoom" role="group" aria-label="时间轴缩放">
          <button
            type="button"
            className="ff-review-timeline-zoom-btn"
            title={`缩小 · 当前 ${zoom.label}`}
            aria-label={`缩小时间轴，当前 ${zoom.label}`}
            disabled={zoomIndex <= 0}
            onClick={(e) => {
              e.stopPropagation()
              setZoomIndex((i) => Math.max(0, i - 1))
            }}
          >
            −
          </button>
          <button
            type="button"
            className="ff-review-timeline-zoom-btn"
            title={`放大 · 当前 ${zoom.label}`}
            aria-label={`放大时间轴，当前 ${zoom.label}`}
            disabled={zoomIndex >= REVIEW_ZOOM_LEVELS.length - 1}
            onClick={(e) => {
              e.stopPropagation()
              setZoomIndex((i) =>
                Math.min(REVIEW_ZOOM_LEVELS.length - 1, i + 1),
              )
            }}
          >
            +
          </button>
        </div>
      </div>
    </aside>
  )
}

function SegmentRow({
  seg,
  active,
  onJump,
  onHoverEnter,
  onHoverLeave,
}: {
  seg: TimelineSegment
  active: boolean
  onJump: () => void
  onHoverEnter: (el: HTMLElement) => void
  onHoverLeave: () => void
}) {
  const ref = useRef<HTMLDivElement>(null)

  return (
    <div
      ref={ref}
      className="ff-review-timeline-seg"
      data-segment-id={seg.startSec}
      data-active={active ? 'true' : undefined}
      data-empty={seg.total === 0 ? 'true' : undefined}
      data-has-event={seg.primaryKind ? 'true' : undefined}
      style={{ height: SEGMENT_PX }}
      onClick={(e) => {
        e.stopPropagation()
        onJump()
      }}
      onMouseEnter={() => {
        if (ref.current) onHoverEnter(ref.current)
      }}
      onMouseLeave={onHoverLeave}
    >
      <span className="ff-review-timeline-tick-wrap" aria-hidden>
        <span className="ff-review-timeline-tick" data-size={seg.tickSize} />
      </span>

      {seg.showLabel ? (
        <span className="ff-review-timeline-label">{seg.label}</span>
      ) : null}

      {seg.primaryKind ? (
        <span
          className="ff-review-timeline-severity"
          data-tone={seg.primaryKind}
          data-density={seg.density}
          data-reviewed={seg.reviewed ? 'true' : undefined}
          data-round-top={seg.roundTop ? 'true' : undefined}
          data-round-bottom={seg.roundBottom ? 'true' : undefined}
        />
      ) : null}
    </div>
  )
}
