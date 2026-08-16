import { useEffect, useMemo, useRef, useState } from 'react'
import {
  CheckIcon,
  ChevronDownIcon,
  MagnifyingGlassIcon,
  VideoCameraIcon,
  XMarkIcon,
} from '@heroicons/react/24/outline'

export type ReviewCameraOption = {
  id: string
  name: string
}

/** null = 全部摄像头；string[] = 子集（可为空=不选任何） */
export type CameraFilter = string[] | null

/**
 * 摄像头筛选：搜索 + 勾选即时生效 + 已选芯片可移除（无需应用/重置）
 */
export function ReviewCameraPicker({
  cameras,
  value,
  onChange,
}: {
  cameras: ReviewCameraOption[]
  value: CameraFilter
  onChange: (next: CameraFilter) => void
}) {
  const rootRef = useRef<HTMLDivElement>(null)
  const searchRef = useRef<HTMLInputElement>(null)
  const [open, setOpen] = useState(false)
  const [q, setQ] = useState('')

  useEffect(() => {
    if (!open) {
      setQ('')
      return
    }
    const t = window.setTimeout(() => searchRef.current?.focus(), 0)
    const onDoc = (e: MouseEvent) => {
      if (!rootRef.current?.contains(e.target as Node)) setOpen(false)
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false)
    }
    document.addEventListener('mousedown', onDoc)
    window.addEventListener('keydown', onKey)
    return () => {
      window.clearTimeout(t)
      document.removeEventListener('mousedown', onDoc)
      window.removeEventListener('keydown', onKey)
    }
  }, [open])

  const filtered = useMemo(() => {
    const needle = q.trim().toLowerCase()
    if (!needle) return cameras
    return cameras.filter((c) => c.name.toLowerCase().includes(needle))
  }, [cameras, q])

  const selectedCams = useMemo(() => {
    if (value === null) return []
    const map = new Map(cameras.map((c) => [c.id, c]))
    return value.map((id) => map.get(id)).filter(Boolean) as ReviewCameraOption[]
  }, [value, cameras])

  const triggerLabel = useMemo(() => {
    if (value === null) return '全部摄像头'
    if (value.length === 0) return '未选摄像头'
    if (value.length === 1) return selectedCams[0]?.name ?? '1 路'
    return `已选 ${value.length} 路`
  }, [value, selectedCams])

  const isChecked = (id: string) => value === null || value.includes(id)
  const allSelected = value === null

  const selectAll = () => onChange(null)

  const toggleOne = (id: string) => {
    const allIds = cameras.map((c) => c.id)
    if (value === null) {
      // 全部 → 去掉这一路
      onChange(allIds.filter((x) => x !== id))
      return
    }
    if (value.includes(id)) {
      onChange(value.filter((x) => x !== id))
      return
    }
    const next = [...value, id]
    onChange(next.length === allIds.length ? null : next)
  }

  const removeOne = (id: string) => {
    if (value === null) {
      onChange(cameras.map((c) => c.id).filter((x) => x !== id))
      return
    }
    onChange(value.filter((x) => x !== id))
  }

  return (
    <div className="ff-review-cam-picker" ref={rootRef}>
      <div className="ff-review-cam-chips">
        <button
          type="button"
          className="ff-review-cam-trigger"
          data-open={open ? 'true' : undefined}
          data-filtered={value !== null ? 'true' : undefined}
          aria-expanded={open}
          aria-haspopup="listbox"
          onClick={() => setOpen((v) => !v)}
        >
          <VideoCameraIcon className="ff-review-cam-trigger-icon" aria-hidden />
          <span>{triggerLabel}</span>
          <ChevronDownIcon className="ff-review-cam-chevron" aria-hidden />
        </button>

        {selectedCams.slice(0, 3).map((c) => (
          <button
            key={c.id}
            type="button"
            className="ff-review-cam-chip"
            title={`移除 ${c.name}`}
            onClick={() => removeOne(c.id)}
          >
            <span>{c.name}</span>
            <XMarkIcon className="ff-review-cam-chip-x" aria-hidden />
          </button>
        ))}
        {selectedCams.length > 3 ? (
          <span className="ff-review-cam-chip-more">+{selectedCams.length - 3}</span>
        ) : null}
        {value !== null ? (
          <button
            type="button"
            className="ff-review-cam-clear"
            onClick={() => onChange(null)}
          >
            清除
          </button>
        ) : null}
      </div>

      {open ? (
        <div className="ff-review-cam-popover" role="listbox" aria-label="选择摄像头" aria-multiselectable>
          <div className="ff-review-cam-search">
            <MagnifyingGlassIcon className="ff-review-cam-search-icon" aria-hidden />
            <input
              ref={searchRef}
              type="search"
              value={q}
              placeholder="搜索摄像头…"
              aria-label="搜索摄像头"
              onChange={(e) => setQ(e.target.value)}
            />
          </div>

          <div className="ff-review-cam-quick">
            <button
              type="button"
              className="ff-review-cam-quick-btn"
              data-active={allSelected ? 'true' : undefined}
              onClick={selectAll}
            >
              全部
            </button>
            <button
              type="button"
              className="ff-review-cam-quick-btn"
              onClick={() => onChange([])}
            >
              全不选
            </button>
          </div>

          <div className="ff-review-cam-list">
            {filtered.map((c) => {
              const on = isChecked(c.id)
              return (
                <button
                  key={c.id}
                  type="button"
                  role="option"
                  aria-selected={on}
                  className="ff-review-cam-option"
                  data-on={on ? 'true' : undefined}
                  onClick={() => toggleOne(c.id)}
                >
                  <span
                    className="ff-review-cam-check"
                    data-on={on ? 'true' : undefined}
                    aria-hidden
                  >
                    {on ? <CheckIcon /> : null}
                  </span>
                  <span className="ff-review-cam-name" title={c.name}>
                    {c.name}
                  </span>
                </button>
              )
            })}
            {!filtered.length ? (
              <div className="ff-review-cam-empty">
                {cameras.length ? '无匹配摄像头' : '暂无摄像头'}
              </div>
            ) : null}
          </div>
        </div>
      ) : null}
    </div>
  )
}
