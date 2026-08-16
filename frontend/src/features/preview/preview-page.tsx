import { useEffect, useMemo, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Button } from '@astryxdesign/core/Button'
import {
  SegmentedControl,
  SegmentedControlItem,
} from '@astryxdesign/core/SegmentedControl'
import { Switch } from '@astryxdesign/core/Switch'
import { TextInput } from '@astryxdesign/core/TextInput'
import { Text } from '@astryxdesign/core/Text'
import { HStack } from '@astryxdesign/core/HStack'
import { VStack } from '@astryxdesign/core/VStack'
import { PageLayout } from '@/components/page-layout'
import { SidePanel } from '@/components/side-panel'
import { CameraStatusBadge } from '@/components/status-badge'
import { listCameras } from '@/apis/devices-api'
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

type Layout = '1' | '4' | '9'

function slotCount(layout: Layout) {
  return layout === '1' ? 1 : layout === '4' ? 4 : 9
}

/** 实时监控默认九宫格；摄像头专属预览按路数选布局 */
function layoutForStreamCount(n: number, cameraOnly = false): Layout {
  if (!cameraOnly) return '9'
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

/** 切换布局时尽量保留已占槽位顺序，不足补空，超出截断 */
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

export function PreviewPage() {
  const autoFilledRef = useRef(false)

  const [layout, setLayout] = useState<Layout>('9')
  const [slotIds, setSlotIds] = useState<Array<string | null>>([])
  const [selectedSlot, setSelectedSlot] = useState(0)
  const [pickSlot, setPickSlot] = useState<number | null>(null)
  const [pickQuery, setPickQuery] = useState('')
  const [detail, setDetail] = useState<PreviewSlotSource | null>(null)
  const [showAiOverlay, setShowAiOverlay] = useState(true)

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
  const byId = useMemo(
    () => new Map(cameras.map((c) => [String(c.id), c])),
    [cameras],
  )

  const allStreamKeys = useMemo(() => collectStreamKeys(cameras), [cameras])

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

    let ordered: string[]
    if (seedCameraOnly && seedCameraId) {
      const cam = byId.get(seedCameraId)
      ordered = cam ? collectStreamKeys([cam]) : []
    } else if (seedKey) {
      ordered = [seedKey, ...allStreamKeys.filter((k) => k !== seedKey)]
    } else {
      ordered = allStreamKeys
    }

    const nextLayout = layoutForStreamCount(ordered.length, seedCameraOnly)
    autoFilledRef.current = true
    setLayout(nextLayout)
    setSlotIds(fillSlots(ordered, nextLayout))
    setSelectedSlot(0)
  }, [
    camerasQuery.isSuccess,
    allStreamKeys,
    seedKey,
    seedCameraOnly,
    seedCameraId,
    byId,
  ])

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

  useEffect(() => {
    if (!autoFilledRef.current) return
    setSlotIds((prev) => {
      if (prev.length === slotCount(layout)) return prev
      return resizeSlots(prev, layout, null)
    })
  }, [layout])

  const detailCamera = detail ? (byId.get(detail.cameraId) ?? null) : null

  function changeLayout(next: Layout) {
    if (next === layout) return
    autoFilledRef.current = true
    const focusId = slotIds[selectedSlot] ?? seedKey ?? null
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
  }

  function openDetail(src: PreviewSlotSource) {
    setDetail(src)
  }

  return (
    <PageLayout
      title="实时监控"
      contentPadding={2}
      actions={
        <HStack gap={3} vAlign="center" wrap="wrap">
          <Switch
            label="AI 叠框"
            value={showAiOverlay}
            onChange={setShowAiOverlay}
          />
          <SegmentedControl
            label="预览布局"
            size="sm"
            value={layout}
            onChange={(v) => changeLayout((v as Layout) || '9')}
          >
            <SegmentedControlItem value="1" label="1 路" />
            <SegmentedControlItem value="4" label="4 路" />
            <SegmentedControlItem value="9" label="9 路" />
          </SegmentedControl>
        </HStack>
      }
    >
      <div className="ff-preview-grid" data-layout={layout}>
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
              frameEpoch={0}
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
          }}
        />
      ) : null}
    </PageLayout>
  )
}
