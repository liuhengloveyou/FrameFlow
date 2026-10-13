import { useEffect, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Button } from '@astryxdesign/core/Button'
import { NumberInput } from '@astryxdesign/core/NumberInput'
import { TextInput } from '@astryxdesign/core/TextInput'
import { Selector } from '@astryxdesign/core/Selector'
import {
  SegmentedControl,
  SegmentedControlItem,
} from '@astryxdesign/core/SegmentedControl'
import { Switch } from '@astryxdesign/core/Switch'
import { HStack } from '@astryxdesign/core/HStack'
import { VStack } from '@astryxdesign/core/VStack'
import { Text } from '@astryxdesign/core/Text'
import { useToast } from '@astryxdesign/core/Toast'
import {
  ArrowsPointingOutIcon,
  BellAlertIcon,
  MapIcon,
  MapPinIcon,
  RectangleStackIcon,
  SignalIcon,
} from '@heroicons/react/24/outline'
import { PageLayout } from '@/components/page-layout'
import { PageToolbar } from '@/components/page-toolbar'
import { InlineStat, InlineStatGroup } from '@/components/inline-stat'
import { ToneBadge } from '@/components/tone-badge'
import { SidePanel } from '@/components/side-panel'
import { CameraStatusBadge } from '@/components/status-badge'
import { MapBaseMap } from './map-base-map'
import { listAlarms } from '@/apis/alarms-api'
import { listCameras } from '@/apis/devices-api'
import { listMapElements, nextMapElementId, saveMapElements } from '@/apis/map-api'
import { getErrorMessage } from '@/lib/http'
import { seedPreviewSource } from '@/lib/preview-seed'
import type { Camera, MapElement, MapElementKind, MapLayer } from '@/types/api'

const LAYER_LABEL: Record<MapLayer, string> = {
  campus: '园区总览',
  building: '一号楼内部',
  parking: '停车场',
}

const KIND_LABEL: Record<MapElementKind, string> = {
  building: '楼栋',
  road: '道路',
  camera: '摄像头',
}

function clamp(n: number, min: number, max: number) {
  return Math.min(max, Math.max(min, n))
}

