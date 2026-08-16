import { useState } from 'react'
import type { ReviewEvent } from '@/types/api'
import { formatAbsoluteShort, formatRelativeTime } from './review-time'

function ObjectGlyph({ event }: { event: ReviewEvent }) {
  const cls = (event.objectClass || event.label || '').toLowerCase()
  const isPerson = cls.includes('人') || cls.includes('person') || event.label === '离岗'
  const isCar =
    cls.includes('车') || cls.includes('car') || cls.includes('bicycle') || cls.includes('非机动')
  const isMotion = event.kind === 'motion'

  return (
    <span className="ff-review-card-glyph" data-kind={event.kind} title={event.objectClass || event.label}>
      {isMotion ? (
        <svg viewBox="0 0 24 24" aria-hidden>
          <path
            fill="currentColor"
            d="M4 12h2l2-6 3 12 2-8 2 4h5v-2h-3.5l-1.2-2.4L12 16 9 4 6.5 12H4z"
          />
        </svg>
      ) : isCar ? (
        <svg viewBox="0 0 24 24" aria-hidden>
          <path
            fill="currentColor"
            d="M5 11l1.5-4.5A2 2 0 0 1 8.4 5h7.2a2 2 0 0 1 1.9 1.5L19 11h1a1 1 0 0 1 1 1v5h-2a2 2 0 1 1-4 0H9a2 2 0 1 1-4 0H3v-5a1 1 0 0 1 1-1h1zm2.2-1h9.6l-1-3H8.2l-1 3z"
          />
        </svg>
      ) : isPerson ? (
        <svg viewBox="0 0 24 24" aria-hidden>
          <path
            fill="currentColor"
            d="M12 12a4 4 0 1 0-4-4 4 4 0 0 0 4 4zm0 2c-4 0-7 2-7 4.5V20h14v-1.5C19 16 16 14 12 14z"
          />
        </svg>
      ) : (
        <svg viewBox="0 0 24 24" aria-hidden>
          <circle cx="12" cy="12" r="5" fill="currentColor" />
        </svg>
      )}
    </span>
  )
}

export function ReviewCard({
  event,
  selected,
  timeActive,
  flash,
  onOpen,
  onToggleSelect,
  cardRef,
}: {
  event: ReviewEvent
  selected?: boolean
  /** 与时间轴游标同一时段 */
  timeActive?: boolean
  /** 时间轴跳转后短暂高亮 */
  flash?: boolean
  onOpen: () => void
  /** range=true：Shift 连选 */
  onToggleSelect: (range?: boolean) => void
  cardRef?: (el: HTMLButtonElement | null) => void
}) {
  const [hover, setHover] = useState(false)
  const reviewed = event.status === 'reviewed'
  const occurredMs = new Date(event.occurredAt).getTime()

  return (
    <button
      type="button"
      ref={cardRef}
      className="ff-review-card"
      data-reviewed={reviewed ? 'true' : undefined}
      data-selected={selected ? 'true' : undefined}
      data-time-active={timeActive ? 'true' : undefined}
      data-flash={flash ? 'true' : undefined}
      data-occurred={Number.isFinite(occurredMs) ? String(occurredMs) : undefined}
      onClick={onOpen}
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
    >
      <span
        className="ff-review-card-check"
        role="checkbox"
        aria-checked={selected}
        tabIndex={0}
        aria-label={selected ? '取消选择' : '选择'}
        onClick={(ev) => {
          ev.stopPropagation()
          onToggleSelect(ev.shiftKey)
        }}
        onKeyDown={(ev) => {
          if (ev.key === 'Enter' || ev.key === ' ') {
            ev.preventDefault()
            ev.stopPropagation()
            onToggleSelect(ev.shiftKey)
          }
        }}
      >
        <span className="ff-review-card-check-box" data-on={selected ? 'true' : undefined} />
      </span>

      <div className="ff-review-card-media">
        {event.thumbUrl ? (
          <img
            src={event.thumbUrl}
            alt=""
            className={hover ? 'ff-review-card-thumb-play' : undefined}
          />
        ) : (
          <div className="ff-review-card-thumb-fallback" />
        )}

        {event.boxes?.length ? (
          <svg className="ff-review-card-boxes" viewBox="0 0 1 1" preserveAspectRatio="none">
            {event.boxes.map((b, i) => (
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

        <div className="ff-review-card-top">
          <ObjectGlyph event={event} />
        </div>

        {hover ? (
          <div className="ff-review-card-hover" aria-hidden>
            <div className="ff-review-card-scrub">
              <span />
            </div>
          </div>
        ) : null}

        <div className="ff-review-card-bottom">
          <span className="ff-review-card-rel">{formatRelativeTime(event.occurredAt)}</span>
          <span className="ff-review-card-abs">{formatAbsoluteShort(event.occurredAt)}</span>
        </div>
      </div>
    </button>
  )
}
