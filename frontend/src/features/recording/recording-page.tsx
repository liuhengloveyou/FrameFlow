import { useEffect, useMemo, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { Button } from '@astryxdesign/core/Button'
import { DateInput } from '@astryxdesign/core/DateInput'
import {
  SegmentedControl,
  SegmentedControlItem,
} from '@astryxdesign/core/SegmentedControl'
import { HStack } from '@astryxdesign/core/HStack'
import { TextInput } from '@astryxdesign/core/TextInput'
import { Text } from '@astryxdesign/core/Text'
import { VStack } from '@astryxdesign/core/VStack'
import { useToast } from '@astryxdesign/core/Toast'
import { PageLayout } from '@/components/page-layout'
import { SidePanel } from '@/components/side-panel'
import { CameraStatusBadge } from '@/components/status-badge'
import { listCameras } from '@/apis/devices-api'
import { listReviewEvents } from '@/apis/review-api'
import { seedPreviewSource } from '@/lib/preview-seed'
import {
  encodeSlotKey,
  channelBizId,
  parseSlotKey,
  type PreviewSlotSource,
} from '@/lib/preview-source'
import type { Camera } from '@/types/api'
import { PreviewTile } from '@/features/preview/preview-tile'
import { MonitorDetail } from '@/features/preview/monitor-detail'
import { formatClock, formatDay } from '@/features/preview/playback-utils'
import { ReviewTimeline } from '@/features/review/review-timeline'
import { RecordingPolicyPanel } from './recording-policy-panel'
import { localDateKey } from './recording-segments'

type Layout = '1' | '4' | '9'

function slotCount(layout: Layout) {
  return layout === '1' ? 1 : layout === '4' ? 4 : 9
}

function collectStreamKeys(cameras: Camera[]): string[] {
  const keys: string[] = []
  const seen = new Set<string>()
  for (const cam of cameras) {
    const channels = cam.channels ?? []
    if (channels.length > 0) {
      for (const ch of channels) {
        const key = encodeSlotKey({
          cameraId: String(cam.id),
          channelId: channelBizId(ch),
        })
        if (seen.has(key)) continue
        seen.add(key)
        keys.push(key)
      }
      continue
    }
    const key = encodeSlotKey({
      cameraId: String(cam.id),
      channelId: cam.channel || undefined,
    })
    if (seen.has(key)) continue
    seen.add(key)
    keys.push(key)
  }
  return keys
}

function resizeSlots(
  prev: Array<string | null>,
  next: Layout,
  preferredFirst?: string | null,
) {
  const count = slotCount(next)
  const ordered: string[] = []
  const seen = new Set<string>()
  if (preferredFirst) {
    ordered.push(preferredFirst)
    seen.add(preferredFirst)
  }
  for (const id of prev) {
    if (!id || seen.has(id)) continue
    ordered.push(id)
    seen.add(id)
  }
  return Array.from({ length: count }, (_, i) => ordered[i] ?? null)
}

function fillSlots(keys: string[], layout: Layout): Array<string | null> {
  const count = slotCount(layout)
  return Array.from({ length: count }, (_, i) => keys[i] ?? null)
}

export function RecordingPage() {
  const toast = useToast()
  const navigate = useNavigate()
  const autoFilledRef = useRef(false)

  const [layout, setLayout] = useState<Layout>('9')
  const [slotIds, setSlotIds] = useState<Array<string | null>>([])
  const [selectedSlot, setSelectedSlot] = useState(0)
  const [pickSlot, setPickSlot] = useState<number | null>(null)
  const [pickQuery, setPickQuery] = useState('')
  const [day, setDay] = useState(formatDay())
  const [activeTimeMs, setActiveTimeMs] = useState<number | null>(null)
  const [policyOpen, setPolicyOpen] = useState(false)
  const [eventPlayer, setEventPlayer] = useState<{
    camera: Camera
    channelId?: string
    seekAt?: string
    initialPlayback: boolean
  } | null>(null)

  const camerasQuery = useQuery({
    queryKey: ['cameras', { forRecordingPreview: true }],
    queryFn: () => listCameras({ pageSize: 100 }),
  })

  const cameras = camerasQuery.data?.items ?? []
  const byId = useMemo(
    () => new Map(cameras.map((c) => [String(c.id), c])),
    [cameras],
  )
  const allStreamKeys = useMemo(() => collectStreamKeys(cameras), [cameras])

  const selectedKey = slotIds[selectedSlot] ?? null
  const selectedSrc = selectedKey ? parseSlotKey(selectedKey) : null
  const selectedCamera = selectedSrc
    ? (byId.get(selectedSrc.cameraId) ?? null)
    : null

  const timelineCameraIds = useMemo(() => {
    const ids: string[] = []
    const seen = new Set<string>()
    for (const key of slotIds) {
      if (!key) continue
      const src = parseSlotKey(key)
      if (!src || seen.has(src.cameraId)) continue
      seen.add(src.cameraId)
      ids.push(src.cameraId)
    }
    return ids
  }, [slotIds])

  const { startMs, endMs } = useMemo(() => {
    const [y, m, d] = day.split('-').map(Number)
    const start = new Date(y, (m || 1) - 1, d || 1, 0, 0, 0, 0)
    const end = new Date(start)
    end.setDate(end.getDate() + 1)
    return { startMs: start.getTime(), endMs: end.getTime() }
  }, [day])

  // 若选中日是今天，时间轴上界收到「现在」，避免未来空白
  const timelineEndMs = useMemo(() => {
    const today = localDateKey()
    if (day === today) return Math.min(endMs, Date.now())
    return endMs
  }, [day, endMs])

  const eventsQuery = useQuery({
    queryKey: [
      'review-events',
      {
        forRecording: true,
        cameraIds: timelineCameraIds,
        from: startMs,
        to: timelineEndMs,
      },
    ],
    queryFn: () =>
      listReviewEvents({
        pageSize: 200,
        cameraIds: timelineCameraIds,
        from: new Date(startMs).toISOString(),
        to: new Date(timelineEndMs).toISOString(),
      }),
    enabled: timelineCameraIds.length > 0,
  })

  const timelineEvents = eventsQuery.data?.items ?? []

  const pickCameras = useMemo(() => {
    const q = pickQuery.trim().toLowerCase()
    if (!q) return cameras
    return cameras.filter(
      (c) =>
        c.name.toLowerCase().includes(q) ||
        c.group.toLowerCase().includes(q) ||
        c.ip.includes(q) ||
        (c.deviceId ?? '').includes(q) ||
        (c.channels ?? []).some(
          (ch) =>
            ch.id.toString().includes(q) || (ch.name ?? '').toLowerCase().includes(q),
        ),
    )
  }, [cameras, pickQuery])

  useEffect(() => {
    if (autoFilledRef.current) return
    if (!camerasQuery.isSuccess) return
    autoFilledRef.current = true
    setSlotIds(fillSlots(allStreamKeys, '9'))
    setSelectedSlot(0)
  }, [camerasQuery.isSuccess, allStreamKeys])

  useEffect(() => {
    if (!autoFilledRef.current) return
    setSlotIds((prev) => {
      if (prev.length === slotCount(layout)) return prev
      return resizeSlots(prev, layout, null)
    })
  }, [layout])

  function changeLayout(next: Layout) {
    if (next === layout) return
    autoFilledRef.current = true
    const focusId = slotIds[selectedSlot] ?? null
    setLayout(next)
    setSlotIds((prev) => resizeSlots(prev, next, focusId))
    setSelectedSlot(0)
  }

  function assignSlot(slotIndex: number, src: PreviewSlotSource | null) {
    const key = src ? encodeSlotKey(src) : null
    setSlotIds((prev) => {
      const next = [...prev]
      if (key) {
        for (let i = 0; i < next.length; i++) {
          if (next[i] === key) next[i] = null
        }
      }
      next[slotIndex] = key
      return next
    })
    setSelectedSlot(slotIndex)
    setPickSlot(null)
    setActiveTimeMs(null)
  }

  const seekPlayback = (timeMs: number, eventId: string | null) => {
    setActiveTimeMs(timeMs)
    const event = eventId
      ? timelineEvents.find((e) => e.id === eventId)
      : null
    const camera =
      (event ? byId.get(event.cameraId) : null) ?? selectedCamera

    if (!camera) {
      toast({ body: '请先选择摄像头', type: 'info' })
      return
    }

    const channelId =
      selectedSrc?.cameraId === String(camera.id)
        ? selectedSrc.channelId
        : camera.channels?.[0]
          ? channelBizId(camera.channels[0])
          : camera.channel || undefined
    const seekAt = new Date(Math.max(0, timeMs - 2000)).toISOString()
    const canPlayback = camera.protocol === 'gb28181'

    seedPreviewSource({
      cameraId: String(camera.id),
      channelId,
      openDetail: true,
      openPlayback: canPlayback,
      seekAt: canPlayback ? seekAt : undefined,
    })
    void navigate({ to: '/preview' })
  }

  return (
    <PageLayout
      title="视频回放"
      contentPadding={0}
      headerHeight={52}
      actions={
        <HStack gap={3} vAlign="center" wrap="wrap">
          <DateInput
            label="日期"
            isLabelHidden
            size="sm"
            value={day}
            max={formatDay()}
            onChange={(v) => {
              setDay(v ?? formatDay())
              setActiveTimeMs(null)
            }}
          />
          <SegmentedControl
            label="预览布局"
            size="sm"
            layout="hug"
            value={layout}
            onChange={(v) => changeLayout((v as Layout) || '9')}
          >
            <SegmentedControlItem value="1" label="1 路" />
            <SegmentedControlItem value="4" label="4 路" />
            <SegmentedControlItem value="9" label="9 路" />
          </SegmentedControl>
          <Button
            label="录像策略"
            size="sm"
            variant="ghost"
            clickAction={() => setPolicyOpen(true)}
          />
        </HStack>
      }
    >
      <div className="ff-recording-shell">
        <div className="ff-recording-layout">
          <div className="ff-recording-main">
            <div className="ff-preview-grid" data-layout={layout}>
              {slotIds.map((key, index) => {
                const src = key ? parseSlotKey(key) : null
                const camera = src ? (byId.get(src.cameraId) ?? null) : null
                const selected = index === selectedSlot
                return (
                  <div
                    key={`slot-${index}-${key ?? 'empty'}`}
                    className="ff-recording-slot-wrap"
                  >
                    <PreviewTile
                      slotIndex={index}
                      camera={camera}
                      channelId={src?.channelId}
                      mode="webrtc"
                      muted
                      selected={selected}
                      showAiOverlay={false}
                      frameEpoch={0}
                      variant="playback"
                      onSelect={() => {
                        setSelectedSlot(index)
                        if (!camera) setPickSlot(index)
                      }}
                      onPickSource={() => setPickSlot(index)}
                      onOpenDetail={() => {
                        if (!src || !camera) return
                        setEventPlayer({
                          camera,
                          channelId: src.channelId,
                          seekAt:
                            activeTimeMs != null
                              ? new Date(
                                  Math.max(0, activeTimeMs - 2000),
                                ).toISOString()
                              : undefined,
                          initialPlayback: true,
                        })
                      }}
                    />
                    {selected && activeTimeMs != null ? (
                      <div className="ff-recording-seek-chip">
                        回放 {formatClock(activeTimeMs)}
                      </div>
                    ) : null}
                  </div>
                )
              })}
            </div>
          </div>

          <ReviewTimeline
            events={timelineEvents}
            preferKind="alert"
            startMs={startMs}
            endMs={timelineEndMs}
            activeTimeMs={activeTimeMs}
            onSeek={(p) => {
              if (p.scrubbing) {
                setActiveTimeMs(p.timeMs)
                return
              }
              seekPlayback(p.timeMs, p.eventId)
            }}
          />
        </div>
      </div>

      {eventPlayer ? (
        <div className="ff-recording-event-player">
          <MonitorDetail
            key={`${eventPlayer.camera.id}-${eventPlayer.seekAt ?? day}-${eventPlayer.initialPlayback}`}
            camera={eventPlayer.camera}
            channelId={eventPlayer.channelId}
            showAiOverlay={false}
            initialPlayback
            playbackOnly
            initialDay={day}
            initialSeekAt={eventPlayer.seekAt}
            onClose={() => setEventPlayer(null)}
          />
        </div>
      ) : null}

      <SidePanel
        open={pickSlot != null}
        title={`选择源 · 槽位 ${(pickSlot ?? 0) + 1}`}
        onClose={() => {
          setPickSlot(null)
          setPickQuery('')
        }}
      >
        <VStack gap={3}>
          <TextInput
            label="搜索"
            isLabelHidden
            placeholder="名称 / 分组 / IP / 通道"
            value={pickQuery}
            onChange={setPickQuery}
            hasClear
          />
          <Button
            label="清空槽位"
            size="sm"
            variant="ghost"
            onClick={() => assignSlot(pickSlot!, null)}
          />
          <div className="ff-pick-list">
            {pickCameras.map((c) => {
              const channels = c.channels ?? []
              const multi = c.protocol === 'gb28181' || channels.length > 1
              if (multi && channels.length > 0) {
                return (
                  <div key={c.id} className="ff-pick-group">
                    <div className="ff-pick-group-title">
                      <span>{c.name}</span>
                      <CameraStatusBadge status={c.status} />
                    </div>
                    {channels.map((ch) => {
                      const key = encodeSlotKey({
                        cameraId: String(c.id),
                        channelId: channelBizId(ch),
                      })
                      return (
                        <button
                          key={key}
                          type="button"
                          className="ff-pick-item ff-pick-item--channel"
                          data-selected={
                            slotIds[pickSlot!] === key ? 'true' : undefined
                          }
                          onClick={() =>
                            assignSlot(pickSlot!, {
                              cameraId: String(c.id),
                              channelId: channelBizId(ch),
                            })
                          }
                        >
                          <span className="ff-pick-item-name">
                            {ch.name?.trim() || `通道 ${channelBizId(ch)}`}
                            {ch.isDefault ? ' · 默认' : ''}
                          </span>
                          <span className="ff-pick-item-meta">
                            {channelBizId(ch)}
                          </span>
                        </button>
                      )
                    })}
                  </div>
                )
              }
              const key = encodeSlotKey({ cameraId: String(c.id) })
              return (
                <button
                  key={c.id}
                  type="button"
                  className="ff-pick-item"
                  data-selected={
                    slotIds[pickSlot!] === key ? 'true' : undefined
                  }
                  onClick={() =>
                    assignSlot(pickSlot!, { cameraId: String(c.id) })
                  }
                >
                  <span className="ff-pick-item-name">{c.name}</span>
                  <span className="ff-pick-item-meta">
                    {c.group} · {c.ip || c.deviceId || c.id}
                  </span>
                  <CameraStatusBadge status={c.status} />
                </button>
              )
            })}
            {!pickCameras.length ? (
              <Text type="supporting" color="secondary">
                无匹配源
              </Text>
            ) : null}
          </div>
        </VStack>
      </SidePanel>

      <SidePanel
        open={policyOpen}
        title="录像策略"
        onClose={() => setPolicyOpen(false)}
      >
        <RecordingPolicyPanel />
      </SidePanel>
    </PageLayout>
  )
}
