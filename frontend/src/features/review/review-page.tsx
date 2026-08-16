import { useEffect, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { Button } from '@astryxdesign/core/Button'
import { Switch } from '@astryxdesign/core/Switch'
import { useToast } from '@astryxdesign/core/Toast'
import { PageLayout } from '@/components/page-layout'
import {
  getReviewKindCounts,
  listReviewEvents,
  markReviewed,
  promoteDetectionToAlert,
} from '@/apis/review-api'
import { listCameras } from '@/apis/devices-api'
import { getErrorMessage } from '@/lib/http'
import { seedPreviewSource } from '@/lib/preview-seed'
import type { Camera, ReviewEvent, ReviewEventKind } from '@/types/api'
import { MonitorDetail } from '@/features/preview/monitor-detail'
import {
  ReviewCameraPicker,
  type CameraFilter,
} from './review-camera-picker'
import { ReviewCameraLanes } from './review-camera-lanes'
import { ReviewDetail } from './review-detail'
import { ReviewRangePicker } from './review-range-picker'
import { resolveReviewRangeBounds, type ReviewTimeRange } from './review-range'
import { ReviewTimeline } from './review-timeline'

const KIND_TABS: {
  id: ReviewEventKind
  label: string
  tone: string
  /** MVP：仅警报有数据源 */
  disabled?: boolean
}[] = [
  { id: 'alert', label: '警报', tone: 'alert' },
  { id: 'detection', label: '检测', tone: 'detection', disabled: true },
  { id: 'motion', label: '变动', tone: 'motion', disabled: true },
]

export function ReviewPage() {
  const toast = useToast()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const lanesRef = useRef<HTMLDivElement>(null)
  const cardRefs = useRef<Record<string, HTMLButtonElement | null>>({})
  const syncFromScrollRef = useRef(false)
  const lastSelectRef = useRef<string | null>(null)
  const seekTokenRef = useRef(0)

  const [kind, setKind] = useState<ReviewEventKind>('alert')
  const [showReviewed, setShowReviewed] = useState(false)
  const [cameraFilter, setCameraFilter] = useState<CameraFilter>(null)
  const [timeRange, setTimeRange] = useState<ReviewTimeRange>({ preset: '24h' })
  const [detail, setDetail] = useState<ReviewEvent | null>(null)
  const [activeTimeMs, setActiveTimeMs] = useState<number | null>(null)
  const [flashId, setFlashId] = useState<string | null>(null)
  const [picked, setPicked] = useState<Set<string>>(() => new Set())
  const flashTimerRef = useRef<number | null>(null)
  const [eventPlayer, setEventPlayer] = useState<{
    camera: Camera
    channelId?: string
    seekAt: string
  } | null>(null)

  const { startMs, endMs } = useMemo(
    () => resolveReviewRangeBounds(timeRange),
    [timeRange],
  )
  const fromIso = useMemo(() => new Date(startMs).toISOString(), [startMs])
  const toIso = useMemo(() => new Date(endMs).toISOString(), [endMs])

  const camerasQuery = useQuery({
    queryKey: ['cameras', { forReview: true }],
    queryFn: () => listCameras({ page: 1, pageSize: 200 }),
  })

  const cameraById = useMemo(() => {
    const map = new Map<string, Camera>()
    for (const c of camerasQuery.data?.items ?? []) map.set(String(c.id), c)
    return map
  }, [camerasQuery.data])

  const cameraOptions = useMemo(() => {
    return (
      camerasQuery.data?.items.map((c) => ({ id: String(c.id), name: c.name })) ?? []
    )
  }, [camerasQuery.data])

  const cameraIdsParam = cameraFilter === null ? undefined : cameraFilter

  const eventsQuery = useQuery({
    queryKey: ['review-events', { kind, showReviewed, cameraFilter, fromIso, toIso }],
    queryFn: () =>
      listReviewEvents({
        kind,
        status: showReviewed ? undefined : 'unreviewed',
        cameraIds: cameraIdsParam,
        from: fromIso,
        to: toIso,
        pageSize: 500,
      }),
  })

  const countsQuery = useQuery({
    queryKey: ['review-counts', { showReviewed, cameraFilter, fromIso, toIso }],
    queryFn: () =>
      getReviewKindCounts({
        status: showReviewed ? undefined : 'unreviewed',
        cameraIds: cameraIdsParam,
        from: fromIso,
        to: toIso,
      }),
  })

  const allEvents = eventsQuery.data?.items ?? []
  const counts = countsQuery.data

  const laneCameras = useMemo(() => {
    if (cameraFilter === null) return cameraOptions
    if (cameraFilter.length === 0) return []
    const set = new Set(cameraFilter)
    return cameraOptions.filter((c) => set.has(c.id))
  }, [cameraOptions, cameraFilter])

  const kindEvents = allEvents
  const itemIdsKey = kindEvents.map((e) => e.id).join(',')

  const activeCardId = useMemo(() => {
    if (activeTimeMs == null || !kindEvents.length) return null
    let bestId = kindEvents[0].id
    let bestDist = Math.abs(
      new Date(kindEvents[0].occurredAt).getTime() - activeTimeMs,
    )
    for (const e of kindEvents) {
      const d = Math.abs(new Date(e.occurredAt).getTime() - activeTimeMs)
      if (d < bestDist) {
        bestDist = d
        bestId = e.id
      }
    }
    return bestId
  }, [activeTimeMs, kindEvents])

  const neighbors = useMemo(() => {
    if (!detail) return {}
    const list = kindEvents.filter((e) => e.cameraId === detail.cameraId)
    const idx = list.findIndex((e) => e.id === detail.id)
    if (idx < 0) return {}
    return { prev: list[idx - 1], next: list[idx + 1] }
  }, [kindEvents, detail])

  const pickedCount = picked.size
  const pickedInView = kindEvents.filter((e) => picked.has(e.id)).map((e) => e.id)

  const invalidate = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['review-events'] }),
      queryClient.invalidateQueries({ queryKey: ['review-counts'] }),
      queryClient.invalidateQueries({ queryKey: ['review-context'] }),
    ])
  }

  const markMut = useMutation({
    mutationFn: (ids: string[]) => markReviewed(ids, 'reviewed'),
    onSuccess: async (n) => {
      toast({ body: `已标记 ${n} 条为已核查`, type: 'info' })
      setPicked(new Set())
      await invalidate()
    },
    onError: (err) => toast({ body: getErrorMessage(err), type: 'error' }),
  })

  const markOneMut = useMutation({
    mutationFn: ({ id, reviewed }: { id: string; reviewed: boolean }) =>
      markReviewed([id], reviewed ? 'reviewed' : 'unreviewed'),
    onSuccess: async (_n, vars) => {
      await invalidate()
      setDetail((s) =>
        s && s.id === vars.id
          ? { ...s, status: vars.reviewed ? 'reviewed' : 'unreviewed' }
          : s,
      )
    },
    onError: (err) => toast({ body: getErrorMessage(err), type: 'error' }),
  })

  const promoteMut = useMutation({
    mutationFn: (id: string) => promoteDetectionToAlert(id),
    onSuccess: async (alert) => {
      toast({ body: '已提升为警报', type: 'info' })
      await invalidate()
      setKind('alert')
      setDetail(alert)
    },
    onError: (err) => toast({ body: getErrorMessage(err), type: 'error' }),
  })

  const toggleSelect = (id: string, range = false) => {
    setPicked((prev) => {
      const next = new Set(prev)
      if (range && lastSelectRef.current) {
        const a = kindEvents.findIndex((e) => e.id === lastSelectRef.current)
        const b = kindEvents.findIndex((e) => e.id === id)
        if (a >= 0 && b >= 0) {
          const [lo, hi] = a < b ? [a, b] : [b, a]
          for (let i = lo; i <= hi; i++) next.add(kindEvents[i].id)
          return next
        }
      }
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
    lastSelectRef.current = id
  }

  const seekTimeline = (payload: {
    timeMs: number
    eventId: string | null
    scrubbing?: boolean
  }) => {
    setActiveTimeMs(payload.timeMs)
    if (payload.scrubbing) return

    const token = ++seekTokenRef.current
    syncFromScrollRef.current = true

    let targetId = payload.eventId
    if (targetId && !kindEvents.some((e) => e.id === targetId)) targetId = null
    if (!targetId && kindEvents.length) {
      let best = kindEvents[0]
      let bestDist = Math.abs(new Date(best.occurredAt).getTime() - payload.timeMs)
      for (const e of kindEvents) {
        const d = Math.abs(new Date(e.occurredAt).getTime() - payload.timeMs)
        if (d < bestDist) {
          best = e
          bestDist = d
        }
      }
      targetId = best.id
    }

    const el = targetId ? cardRefs.current[targetId] : null
    if (el) {
      el.scrollIntoView({ behavior: 'smooth', block: 'center', inline: 'nearest' })
    }

    if (targetId) {
      setFlashId(targetId)
      if (flashTimerRef.current) window.clearTimeout(flashTimerRef.current)
      flashTimerRef.current = window.setTimeout(() => {
        setFlashId((cur) => (cur === targetId ? null : cur))
      }, 2800)
    }

    window.setTimeout(() => {
      if (seekTokenRef.current === token) syncFromScrollRef.current = false
    }, 360)
  }

  useEffect(() => {
    const root = lanesRef.current
    if (!root || !kindEvents.length) return
    const onScroll = () => {
      if (syncFromScrollRef.current) return
      const rootBox = root.getBoundingClientRect()
      const cy = rootBox.top + Math.min(160, rootBox.height / 3)
      let bestId: string | null = null
      let bestDist = Number.POSITIVE_INFINITY
      for (const e of kindEvents) {
        const el = cardRefs.current[e.id]
        if (!el) continue
        const r = el.getBoundingClientRect()
        // 竖向时间轴：优先按垂直距离选中卡片
        const dist =
          Math.abs(r.top + r.height / 2 - cy) * 2 +
          Math.abs(r.left + r.width / 2 - (rootBox.left + rootBox.width / 2)) * 0.25
        if (dist < bestDist) {
          bestDist = dist
          bestId = e.id
        }
      }
      if (!bestId) return
      const hit = kindEvents.find((e) => e.id === bestId)
      if (hit) setActiveTimeMs(new Date(hit.occurredAt).getTime())
    }
    root.addEventListener('scroll', onScroll, { passive: true })
    onScroll()
    return () => root.removeEventListener('scroll', onScroll)
  }, [itemIdsKey, kindEvents])

  useEffect(() => {
    if (!detail) return
    const fresh = allEvents.find((e) => e.id === detail.id)
    if (
      fresh &&
      (fresh.status !== detail.status || fresh.promotedAlertId !== detail.promotedAlertId)
    ) {
      setDetail(fresh)
    }
  }, [allEvents, detail])

  return (
    <PageLayout title="事件核查" contentPadding={0}>
      <div className="ff-review-shell">
        <div className="ff-review-toolbar">
          <div className="ff-review-filters">
            <ReviewCameraPicker
              cameras={cameraOptions}
              value={cameraFilter}
              onChange={(next) => {
                setCameraFilter(next)
                setPicked(new Set())
              }}
            />
            <ReviewRangePicker
              value={timeRange}
              onChange={(next) => {
                setTimeRange(next)
                setPicked(new Set())
                setActiveTimeMs(null)
              }}
            />
            <label className="ff-review-reviewed-toggle">
              <span>显示已核查</span>
              <Switch
                label="显示已核查"
                isLabelHidden
                value={showReviewed}
                onChange={setShowReviewed}
              />
            </label>
            <div role="tablist" aria-label="事件类型" className="ff-review-kind-tabs">
              {KIND_TABS.map((t) => {
                const n = counts?.[t.id]
                const active = kind === t.id
                return (
                  <button
                    key={t.id}
                    type="button"
                    role="tab"
                    aria-selected={active}
                    aria-disabled={t.disabled ? true : undefined}
                    disabled={t.disabled}
                    title={t.disabled ? '即将开放：独立检测/变动事件源' : undefined}
                    className="ff-review-filter"
                    data-tone={t.tone}
                    data-active={active ? 'true' : undefined}
                    data-disabled={t.disabled ? 'true' : undefined}
                    onClick={() => {
                      if (t.disabled) return
                      setKind(t.id)
                      setPicked(new Set())
                    }}
                  >
                    <span className="ff-review-filter-dot" />
                    <span>
                      {t.label}
                      {n != null ? ` ${n}` : ''}
                    </span>
                  </button>
                )
              })}
            </div>
          </div>

          <div className="ff-review-actions">
            {pickedCount > 0 ? (
              <>
                <span className="ff-review-selected-label">已选 {pickedCount} 项</span>
                <Button
                  label="取消选择"
                  size="sm"
                  variant="ghost"
                  clickAction={() => setPicked(new Set())}
                />
                <Button
                  label="标记为已核查"
                  size="sm"
                  isLoading={markMut.isPending}
                  clickAction={async () => {
                    const ids = pickedInView.length ? pickedInView : [...picked]
                    await markMut.mutateAsync(ids)
                  }}
                />
              </>
            ) : (
              <Button
                label="全部标记已核查"
                size="sm"
                variant="secondary"
                isDisabled={!kindEvents.length}
                isLoading={markMut.isPending}
                clickAction={async () => {
                  await markMut.mutateAsync(kindEvents.map((e) => e.id))
                }}
              />
            )}
          </div>
        </div>

        <div className="ff-review-layout">
          {eventsQuery.isLoading ? (
            <div className="ff-placeholder">加载中…</div>
          ) : (
            <ReviewCameraLanes
              cameras={laneCameras}
              events={kindEvents}
              selectedIds={picked}
              activeEventId={activeCardId}
              flashEventId={flashId}
              cardRefs={cardRefs}
              scrollRef={lanesRef}
              onOpen={setDetail}
              onToggleSelect={toggleSelect}
            />
          )}

          <ReviewTimeline
            events={allEvents}
            preferKind={kind}
            startMs={startMs}
            endMs={endMs}
            activeTimeMs={activeTimeMs}
            onSeek={seekTimeline}
          />
        </div>
      </div>

      {detail ? (
        <ReviewDetail
          event={detail}
          neighbors={neighbors}
          marking={markOneMut.isPending}
          promoting={promoteMut.isPending}
          onClose={() => setDetail(null)}
          onSelect={setDetail}
          onMark={(reviewed) => {
            void markOneMut.mutateAsync({ id: detail.id, reviewed })
          }}
          onPromote={() => {
            void promoteMut.mutateAsync(detail.id)
          }}
          onOpenLive={() => {
            seedPreviewSource({ cameraId: detail.cameraId })
            void navigate({ to: '/preview' })
          }}
          onOpenEventVideo={() => {
            const cam = cameraById.get(detail.cameraId)
            if (!cam) {
              toast({ body: '未找到对应摄像头', type: 'error' })
              return
            }
            const at = new Date(detail.occurredAt).getTime()
            const seekAt = Number.isFinite(at)
              ? new Date(Math.max(0, at - 2000)).toISOString()
              : detail.occurredAt
            const channelId =
              (cam.channels?.[0] ? String(cam.channels[0].id) : undefined) ||
              cam.channel ||
              undefined
            setEventPlayer({ camera: cam, channelId, seekAt })
          }}
        />
      ) : null}

      {eventPlayer ? (
        <div className="ff-review-event-player">
          <MonitorDetail
            key={`${eventPlayer.camera.id}-${eventPlayer.seekAt}`}
            camera={eventPlayer.camera}
            channelId={eventPlayer.channelId}
            showAiOverlay={false}
            initialPlayback
            initialSeekAt={eventPlayer.seekAt}
            onClose={() => setEventPlayer(null)}
          />
        </div>
      ) : null}
    </PageLayout>
  )
}