export function MapPage() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const worldRef = useRef<HTMLDivElement>(null)
  const dragPanRef = useRef<{ x: number; y: number; panX: number; panY: number } | null>(null)
  const dragElRef = useRef<{
    id: string
    startX: number
    startY: number
    origX: number
    origY: number
  } | null>(null)

  const [layer, setLayer] = useState<MapLayer>('campus')
  const [zoom, setZoom] = useState(1)
  const [pan, setPan] = useState({ x: 0, y: 0 })
  const [editMode, setEditMode] = useState(false)
  const [elements, setElements] = useState<MapElement[]>([])
  const [dirty, setDirty] = useState(false)
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [selectedCameraId, setSelectedCameraId] = useState<string | null>(null)
  /** 右侧列视图：设备点位 / 告警热力 */
  const [view, setView] = useState<'points' | 'heat'>('points')
  const [pointQuery, setPointQuery] = useState('')
  /** 内置示例底图：园区尚未接入真实地理底图，先用它撑起画布 */
  const [showBaseMap, setShowBaseMap] = useState(true)
  const shellRef = useRef<HTMLDivElement>(null)

  const mapQuery = useQuery({
    queryKey: ['map-elements'],
    queryFn: listMapElements,
  })
  const camerasQuery = useQuery({
    queryKey: ['cameras', { forMap: true }],
    queryFn: () => listCameras({ pageSize: 50 }),
  })
  const alarmsQuery = useQuery({
    queryKey: ['alarms', { forMap: true, status: 'open' }],
    queryFn: () => listAlarms({ status: 'open', pageSize: 100 }),
    refetchInterval: 15_000,
  })

  useEffect(() => {
    if (mapQuery.data && !dirty) {
      setElements(structuredClone(mapQuery.data))
    }
  }, [mapQuery.data, dirty])

  const cameras = camerasQuery.data?.items ?? []
  const cameraById = useMemo(
    () => new Map(cameras.map((c) => [String(c.id), c] as const)),
    [cameras],
  )
  const alarmingCameraIds = useMemo(() => {
    const set = new Set<string>()
    for (const a of alarmsQuery.data?.items ?? []) {
      if (a.cameraId) set.add(a.cameraId)
    }
    return set
  }, [alarmsQuery.data])

  const layerElements = useMemo(
    () => elements.filter((e) => e.layer === layer),
    [elements, layer],
  )

  const selected = elements.find((e) => e.id === selectedId) ?? null
  const selectedCamera =
    selected?.kind === 'camera' && selected.cameraId
      ? (cameraById.get(selected.cameraId) ?? null)
      : selectedCameraId
        ? (cameraById.get(selectedCameraId) ?? null)
        : null

  const boundCameraIds = useMemo(
    () =>
      new Set(
        elements
          .filter((e) => e.kind === 'camera' && e.cameraId)
          .map((e) => e.cameraId!),
      ),
    [elements],
  )

  const unboundCameras = cameras.filter((c) => !boundCameraIds.has(String(c.id)))

  const stats = useMemo(() => {
    const cams = layerElements.filter((e) => e.kind === 'camera')
    const online = cams.filter((e) => {
      const c = e.cameraId ? cameraById.get(e.cameraId) : null
      return c?.status === 'online' || c?.status === 'running'
    }).length
    const alarming = cams.filter((e) => e.cameraId && alarmingCameraIds.has(e.cameraId)).length
    return {
      total: cams.length,
      online,
      alarming,
      buildings: layerElements.filter((e) => e.kind === 'building').length,
      roads: layerElements.filter((e) => e.kind === 'road').length,
    }
  }, [layerElements, cameraById, alarmingCameraIds])

  const saveMutation = useMutation({
    mutationFn: () => saveMapElements(elements),
    onSuccess: async (saved) => {
      setElements(saved)
      setDirty(false)
      toast({ body: '地图元素已保存', type: 'info' })
      await queryClient.invalidateQueries({ queryKey: ['map-elements'] })
    },
    onError: (error) => toast({ body: getErrorMessage(error), type: 'error' }),
  })

  function resetView() {
    setZoom(1)
    setPan({ x: 0, y: 0 })
  }

  function changeLayer(next: MapLayer) {
    setLayer(next)
    resetView()
    setSelectedId(null)
    setSelectedCameraId(null)
  }

  function patchElement(id: string, patch: Partial<MapElement>) {
    setElements((prev) => prev.map((e) => (e.id === id ? { ...e, ...patch } : e)))
    setDirty(true)
  }

  function addElement(kind: MapElementKind) {
    const id = nextMapElementId(kind === 'building' ? 'b' : kind === 'road' ? 'r' : 'c')
    const base: MapElement = {
      id,
      kind,
      layer,
      label:
        kind === 'building' ? '新建楼栋' : kind === 'road' ? '新建道路' : '摄像头点位',
      x: 40,
      y: 40,
      width: kind === 'road' ? 36 : kind === 'building' ? 18 : undefined,
      height: kind === 'road' ? 5 : kind === 'building' ? 16 : undefined,
    }
    if (kind === 'camera' && unboundCameras[0]) {
      base.cameraId = String(unboundCameras[0].id)
      base.label = unboundCameras[0].name
    }
    setElements((prev) => [...prev, base])
    setSelectedId(id)
    setSelectedCameraId(null)
    setDirty(true)
    setEditMode(true)
  }

  function removeSelected() {
    if (!selected) return
    setElements((prev) => prev.filter((e) => e.id !== selected.id))
    setSelectedId(null)
    setDirty(true)
  }

  function discardEdits() {
    if (mapQuery.data) setElements(structuredClone(mapQuery.data))
    setDirty(false)
    setSelectedId(null)
    toast({ body: '已丢弃未保存修改', type: 'info' })
  }

  function beginElementDrag(e: React.MouseEvent, el: MapElement) {
    if (!editMode) return
    e.stopPropagation()
    e.preventDefault()
    setSelectedId(el.id)
    dragElRef.current = {
      id: el.id,
      startX: e.clientX,
      startY: e.clientY,
      origX: el.x,
      origY: el.y,
    }
  }

  function onCanvasMouseMove(e: React.MouseEvent) {
    if (dragElRef.current && worldRef.current) {
      const rect = worldRef.current.getBoundingClientRect()
      if (rect.width <= 0 || rect.height <= 0) return
      const dx = ((e.clientX - dragElRef.current.startX) / rect.width) * 100
      const dy = ((e.clientY - dragElRef.current.startY) / rect.height) * 100
      const el = elements.find((x) => x.id === dragElRef.current!.id)
      const w = el?.width ?? 0
      const h = el?.height ?? 0
      const isPin = el?.kind === 'camera'
      patchElement(dragElRef.current.id, {
        x: clamp(dragElRef.current.origX + dx, 0, isPin ? 100 : 100 - w),
        y: clamp(dragElRef.current.origY + dy, 0, isPin ? 100 : 100 - h),
      })
      return
    }
    if (!dragPanRef.current) return
    setPan({
      x: dragPanRef.current.panX + (e.clientX - dragPanRef.current.x),
      y: dragPanRef.current.panY + (e.clientY - dragPanRef.current.y),
    })
  }

  function endDrag() {
    dragElRef.current = null
    dragPanRef.current = null
  }

  const shapes = layerElements.filter((e) => e.kind === 'building' || e.kind === 'road')
  const pins = layerElements.filter((e) => e.kind === 'camera')

  /** 右侧列：按名称 / 区域 / 摄像头名过滤 */
  const filteredPins = useMemo(() => {
    const q = pointQuery.trim().toLowerCase()
    if (!q) return pins
    return pins.filter((el) => {
      const cam = el.cameraId ? cameraById.get(el.cameraId) : null
      return (
        el.label.toLowerCase().includes(q) ||
        (cam?.name ?? '').toLowerCase().includes(q) ||
        (cam?.group ?? '').toLowerCase().includes(q)
      )
    })
  }, [pins, pointQuery, cameraById])

  return (
    <PageLayout
      title="地图空间"
      badge={`已上图 ${boundCameraIds.size} / ${cameras.length}`}
      contentPadding={3}
      toolbar={
        <PageToolbar
          filters={
            <>
              <TextInput
                label="点位"
                isLabelHidden
                placeholder="搜索点位名称 / 区域"
                value={pointQuery}
                onChange={setPointQuery}
                hasClear
              />
              <SegmentedControl
                label="地图层级"
                value={layer}
                onChange={(v) => changeLayer((v as MapLayer) || 'campus')}
              >
                <SegmentedControlItem value="campus" label="园区总览" />
                <SegmentedControlItem value="building" label="一号楼" />
                <SegmentedControlItem value="parking" label="停车场" />
              </SegmentedControl>
              <SegmentedControl
                label="列视图"
                value={view}
                onChange={(v) => setView((v as 'points' | 'heat') || 'points')}
              >
                <SegmentedControlItem value="points" label="设备点位" />
                <SegmentedControlItem value="heat" label="告警热力" />
              </SegmentedControl>
            </>
          }
          stats={
            <InlineStatGroup>
              <InlineStat
                icon={<MapPinIcon />}
                value={stats.total}
                label="本层点位"
                title="当前层级已布点的摄像头"
              />
              <InlineStat
                icon={<SignalIcon />}
                value={stats.online}
                label="在线"
                tone="ok"
              />
              <InlineStat
                icon={<BellAlertIcon />}
                value={stats.alarming}
                label="告警中"
                tone="warn"
              />
              <InlineStat
                icon={<RectangleStackIcon />}
                value={unboundCameras.length}
                label="未上图"
                tone="teal"
                title="尚未绑定到地图的摄像头"
              />
            </InlineStatGroup>
          }
          actions={
            <>
              <Switch
                label="编辑地图"
                value={editMode}
                onChange={(v) => {
                  setEditMode(v)
                  if (!v) {
                    setSelectedId(null)
                  }
                }}
              />
              {editMode ? (
                <>
                  <Button
                    label="添加楼栋"
                    size="sm"
                    variant="secondary"
                    onClick={() => addElement('building')}
                  />
                  <Button
                    label="添加道路"
                    size="sm"
                    variant="secondary"
                    onClick={() => addElement('road')}
                  />
                  <Button
                    label="添加摄像头"
                    size="sm"
                    variant="secondary"
                    isDisabled={unboundCameras.length === 0}
                    onClick={() => addElement('camera')}
                  />
                  <Button
                    label="保存"
                    size="sm"
                    variant="primary"
                    isDisabled={!dirty}
                    isLoading={saveMutation.isPending}
                    clickAction={async () => {
                      await saveMutation.mutateAsync()
                    }}
                  />
                  <Button
                    label="丢弃"
                    size="sm"
                    variant="ghost"
                    isDisabled={!dirty}
                    onClick={discardEdits}
                  />
                </>
              ) : null}
              <button
                type="button"
                className="ff-topbar-iconbtn"
                aria-label="全屏"
                title="全屏"
                onClick={() => {
                  const el = shellRef.current
                  if (!el) return
                  if (document.fullscreenElement) {
                    void document.exitFullscreen().catch(() => {})
                  } else {
                    void el.requestFullscreen?.().catch(() => {})
                  }
                }}
              >
                <ArrowsPointingOutIcon aria-hidden />
              </button>
            </>
          }
        />
      }
    >
      <div className="ff-map-shell" ref={shellRef}>
        <div className="ff-map-main">
          <div
            className="ff-map-canvas"
            data-layer={layer}
            data-edit={editMode ? 'true' : undefined}
            onWheel={(e) => {
              e.preventDefault()
              setZoom((z) => Math.min(2.2, Math.max(0.7, z + (e.deltaY > 0 ? -0.08 : 0.08))))
            }}
            onMouseDown={(e) => {
              if (dragElRef.current) return
              if ((e.target as HTMLElement).closest('.ff-map-pin, .ff-map-shape')) return
              if (editMode) {
                setSelectedId(null)
                setSelectedCameraId(null)
              }
              dragPanRef.current = { x: e.clientX, y: e.clientY, panX: pan.x, panY: pan.y }
            }}
            onMouseMove={onCanvasMouseMove}
            onMouseUp={endDrag}
            onMouseLeave={endDrag}
          >
            <div
              ref={worldRef}
              className="ff-map-world"
              data-view={view}
              data-basemap={showBaseMap ? 'true' : undefined}
              style={{
                transform: `translate(${pan.x}px, ${pan.y}px) scale(${zoom})`,
              }}
            >
              <div className="ff-map-layer" aria-hidden />
              {showBaseMap ? <MapBaseMap layer={layer} /> : null}
              <div className="ff-map-label ff-map-label-n">{LAYER_LABEL[layer]}</div>

              {shapes.map((el) => {
                const active = selectedId === el.id
                return (
                  <button
                    key={el.id}
                    type="button"
                    className="ff-map-shape"
                    data-kind={el.kind}
                    data-active={active ? 'true' : undefined}
                    data-editable={editMode ? 'true' : undefined}
                    style={{
                      left: `${el.x}%`,
                      top: `${el.y}%`,
                      width: `${el.width ?? 10}%`,
                      height: `${el.height ?? 10}%`,
                    }}
                    onMouseDown={(e) => beginElementDrag(e, el)}
                    onClick={(e) => {
                      e.stopPropagation()
                      setSelectedId(el.id)
                      setSelectedCameraId(null)
                    }}
                  >
                    {el.label}
                  </button>
                )
              })}

              {pins.map((el) => {
                const camera = el.cameraId ? cameraById.get(el.cameraId) : undefined
                const active = selectedId === el.id || selectedCameraId === el.cameraId
                const alarming = Boolean(el.cameraId && alarmingCameraIds.has(el.cameraId))
                return (
                  <button
                    key={el.id}
                    type="button"
                    className="ff-map-pin"
                    data-status={camera?.status ?? 'offline'}
                    data-alarm={alarming ? 'true' : undefined}
                    data-active={active ? 'true' : undefined}
                    data-editable={editMode ? 'true' : undefined}
                    data-dim={view === 'heat' && !alarming ? 'true' : undefined}
                    style={{ left: `${el.x}%`, top: `${el.y}%` }}
                    onMouseDown={(e) => beginElementDrag(e, el)}
                    onClick={(e) => {
                      e.stopPropagation()
                      setSelectedId(el.id)
                      if (!editMode && el.cameraId) setSelectedCameraId(el.cameraId)
                    }}
                    title={alarming ? `${el.label} · 报警中` : el.label}
                  >
                    <span className="ff-map-pin-dot" />
                    <span className="ff-map-pin-name">{el.label}</span>
                  </button>
                )
              })}
            </div>

            {/* 图例浮层 */}
            <div className="ff-map-legend-float">
              <span className="ff-map-legend-float-title">
                {view === 'heat' ? '告警热力' : '点位图例'}
              </span>
              <span className="ff-map-legend-row">
                <i style={{ background: 'var(--ff-ok)' }} />
                在线 {stats.online}/{stats.total}
              </span>
              <span className="ff-map-legend-row">
                <i style={{ background: 'var(--ff-warn)' }} />
                告警 {stats.alarming}
              </span>
              <span className="ff-map-legend-row">
                <i style={{ background: 'var(--ff-off)' }} />
                离线
              </span>
              {editMode ? (
                <span className="ff-map-legend-row" style={{ color: 'var(--ff-ink-3)' }}>
                  编辑中 · 拖拽移动
                </span>
              ) : null}
            </div>

            {/* 缩放控件 */}
            <div className="ff-map-zoom-ctl">
              <button
                type="button"
                aria-label="放大"
                title="放大"
                onClick={() => setZoom((z) => Math.min(2.2, z + 0.15))}
              >
                <svg viewBox="0 0 16 16" fill="none" aria-hidden>
                  <path d="M8 3v10M3 8h10" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
                </svg>
              </button>
              <button
                type="button"
                aria-label="缩小"
                title="缩小"
                onClick={() => setZoom((z) => Math.max(0.7, z - 0.15))}
              >
                <svg viewBox="0 0 16 16" fill="none" aria-hidden>
                  <path d="M3 8h10" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
                </svg>
              </button>
              <button
                type="button"
                aria-label="重置视图"
                title="重置视图"
                onClick={resetView}
              >
                <svg viewBox="0 0 16 16" fill="none" aria-hidden>
                  <path
                    d="M2.5 8a5.5 5.5 0 1 1 1.6 3.9M2.5 8V4.5M2.5 8H6"
                    stroke="currentColor"
                    strokeWidth="1.5"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  />
                </svg>
              </button>
            </div>

            {/* 指北针：与图层无关的固定装饰 */}
            <div className="ff-map-compass" aria-hidden>
              <svg viewBox="0 0 24 24" fill="none">
                <path d="M12 3.2 15.4 12.6 12 11.1 8.6 12.6z" fill="var(--ff-danger)" />
                <path d="M12 20.8 8.6 11.4 12 12.9 15.4 11.4z" fill="#cdd6cb" />
              </svg>
            </div>

            {/* 示例底图开关：明确说明画布用的是内置演示底图 */}
            <button
              type="button"
              className="ff-map-basemap-chip"
              data-on={showBaseMap ? 'true' : undefined}
              aria-pressed={showBaseMap}
              title={
                showBaseMap
                  ? '当前画布为内置示例底图（非真实地理数据），点击可隐藏'
                  : '显示内置示例底图（非真实地理数据）'
              }
              onClick={() => setShowBaseMap((v) => !v)}
            >
              <MapIcon aria-hidden />
              {showBaseMap ? '示例底图' : '显示示例底图'}
            </button>
          </div>
        </div>

        {/* 右侧：设备点位列表面板 */}
        <aside className="ff-map-side">
          <div className="ff-map-side-head">
            <span className="ff-map-side-title">
              <MapPinIcon aria-hidden />
              设备点位
            </span>
            <span className="ff-map-side-count">{filteredPins.length}</span>
            {pointQuery ? (
              <button
                type="button"
                className="ff-search-chip"
                style={{ marginLeft: 'auto' }}
                onClick={() => setPointQuery('')}
              >
                全部
              </button>
            ) : null}
          </div>
          <div className="ff-map-side-list">
            {filteredPins.map((el) => {
              const camera = el.cameraId ? cameraById.get(el.cameraId) : undefined
              const alarming = Boolean(el.cameraId && alarmingCameraIds.has(el.cameraId))
              const active =
                selectedId === el.id || selectedCameraId === el.cameraId
              const statusText = alarming
                ? '告警中'
                : camera?.status === 'online' || camera?.status === 'running'
                  ? '在线'
                  : camera?.status === 'error'
                    ? '异常'
                    : '离线'
              return (
                <button
                  key={el.id}
                  type="button"
                  className="ff-map-point"
                  data-active={active ? 'true' : undefined}
                  data-alarm={alarming ? 'true' : undefined}
                  onClick={() => {
                    setSelectedId(el.id)
                    if (!editMode && el.cameraId) setSelectedCameraId(el.cameraId)
                  }}
                >
                  <span
                    className="ff-map-point-dot"
                    data-status={camera?.status ?? 'offline'}
                    data-alarm={alarming ? 'true' : undefined}
                  />
                  <span className="ff-map-point-body">
                    <span className="ff-map-point-name">{el.label}</span>
                    <span className="ff-map-point-meta">
                      {camera?.group || '未绑定区域'} · {statusText}
                    </span>
                  </span>
                </button>
              )
            })}
            {!filteredPins.length ? (
              pins.length || pointQuery ? (
                <p className="ff-pb-empty">无匹配点位</p>
              ) : (
                <div className="ff-map-side-empty">
                  <MapPinIcon aria-hidden />
                  <p>本层还没有点位</p>
                  <span>
                    打开工具条上的「编辑地图」，用「添加摄像头」把设备拖到示例底图上即可。
                  </span>
                </div>
              )
            ) : null}
          </div>
        </aside>
      </div>

      <SidePanel
        open={Boolean(selected) || Boolean(selectedCamera && !editMode)}
        title={
          editMode && selected
            ? `编辑${KIND_LABEL[selected.kind]} · ${selected.label}`
            : (selectedCamera?.name ?? '摄像头')
        }
        onClose={() => {
          setSelectedId(null)
          setSelectedCameraId(null)
        }}
        footer={
          editMode && selected ? (
            <HStack gap={2} justify="end">
              <Button
                label="删除元素"
                size="sm"
                variant="destructive"
                onClick={removeSelected}
              />
            </HStack>
          ) : null
        }
      >
        {editMode && selected ? (
          <ElementEditor
            element={selected}
            cameras={cameras}
            unboundCameras={unboundCameras}
            onChange={(patch) => patchElement(selected.id, patch)}
          />
        ) : selectedCamera ? (
          <CameraDetail camera={selectedCamera} layer={layer} />
        ) : null}
      </SidePanel>
    </PageLayout>
  )
}

