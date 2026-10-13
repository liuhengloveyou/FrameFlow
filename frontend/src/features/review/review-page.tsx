import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { Button } from '@astryxdesign/core/Button'
import { Selector } from '@astryxdesign/core/Selector'
import { Text } from '@astryxdesign/core/Text'
import { Icon } from '@astryxdesign/core/Icon'
import { CheckCircleIcon, MegaphoneIcon } from '@heroicons/react/24/outline'
import { PageLayout } from '@/components/page-layout'
import { PageToolbar } from '@/components/page-toolbar'
import { DataTable, proportional, pixel, type DataTableColumn } from '@/components/data-table'
import { InlineStat, InlineStatGroup } from '@/components/inline-stat'
import {
  ConfidenceValue,
  EventTypeBadge,
  ReviewStatusBadge,
} from '@/components/event-badges'
import { MediaThumb } from '@/components/media-thumb'
import {
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
import { ReviewDetail } from './review-detail'
import { ReviewRangePicker } from './review-range-picker'
import { resolveReviewRangeBounds, type ReviewTimeRange } from './review-range'

type StatusFilter = 'unreviewed' | 'reviewed' | 'all'

const STATUS_OPTIONS = [
  { label: '待核查', value: 'unreviewed' },
  { label: '已确认', value: 'reviewed' },
  { label: '全部', value: 'all' },
]

const KIND_OPTIONS = [
  { label: '警报', value: 'alert' },
]

function formatTime(iso: string) {
  const d = new Date(iso)
  const p = (n: number) => String(n).padStart(2, '0')
  return `${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}

export function ReviewPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const [kind, setKind] = useState<ReviewEventKind>('alert')
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('unreviewed')
  const [cameraFilter, setCameraFilter] = useState<CameraFilter>(null)
  const [timeRange, setTimeRange] = useState<ReviewTimeRange>({ preset: '7d' })
  const [detail, setDetail] = useState<ReviewEvent | null>(null)
  const [picked, setPicked] = useState<Set<string>>(() => new Set())
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

  // 一次取全量（含已核查），处置状态与统计均在前端过滤
  const eventsQuery = useQuery({
    queryKey: ['review-events', { kind, cameraFilter, fromIso, toIso }],
    queryFn: () =>
      listReviewEvents({
        kind,
        cameraIds: cameraIdsParam,
        from: fromIso,
        to: toIso,
        pageSize: 500,
      }),
  })

  const allEvents = eventsQuery.data?.items ?? []

  const stats = useMemo(() => {
    let unreviewed = 0
    let reviewed = 0
    let promoted = 0
    for (const e of allEvents) {
      if (e.status === 'reviewed') reviewed += 1
      else unreviewed += 1
      if (e.promotedAlertId) promoted += 1
    }
    return { unreviewed, reviewed, promoted }
  }, [allEvents])

  const kindEvents = useMemo(() => {
    if (statusFilter === 'all') return allEvents
    return allEvents.filter((e) =>
      statusFilter === 'reviewed' ? e.status === 'reviewed' : e.status === 'unreviewed',
    )
  }, [allEvents, statusFilter])

  const pickedCount = picked.size
  const pickedInView = kindEvents.filter((e) => picked.has(e.id)).map((e) => e.id)

  const invalidate = async () => {
    await queryClient.invalidateQueries({ queryKey: ['review-events'] })
    await queryClient.invalidateQueries({ queryKey: ['review-counts'] })
    await queryClient.invalidateQueries({ queryKey: ['review-context'] })
  }

  const markMut = useMutation({
    mutationFn: (ids: string[]) => markReviewed(ids, 'reviewed'),
    onSuccess: async (n) => {
      setDetail((s) =>
        s && picked.has(s.id) ? { ...s, status: 'reviewed' } : s,
      )
      setPicked(new Set())
      await invalidate()
      void n
    },
    onError: (err) => getErrorMessage(err),
  })

  const unmarkMut = useMutation({
    mutationFn: (id: string) => markReviewed([id], 'unreviewed'),
    onSuccess: async () => {
      setDetail((s) => (s ? { ...s, status: 'unreviewed' } : s))
      await invalidate()
    },
  })

  const promoteMut = useMutation({
    mutationFn: (id: string) => promoteDetectionToAlert(id),
    onSuccess: async (alert) => {
      setKind('alert')
      setDetail(alert)
      await invalidate()
    },
  })

  function togglePick(id: string) {
    setPicked((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  function togglePickAll() {
    if (pickedInView.length && pickedInView.length === kindEvents.length) {
      setPicked(new Set())
      return
    }
    setPicked(new Set(kindEvents.map((e) => e.id)))
  }

  const batchBusy = markMut.isPending

  const columns = [
    {
      key: 'pick',
      header: (
        <input
          type="checkbox"
          aria-label="全选本页"
          checked={kindEvents.length > 0 && pickedInView.length === kindEvents.length}
          onChange={togglePickAll}
        />
      ),
      width: pixel(40),
      align: 'center',
      render: (r: ReviewEvent) => (
        <input
          type="checkbox"
          aria-label={`选择 ${r.title}`}
          checked={picked.has(r.id)}
          onChange={() => togglePick(r.id)}
          onClick={(e) => e.stopPropagation()}
        />
      ),
    },
    {
      key: 'evidence',
      header: '证据',
      width: pixel(132),
      render: (r: ReviewEvent) => (
        <MediaThumb src={r.thumbUrl} alt={r.title} seed={r.id} />
      ),
    },
    {
      key: 'occurredAt',
      header: '时间',
      width: proportional(1.3, { minWidth: 130 }),
      render: (r: ReviewEvent) => (
        <Text type="supporting">{formatTime(r.occurredAt)}</Text>
      ),
    },
    {
      key: 'type',
      header: '事件类型',
      width: proportional(1.1, { minWidth: 110 }),
      render: (r: ReviewEvent) => (
        <EventTypeBadge label={r.title || r.label} />
      ),
    },
    {
      key: 'device',
      header: '设备 / 通道',
      width: proportional(1.4, { minWidth: 140 }),
      render: (r: ReviewEvent) => (
        <Text type="body" maxLines={1}>
          {r.cameraName}
        </Text>
      ),
    },
    {
      key: 'score',
      header: 'AI 置信度',
      width: pixel(96),
      align: 'end',
      render: (r: ReviewEvent) => <ConfidenceValue score={r.score} />,
    },
    {
      key: 'status',
      header: '处置状态',
      width: proportional(1, { minWidth: 96 }),
      render: (r: ReviewEvent) => <ReviewStatusBadge status={r.status} />,
    },
    {
      key: 'actions',
      header: '操作',
      width: pixel(150),
      render: (r: ReviewEvent) => (
        <div className="ff-review-row-actions">
          {r.status === 'reviewed' ? (
            <Button
              label="恢复"
              size="sm"
              variant="ghost"
              isLoading={unmarkMut.isPending}
              onClick={(e) => {
                e.stopPropagation()
                void unmarkMut.mutateAsync(r.id)
              }}
            />
          ) : (
            <Button
              label="确认"
              size="sm"
              variant="ghost"
              isLoading={markMut.isPending}
              onClick={(e) => {
                e.stopPropagation()
                void markMut.mutateAsync([r.id])
              }}
            />
          )}
          <Button
            label="详情"
            size="sm"
            variant="ghost"
            onClick={(e) => {
              e.stopPropagation()
              setDetail(r)
            }}
          />
        </div>
      ),
    },
  ] satisfies DataTableColumn<ReviewEvent>[]

  return (
    <PageLayout
      title="事件核查"
      badge={stats.unreviewed ? `待核查 ${stats.unreviewed}` : '全部已核查'}
      contentPadding={3}
      toolbar={
        <PageToolbar
          filters={
            <>
              <Selector
                label="事件类型"
                isLabelHidden
                placeholder="事件类型：全部"
                value={kind}
                onChange={(v) => setKind((v as ReviewEventKind) || 'alert')}
                options={KIND_OPTIONS}
              />
              <Selector
                label="处置状态"
                isLabelHidden
                placeholder="处置状态：待核查"
                value={statusFilter}
                onChange={(v) => setStatusFilter((v as StatusFilter) || 'unreviewed')}
                options={STATUS_OPTIONS}
              />
              <ReviewRangePicker
                value={timeRange}
                onChange={(next) => {
                  setTimeRange(next)
                  setPicked(new Set())
                }}
              />
              <ReviewCameraPicker
                cameras={cameraOptions}
                value={cameraFilter}
                onChange={(next) => {
                  setCameraFilter(next)
                  setPicked(new Set())
                }}
              />
            </>
          }
          actions={
            <Button
              label={
                pickedCount > 0 ? `批量核查 · ${pickedCount}` : '全部标记已核查'
              }
              size="sm"
              variant="primary"
              isDisabled={!kindEvents.length}
              isLoading={batchBusy}
              clickAction={async () => {
                const ids = pickedInView.length ? pickedInView : kindEvents.map((e) => e.id)
                await markMut.mutateAsync(ids)
              }}
            />
          }
          stats={
            <InlineStatGroup>
              <InlineStat
                icon={<Icon icon={CheckCircleIcon} size="sm" />}
                value={stats.unreviewed}
                label="待核查"
                tone="warn"
              />
              <InlineStat
                icon={<CheckCircleIcon />}
                value={stats.reviewed}
                label="已确认"
                tone="ok"
              />
              <InlineStat
                icon={<MegaphoneIcon />}
                value={stats.promoted}
                label="已提升"
                title="已提升为报警的事件数"
              />
            </InlineStatGroup>
          }
        />
      }
    >
      {eventsQuery.isLoading ? (
        <div className="ff-placeholder">加载中…</div>
      ) : (
        <div className="ff-table-wrap">
          <DataTable
            data={kindEvents}
            idKey="id"
            density="balanced"
            isStriped
            emptyLabel="暂无事件，当前筛选条件下没有记录"
            onRowClick={(r) => setDetail(r)}
            columns={columns}
          />
        </div>
      )}

      {pickedCount > 0 ? (
        <Text type="supporting" color="secondary">
          已选 {pickedCount} 项 · 点击「批量核查」标记为已确认
        </Text>
      ) : null}

      {detail ? (
        <ReviewDetail
          event={detail}
          neighbors={{}}
          marking={unmarkMut.isPending}
          promoting={promoteMut.isPending}
          onClose={() => setDetail(null)}
          onSelect={setDetail}
          onMark={(reviewed) => {
            if (reviewed) void markMut.mutateAsync([detail.id])
            else void unmarkMut.mutateAsync(detail.id)
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
            if (!cam) return
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
