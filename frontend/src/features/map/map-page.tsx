import { useEffect, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Button } from '@astryxdesign/core/Button'
import { Badge } from '@astryxdesign/core/Badge'
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
import { PageLayout } from '@/components/page-layout'
import { SidePanel } from '@/components/side-panel'
import { CameraStatusBadge } from '@/components/status-badge'
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

  return (
    <PageLayout
      title="地图空间"
      contentPadding={3}
      actions={
        <HStack gap={2} vAlign="center" wrap="wrap">
          <Badge
            label={`在线 ${stats.online}/${stats.total}`}
            variant="success"
          />
          <Badge
            label={`楼栋 ${stats.buildings} · 道路 ${stats.roads}`}
            variant="blue"
          />
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
          <Button
            label="缩小"
            size="sm"
            variant="secondary"
            onClick={() => setZoom((z) => Math.max(0.7, z - 0.15))}
          />
          <Button
            label="放大"
            size="sm"
            variant="secondary"
            onClick={() => setZoom((z) => Math.min(2.2, z + 0.15))}
          />
          <Button label="重置视图" size="sm" variant="ghost" onClick={resetView} />
          <Button
            label="刷新"
            variant={editMode ? 'secondary' : 'primary'}
            size="sm"
            isLoading={mapQuery.isFetching || camerasQuery.isFetching}
            clickAction={async () => {
              if (dirty) {
                toast({ body: '请先保存或丢弃编辑中的修改', type: 'error' })
                return
              }
              await Promise.all([mapQuery.refetch(), camerasQuery.refetch()])
            }}
          />
        </HStack>
      }
      toolbar={
        <HStack gap={3} vAlign="center" hAlign="between" wrap="wrap">
          <SegmentedControl
            label="地图层级"
            value={layer}
            onChange={(v) => changeLayer((v as MapLayer) || 'campus')}
          >
            <SegmentedControlItem value="campus" label="园区总览" />
            <SegmentedControlItem value="building" label="一号楼" />
            <SegmentedControlItem value="parking" label="停车场" />
          </SegmentedControl>
          <div className="ff-map-legend">
            <span className="ff-map-legend-item" data-status="online">
              在线 {stats.online}/{stats.total}
            </span>
            <span className="ff-map-legend-item" data-status="offline">
              离线
            </span>
            <span className="ff-map-legend-item" data-status="alarm">
              报警 {stats.alarming}
            </span>
            <span className="ff-map-legend-item" data-status="error">
              异常
            </span>
            {editMode ? (
              <Text type="supporting" color="secondary">
                拖拽元素移动 · 点选后在侧栏编辑属性
                {dirty ? ' · 未保存' : ''}
              </Text>
            ) : null}
          </div>
        </HStack>
      }
    >
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
          style={{
            transform: `translate(${pan.x}px, ${pan.y}px) scale(${zoom})`,
          }}
        >
          <div className="ff-map-layer" aria-hidden />
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
        <div className="ff-map-zoom-badge">
          {LAYER_LABEL[layer]} · {Math.round(zoom * 100)}%
          {editMode ? ' · 编辑中' : ''}
        </div>
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
      <Badge label={KIND_LABEL[element.kind]} variant="neutral" />
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