function ElementEditor({
  element,
  cameras,
  unboundCameras,
  onChange,
}: {
  element: MapElement
  cameras: Camera[]
  unboundCameras: Camera[]
  onChange: (patch: Partial<MapElement>) => void
}) {
  const cameraOptions = [
    ...unboundCameras.map((c) => ({ label: c.name, value: String(c.id) })),
    ...(element.cameraId
      ? cameras
          .filter((c) => String(c.id) === element.cameraId)
          .map((c) => ({ label: c.name, value: String(c.id) }))
      : []),
  ]

  return (
    <VStack gap={3}>
      <ToneBadge tone="off">{KIND_LABEL[element.kind]}</ToneBadge>
      <TextInput
        label="名称"
        value={element.label}
        onChange={(v) => onChange({ label: v })}
      />
      <Selector
        label="所属图层"
        value={element.layer}
        onChange={(v) => onChange({ layer: (v as MapLayer) || element.layer })}
        options={[
          { label: '园区总览', value: 'campus' },
          { label: '一号楼内部', value: 'building' },
          { label: '停车场', value: 'parking' },
        ]}
      />
      {element.kind === 'camera' ? (
        <Selector
          label="绑定摄像头"
          value={element.cameraId ?? ''}
          onChange={(id) => {
            const cam = cameras.find((c) => String(c.id) === id)
            onChange({
              cameraId: id || undefined,
              label: cam?.name ?? element.label,
            })
          }}
          options={cameraOptions}
          placeholder={cameraOptions.length ? '选择摄像头' : '无可用摄像头'}
        />
      ) : null}
      <HStack gap={2}>
        <NumberInput
          label="X %"
          value={Math.round(element.x * 10) / 10}
          min={0}
          max={100}
          step={0.5}
          onChange={(v) => onChange({ x: v ?? 0 })}
        />
        <NumberInput
          label="Y %"
          value={Math.round(element.y * 10) / 10}
          min={0}
          max={100}
          step={0.5}
          onChange={(v) => onChange({ y: v ?? 0 })}
        />
      </HStack>
      {element.kind !== 'camera' ? (
        <HStack gap={2}>
          <NumberInput
            label="宽 %"
            value={element.width ?? 10}
            min={2}
            max={100}
            step={0.5}
            onChange={(v) => onChange({ width: v ?? 10 })}
          />
          <NumberInput
            label="高 %"
            value={element.height ?? 10}
            min={2}
            max={100}
            step={0.5}
            onChange={(v) => onChange({ height: v ?? 10 })}
          />
        </HStack>
      ) : null}
      <Text type="supporting" color="secondary">
        可在画布上拖拽移动；切换图层后元素会出现在对应视图。
      </Text>
    </VStack>
  )
}

function CameraDetail({ camera, layer }: { camera: Camera; layer: MapLayer }) {
  return (
    <VStack gap={3}>
      <CameraStatusBadge status={camera.status} />
      <VStack gap={1}>
        <Text type="supporting" color="secondary">
          {camera.ip}:{camera.port} · {camera.vendor}
        </Text>
        <Text type="supporting" color="secondary">
          分组 {camera.group} · {camera.resolution}@{camera.fps}
        </Text>
        <Text type="supporting" color="secondary">
          当前图层 {LAYER_LABEL[layer]}
        </Text>
      </VStack>
      <HStack gap={2}>
        <Link
          to="/preview"
          onClick={() => seedPreviewSource({ cameraId: String(camera.id) })}
        >
          <Button label="预览" size="sm" variant="primary" />
        </Link>
        <Link to="/devices/$id/ai" params={{ id: String(camera.id) }}>
          <Button label="视频 AI" size="sm" variant="secondary" />
        </Link>
      </HStack>
    </VStack>
  )
}
