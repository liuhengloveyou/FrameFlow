import { useEffect, useRef, useState } from 'react'
import { CalendarDaysIcon, ChevronDownIcon } from '@heroicons/react/24/outline'
import {
  localDateKey,
  reviewRangeLabel,
  type ReviewRangePreset,
  type ReviewTimeRange,
} from './review-range'

const PRESETS: { id: ReviewRangePreset; label: string }[] = [
  { id: '24h', label: '过去 24 小时' },
  { id: 'today', label: '今天' },
  { id: 'yesterday', label: '昨天' },
  { id: '7d', label: '过去 7 天' },
]

export function ReviewRangePicker({
  value,
  onChange,
}: {
  value: ReviewTimeRange
  onChange: (next: ReviewTimeRange) => void
}) {
  const rootRef = useRef<HTMLDivElement>(null)
  const [open, setOpen] = useState(false)

  useEffect(() => {
    if (!open) return
    const onDoc = (e: MouseEvent) => {
      if (!rootRef.current?.contains(e.target as Node)) setOpen(false)
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false)
    }
    document.addEventListener('mousedown', onDoc)
    window.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDoc)
      window.removeEventListener('keydown', onKey)
    }
  }, [open])

  return (
    <div className="ff-review-range" ref={rootRef}>
      <button
        type="button"
        className="ff-review-cam-trigger"
        data-open={open ? 'true' : undefined}
        data-filtered={value.preset !== '24h' ? 'true' : undefined}
        aria-haspopup="listbox"
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
      >
        <CalendarDaysIcon className="ff-review-cam-trigger-icon" aria-hidden />
        <span>{reviewRangeLabel(value)}</span>
        <ChevronDownIcon className="ff-review-cam-chevron" aria-hidden />
      </button>

      {open ? (
        <div className="ff-review-range-popover" role="listbox" aria-label="选择时间段">
          {PRESETS.map((p) => (
            <button
              key={p.id}
              type="button"
              role="option"
              aria-selected={value.preset === p.id}
              className="ff-review-range-option"
              data-on={value.preset === p.id ? 'true' : undefined}
              onClick={() => {
                onChange({ preset: p.id })
                setOpen(false)
              }}
            >
              {p.label}
            </button>
          ))}

          <div className="ff-review-range-day">
            <label htmlFor="ff-review-range-day">指定日期</label>
            <input
              id="ff-review-range-day"
              type="date"
              max={localDateKey()}
              value={value.preset === 'day' ? (value.day ?? localDateKey()) : ''}
              onChange={(e) => {
                const day = e.target.value
                if (!day) return
                onChange({ preset: 'day', day })
                setOpen(false)
              }}
            />
          </div>
        </div>
      ) : null}
    </div>
  )
}
