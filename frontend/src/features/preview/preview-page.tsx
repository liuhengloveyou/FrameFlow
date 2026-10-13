import { useEffect, useMemo, useRef, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { Button } from '@astryxdesign/core/Button'
import { Switch } from '@astryxdesign/core/Switch'
import { TextInput } from '@astryxdesign/core/TextInput'
import { Text } from '@astryxdesign/core/Text'
import { VStack } from '@astryxdesign/core/VStack'
import {
  ArrowsPointingOutIcon,
  BellAlertIcon,
  ClipboardDocumentCheckIcon,
  CpuChipIcon,
  Squares2X2Icon,
  VideoCameraIcon,
} from '@heroicons/react/24/outline'
import { PageLayout } from '@/components/page-layout'
import { SidePanel } from '@/components/side-panel'
import { InlineStat, InlineStatGroup } from '@/components/inline-stat'
import {
  GridPager,
  LayoutSwitcher,
  type ScreenLayout,
} from '@/components/layout-switcher'
import { CameraStatusBadge } from '@/components/status-badge'
import { listCameras } from '@/apis/devices-api'
import { listAlarms } from '@/apis/alarms-api'
import { listReviewEvents } from '@/apis/review-api'
import { takePreviewSeed } from '@/lib/preview-seed'
import {
  encodeSlotKey,
  channelBizId,
  parseSlotKey,
  type PreviewSlotSource,
} from '@/lib/preview-source'
import type { Camera } from '@/types/api'
import { MonitorDetail } from './monitor-detail'
import { PreviewTile } from './preview-tile'

function slotCount(layout: ScreenLayout) {
  return layout === '1' ? 1 : layout === '4' ? 4 : layout === '9' ? 9 : 16
}

/** 实时监控默认 2x2；摄像头专属预览按路数选布局 */
function layoutForStreamCount(n: number, cameraOnly = false): ScreenLayout {
  if (!cameraOnly) return '4'
  if (n <= 1) return '1'
  if (n <= 4) return '4'
  return '9'
}

/** 展开系统内可播流：有通道则每通道一路，否则设备本身一路 */
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

/** 可分析通道数：设备通道之和（无通道表按 1 路计） */
function countAiChannels(cameras: Camera[]) {
  let n = 0
  for (const c of cameras) {
    if (c.channels?.length) n += c.channels.length
    else n += 1
  }
  return n
}

/** 按页填充槽位：第 page 页的 count 路，不足补 null */
function fillFromPage(
  keys: string[],
  layout: ScreenLayout,
  page: number,
): Array<string | null> {
  const count = slotCount(layout)
  const start = (page - 1) * count
  return Array.from({ length: count }, (_, i) => keys[start + i] ?? null)
}

export function PreviewPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const gridRef = useRef<HTMLDivElement>(null)
  const autoFilledRef = useRef(false)
  const orderedKeysRef = useRef<string[]>([])

  const [layout, setLayout] = useState<ScreenLayout>('4')
  const [page, setPage] = useState(1)
  const [slotIds, setSlotIds] = useState<Array<string | null>>([])
  const [selectedSlot, setSelectedSlot] = useState(0)
  const [pickSlot, setPickSlot] = useState<number | null>(null)
  const [pickQuery, setPickQuery] = useState('')
  const [detail, setDetail] = useState<PreviewSlotSource | null>(null)
  const [showAiOverlay, setShowAiOverlay] = useState(true)
  const [gridFullscreen, setGridFullscreen] = useState(false)

  const [seed] = useState(() => takePreviewSeed())
  const seedKey = seed
    ? encodeSlotKey({
        cameraId: seed.cameraId,
        channelId: seed.channelId,
      })
    : null
  const seedCameraOnly = seed?.cameraOnly === true
  const seedCameraId = seed?.cameraId ?? null
  const seedOpenDetail = seed?.openDetail === true
  const seedOpenPlayback = seed?.openPlayback === true
  const seedSeekAt = seed?.seekAt
  const detailOpenedRef = useRef(false)
  const [detailBoot, setDetailBoot] = useState<{
    openPlayback: boolean
    seekAt?: string
  } | null>(null)

  const camerasQuery = useQuery({
    queryKey: ['cameras', { forPreview: true }],
    queryFn: () => listCameras({ pageSize: 100 }),
  })

  const cameras = camerasQuery.data?.items ?? []
  const totalCameras = camerasQuery.data?.total ?? cameras.length
  const byId = useMemo(
    () => new Map(cameras.map((c) => [String(c.id), c])),
    [cameras],
  )

  const allStreamKeys = useMemo(() => collectStreamKeys(cameras), [cameras])

  /** 指标：在线设备 / 实时告警 / 今日识别事件 / AI 分析通道 */
  const onlineCameras = useMemo(
    () =>
      cameras.filter((c) => c.status === 'online' || c.status === 'running')
        .length,
    [cameras],
  )
  const aiChannels = useMemo(() => countAiChannels(cameras), [cameras])

  const alarmsCountQuery = useQuery({
    queryKey: ['open-alarms-count'],
    queryFn: () => listAlarms({ status: 'open', pageSize: 1 }),
    refetchInterval: 30_000,
    staleTime: 10_000,
  })

  const todayEventsQuery = useQuery({
    queryKey: ['review-events', { forMonitorToday: true }],
    queryFn: () => {
      const start = new Date()
      start.setHours(0, 0, 0, 0)
      return listReviewEvents({
        from: start.toISOString(),
        pageSize: 1,
      })
    },
    staleTime: 30_000,
  })

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

  // 首屏：按种子排序流，落第 1 页
  useEffect(() => {
    if (autoFilledRef.current) return
    if (!camerasQuery.isSuccess) return

    let ordered: string[]
    if (seedCameraOnly && seedCameraId) {
      const cam = byId.get(seedCameraId)
      ordered = cam ? collectStreamKeys([cam]) : []
    } else if (seedKey) {
      ordered = [seedKey, ...allStreamKeys.filter((k) => k !== seedKey)]
    } else {
      ordered = allStreamKeys
    }
    orderedKeysRef.current = ordered

    const nextLayout = layoutForStreamCount(ordered.length, seedCameraOnly)
    autoFilledRef.current = true
    setLayout(nextLayout)
    setPage(1)
    setSlotIds(fillFromPage(ordered, nextLayout, 1))
    setSelectedSlot(0)
  }, [
    camerasQuery.isSuccess,
    allStreamKeys,
    seedKey,
    seedCameraOnly,
    seedCameraId,
    byId,
  ])

  // 种子直达详情（从地图/设备页跳入）
  useEffect(() => {
    if (detailOpenedRef.current) return
    if (!seedOpenDetail || !seedKey) return
    if (!camerasQuery.isSuccess) return
    const cam = byId.get(seedCameraId ?? '')
    if (!cam) return
    detailOpenedRef.current = true
    setDetail({
      cameraId: String(cam.id),
      channelId: seed?.channelId,
    })
    setDetailBoot({
      openPlayback: seedOpenPlayback,
      seekAt: seedSeekAt,
    })
  }, [
    camerasQuery.isSuccess,
    seedOpenDetail,
    seedOpenPlayback,
    seedSeekAt,
    seedKey,
    seedCameraId,
    seed?.channelId,
    byId,
  ])

  const pageCount = Math.max(
    1,
    Math.ceil((orderedKeysRef.current.length || allStreamKeys.length) / slotCount(layout)),
  )

  // 数据变化导致页数变小时，夹回有效页
  useEffect(() => {
    if (!autoFilledRef.current) return
    if (page > pageCount) {
      const next = pageCount
      setPage(next)
      setSlotIds(fillFromPage(orderedKeysRef.current, layout, next))
    }
  }, [page, pageCount, layout])

  const detailCamera = detail ? (byId.get(detail.cameraId) ?? null) : null

  function changeLayout(next: ScreenLayout) {
    if (next === layout) return
    autoFilledRef.current = true
    setLayout(next)
    setPage(1)
    setSlotIds(fillFromPage(orderedKeysRef.current, next, 1))
    setSelectedSlot(0)
  }

  function gotoPage(next: number) {
    const clamped = Math.min(Math.max(1, next), pageCount)
    if (clamped === page) return
    setPage(clamped)
    setSlotIds(fillFromPage(orderedKeysRef.current, layout, clamped))
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
  }

  function openDetail(src: PreviewSlotSource) {
    setDetail(src)
  }

  // 宫格全屏（浏览器原生）
  useEffect(() => {
    const onFsChange = () => {
      const el = gridRef.current
      setGridFullscreen(Boolean(el && document.fullscreenElement === el))
    }
    document.addEventListener('fullscreenchange', onFsChange)
    return () => document.removeEventListener('fullscreenchange', onFsChange)
  }, [])

  function toggleGridFullscreen() {
    const el = gridRef.current
    if (!el) return
    if (document.fullscreenElement) {
      void document.exitFullscreen().catch(() => {})
    } else {
      void el.requestFullscreen?.().catch(() => {
        // 浏览器拒绝全屏时退回单路大图（打开当前选中槽详情）
        const key = slotIds[selectedSlot]
        const src = key ? parseSlotKey(key) : null
        if (src && byId.get(src.cameraId)) openDetail(src)
      })
    }
  }

  return (
    <PageLayout title="实时监控" badge="全部设备" contentPadding={2}>
      <div className="ff-screen-bar">
        <span className="ff-screen-bar-label">
          <Squares2X2Icon aria-hidden />
          画面布局
        </span>
        <LayoutSwitcher value={layout} onChange={changeLayout} />
        <InlineStatGroup>
          <InlineStat
            icon={<VideoCameraIcon />}
            value={onlineCameras}
            unit={`/ ${totalCameras}`}
            label="在线设备"
            tone="ok"
            title="查看设备管理"
            onClick={() => void navigate({ to: '/devices' })}
          />
          <InlineStat
            icon={<BellAlertIcon />}
            value={alarmsCountQuery.data?.total ?? 0}
            label="实时告警"
            tone="warn"
            title="查看报警记录"
            onClick={() => void navigate({ to: '/alarms' })}
          />
          <InlineStat
            icon={<ClipboardDocumentCheckIcon />}
            value={todayEventsQuery.data?.total ?? 0}
            label="今日识别事件"
            title="前往事件核查"
            onClick={() => void navigate({ to: '/review' })}
          />
          <InlineStat
            icon={<CpuChipIcon />}
            value={aiChannels}
            label="AI 分析通道"
            tone="teal"
            title="前往算法接入"
            onClick={() => void navigate({ to: '/algorithms' })}
          />
        </InlineStatGroup>
        <div className="ff-screen-bar-right">
          <Switch
            label="AI 叠框"
            value={showAiOverlay}
            onChange={setShowAiOverlay}
          />
          <GridPager
            page={page}
            pageCount={pageCount}
            onChange={gotoPage}
          />
          <button
            type="button"
            className="ff-topbar-iconbtn"
            aria-label="全屏"
            title="全屏"
            onClick={toggleGridFullscreen}
          >
            <ArrowsPointingOutIcon aria-hidden />
          </button>
        </div>
      </div>

      <div
        ref={gridRef}
        className="ff-preview-grid"
        data-layout={layout}
        data-fullscreen={gridFullscreen ? 'true' : undefined}
      >
        {slotIds.map((key, index) => {
          const src = key ? parseSlotKey(key) : null
          const camera = src ? (byId.get(src.cameraId) ?? null) : null
          return (
            <PreviewTile
              key={`slot-${index}-${key ?? 'empty'}`}
              slotIndex={index}
              camera={camera}
              channelId={src?.channelId}
              mode="webrtc"
              muted
              selected={index === selectedSlot}
              showAiOverlay={showAiOverlay}
              frameEpoch={page}
              suspended={detail != null}
              onSelect={() => {
                setSelectedSlot(index)
                if (!camera) setPickSlot(index)
              }}
              onPickSource={() => setPickSlot(index)}
              onOpenDetail={() => {
                if (src && camera) openDetail(src)
              }}
            />
          )
        })}
      </div>

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

      {detail && detailCamera ? (
        <MonitorDetail
          camera={detailCamera}
          channelId={detail.channelId}
          showAiOverlay={showAiOverlay}
          initialPlayback={detailBoot?.openPlayback === true}
          initialSeekAt={detailBoot?.seekAt}
          onClose={() => {
            setDetail(null)
            setDetailBoot(null)
            void queryClient.invalidateQueries({ queryKey: ['cameras'] })
          }}
        />
      ) : null}
    </PageLayout>
  )
}
