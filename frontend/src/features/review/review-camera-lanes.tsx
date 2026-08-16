import { useMemo } from 'react'
import type { ReviewEvent } from '@/types/api'
import { ReviewCard } from './review-card'

export type LaneCamera = { id: string; name: string }

/**
 * 视频主体：摄像头横排为列，列内事件竖排；主区纵向滚动即沿时间浏览。
 */
export function ReviewCameraLanes({
  cameras,
  events,
  selectedIds,
  activeEventId,
  flashEventId,
  cardRefs,
  scrollRef,
  onOpen,
  onToggleSelect,
}: {
  cameras: LaneCamera[]
  events: ReviewEvent[]
  selectedIds: Set<string>
  activeEventId: string | null
  flashEventId?: string | null
  cardRefs: React.RefObject<Record<string, HTMLButtonElement | null>>
  scrollRef: React.RefObject<HTMLDivElement | null>
  onOpen: (e: ReviewEvent) => void
  onToggleSelect: (id: string, range?: boolean) => void
}) {
  const rows = useMemo(() => {
    const byCam = new Map<string, ReviewEvent[]>()
    for (const c of cameras) byCam.set(c.id, [])
    for (const e of events) {
      const list = byCam.get(e.cameraId)
      if (list) list.push(e)
    }
    for (const list of byCam.values()) {
      list.sort((a, b) => b.occurredAt.localeCompare(a.occurredAt))
    }
    return cameras.map((cam) => ({
      cam,
      events: byCam.get(cam.id) ?? [],
    }))
  }, [cameras, events])

  if (!cameras.length) {
    return (
      <div className="ff-review-lanes" ref={scrollRef}>
        <div className="ff-placeholder">请选择至少一个摄像头</div>
      </div>
    )
  }

  return (
    <div className="ff-review-lanes" ref={scrollRef}>
      {rows.map(({ cam, events: rowEvents }) => (
        <section
          key={cam.id}
          className="ff-review-lane"
          data-empty={rowEvents.length === 0 ? 'true' : undefined}
          aria-label={cam.name}
        >
          <header className="ff-review-lane-head">
            <h3 className="ff-review-lane-title" title={cam.name}>
              {cam.name}
            </h3>
            <span className="ff-review-lane-count">
              {rowEvents.length ? `${rowEvents.length}` : '0'}
            </span>
          </header>
          <div className="ff-review-lane-strip">
            {rowEvents.length === 0 ? (
              <div className="ff-review-lane-empty">该时段暂无此类事件</div>
            ) : (
              rowEvents.map((e) => (
                <div key={e.id} className="ff-review-lane-card">
                  <ReviewCard
                    event={e}
                    selected={selectedIds.has(e.id)}
                    timeActive={activeEventId === e.id}
                    flash={flashEventId === e.id}
                    cardRef={(el) => {
                      cardRefs.current[e.id] = el
                    }}
                    onOpen={() => onOpen(e)}
                    onToggleSelect={(range) => onToggleSelect(e.id, range)}
                  />
                </div>
              ))
            )}
          </div>
        </section>
      ))}
    </div>
  )
}
