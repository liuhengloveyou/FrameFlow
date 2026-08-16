import { useEffect, useMemo, useRef, useState } from 'react'
import {
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { Badge } from '@astryxdesign/core/Badge'
import { Button } from '@astryxdesign/core/Button'
import { TextInput } from '@astryxdesign/core/TextInput'
import { Selector } from '@astryxdesign/core/Selector'
import { RadioList, RadioListItem } from '@astryxdesign/core/RadioList'
import { MoreMenu } from '@astryxdesign/core/MoreMenu'
import { HStack } from '@astryxdesign/core/HStack'
import { VStack } from '@astryxdesign/core/VStack'
import { Text } from '@astryxdesign/core/Text'
import { Icon } from '@astryxdesign/core/Icon'
import { useToast } from '@astryxdesign/core/Toast'
import { VideoCameraIcon, ChevronRightIcon, ChevronDownIcon } from '@heroicons/react/24/outline'
import { PageLayout } from '@/components/page-layout'
import { SidePanel } from '@/components/side-panel'
import { PaginationBar } from '@/components/pagination-bar'
import { DataTable, proportional, pixel, type DataTableColumn } from '@/components/data-table'
import { CameraStatusBadge } from '@/components/status-badge'
import { seedPreviewSource } from '@/lib/preview-seed'
import { channelBizId } from '@/lib/preview-source'
import { MonitorDetail } from '@/features/preview/monitor-detail'
import {
  createCamera,
  deleteCamera,
  listCameras,
  startCamera,
  stopCamera,
  testCameraStream,
  updateCamera,
} from '@/apis/devices-api'
import {
  discoverOnvifDevices,
  getOnvifSettings,
  listOnvifProfiles,
  parseOnvifUrl,
  type OnvifSource,
} from '@/apis/onvif-api'
import {
  confirmGb28181Device,
  listGb28181Devices,
  rejectGb28181Device,
  syncGb28181Catalog,
} from '@/apis/gb28181-api'
import {
  buildStreamUrl,
  CAMERA_PROTOCOL_LABEL,
  CAMERA_PROTOCOL_OPTIONS,
  GB28181_TRANSPORT_OPTIONS,
  inferProtocol,
  normalizeGb28181Transport,
  protocolFieldDefaults,
  RTMP_INGEST_MODE_OPTIONS,
  RTSP_INGEST_MODE_OPTIONS,
  STREAM_TRANSPORT_OPTIONS,
  validateCameraInput,
  withAdvertiseHost,
} from '@/lib/camera-protocol'
import { getRtmpSettings } from '@/apis/rtmp-api'
import { getRtspSettings } from '@/apis/rtsp-api'
import { getErrorMessage } from '@/lib/http'
import type {
  Camera,
  CameraChannel,
  CameraIngestMode,
  CameraInput,
  CameraProtocol,
  Gb28181Transport,
  StreamTransport,
} from '@/types/api'

const PAGE_SIZE_OPTIONS = [10, 20, 50]

type DeviceTableRow = {
  id: string
  kind: 'device' | 'channel'
  camera: Camera
  channel?: CameraChannel
}

const emptyForm: CameraInput = {
  name: '',
  ip: '',
  port: 554,
  vendor: '',
  group: '',
  resolution: '1920x1080',
  fps: 25,
  protocol: 'rtsp',
  path: 'stream1',
  channel: '',
  deviceId: '',
  username: '',
  password: '',
  streamUrl: '',
  ingestMode: 'pull',
  streamKey: '',
  allowAnonymous: true,
  advertiseHost: '',
  transport: 'tcp',
  resetPushSecret: false,
}

/** 最后心跳：本地墙钟，精确到秒 */
function formatKeepaliveSecond(iso?: string): string {
  if (!iso?.trim()) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}

export function DevicesPage() {
  const toast = useToast()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [name, setName] = useState('')
  const [status, setStatus] = useState('')
  const [group, setGroup] = useState('')
  const [protocolFilter, setProtocolFilter] = useState('')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [editing, setEditing] = useState<Camera | null>(null)
  const [form, setForm] = useState<CameraInput>(emptyForm)
  const [createdPushInfo, setCreatedPushInfo] = useState<{
    publishUrl?: string
  } | null>(null)
  const [expandedIds, setExpandedIds] = useState<Set<string>>(() => new Set())
  /** 用户主动收起过的设备，避免列表刷新后再次强制展开 */
  const collapsedByUserRef = useRef(new Set<string>())
  const [monitorDetail, setMonitorDetail] = useState<{
    camera: Camera
    channelId?: string
  } | null>(null)
  const [onvifDevices, setOnvifDevices] = useState<OnvifSource[]>([])
  const [onvifProfiles, setOnvifProfiles] = useState<OnvifSource[]>([])
  const [onvifScanning, setOnvifScanning] = useState(false)
  const [onvifLoadingProfiles, setOnvifLoadingProfiles] = useState(false)

  function streamCount(camera: Camera): number {
    const n = camera.channels?.length ?? 0
    return n > 0 ? n : 1
  }

  function openCameraPreview(camera: Camera) {
    if (streamCount(camera) <= 1) {
      const ch = camera.channels?.[0]
      const channelId = ch ? channelBizId(ch) : camera.channel || undefined
      setMonitorDetail({ camera, channelId })
      return
    }
    seedPreviewSource({ cameraId: String(camera.id), cameraOnly: true })
    void navigate({ to: '/preview' })
  }

  useEffect(() => {
    setPage(1)
  }, [name, status, group, protocolFilter, pageSize])

  const query = useQuery({
    queryKey: ['cameras', { name, status, group, protocol: protocolFilter, page, pageSize }],
    queryFn: () =>
      listCameras({
        name,
        status: status || undefined,
        group: group || undefined,
        protocol: protocolFilter || undefined,
        page,
        pageSize,
      }),
    placeholderData: (prev) => prev,
  })

  const allForGroups = useQuery({
    queryKey: ['cameras', { groups: true }],
    queryFn: () => listCameras({ pageSize: 100 }),
  })

  const gbQuery = useQuery({
    queryKey: ['gb28181-devices'],
    queryFn: listGb28181Devices,
    refetchInterval: 10_000,
  })
  const pendingGb = useMemo(
    () => (gbQuery.data ?? []).filter((d) => d.status === 'pending'),
    [gbQuery.data],
  )

  const confirmGbMut = useMutation({
    mutationFn: confirmGb28181Device,
    onSuccess: (d) => {
      toast({ body: `已确认国标设备 ${d.id}`, type: 'info' })
      void queryClient.invalidateQueries({ queryKey: ['gb28181-devices'] })
      void queryClient.invalidateQueries({ queryKey: ['cameras'] })
    },
    onError: (error) => toast({ body: getErrorMessage(error), type: 'error' }),
  })
  const rejectGbMut = useMutation({
    mutationFn: rejectGb28181Device,
    onSuccess: (d) => {
      toast({ body: `已拒绝国标设备 ${d.id}`, type: 'info' })
      void queryClient.invalidateQueries({ queryKey: ['gb28181-devices'] })
      void queryClient.invalidateQueries({ queryKey: ['cameras'] })
    },
    onError: (error) => toast({ body: getErrorMessage(error), type: 'error' }),
  })
  const syncCatalogMut = useMutation({
    mutationFn: syncGb28181Catalog,
    onSuccess: () => {
      toast({ body: '已发起目录同步', type: 'info' })
      void queryClient.invalidateQueries({ queryKey: ['cameras'] })
      void queryClient.invalidateQueries({ queryKey: ['gb28181-devices'] })
    },
    onError: (error) => toast({ body: getErrorMessage(error), type: 'error' }),
  })

  const groups = useMemo(() => {
    const set = new Set((allForGroups.data?.items ?? []).map((c) => c.group).filter(Boolean))
    return Array.from(set)
  }, [allForGroups.data?.items])

  const items = query.data?.items ?? []
  const total = query.data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / pageSize))

  const tableRows = useMemo(() => {
    const rows: DeviceTableRow[] = []
    for (const camera of items) {
      const camKey = String(camera.id)
      rows.push({ id: camKey, kind: 'device', camera })
      if (!expandedIds.has(camKey)) continue
      const channels = camera.channels ?? []
      if (channels.length === 0) {
        rows.push({ id: `${camKey}:__empty__`, kind: 'channel', camera })
        continue
      }
      for (const channel of channels) {
        rows.push({
          id: `${camKey}:${channel.id}`,
          kind: 'channel',
          camera,
          channel,
        })
      }
    }
    return rows
  }, [items, expandedIds])

  function canExpand(camera: Camera) {
    return camera.protocol === 'gb28181' || (camera.channelCount ?? 0) > 1
  }

  function toggleExpand(deviceId: string) {
    setExpandedIds((prev) => {
      const next = new Set(prev)
      if (next.has(deviceId)) {
        next.delete(deviceId)
        collapsedByUserRef.current.add(deviceId)
      } else {
        next.add(deviceId)
        collapsedByUserRef.current.delete(deviceId)
      }
      return next
    })
  }

  // 默认可展开设备为展开状态（用户收起过的除外）
  useEffect(() => {
    setExpandedIds((prev) => {
      let changed = false
      const next = new Set(prev)
      for (const camera of items) {
        if (!canExpand(camera)) continue
        const camKey = String(camera.id)
        if (collapsedByUserRef.current.has(camKey)) continue
        if (!next.has(camKey)) {
          next.add(camKey)
          changed = true
        }
      }
      return changed ? next : prev
    })
  }, [items])

  useEffect(() => {
    if (page > totalPages) setPage(totalPages)
  }, [page, totalPages])

  const isRtmpPush = form.protocol === 'rtmp' && (form.ingestMode ?? 'push') === 'push'
  const isRtmpPull = form.protocol === 'rtmp' && form.ingestMode === 'pull'
  const isRtspPush = form.protocol === 'rtsp' && form.ingestMode === 'push'
  const isRtspPull = form.protocol === 'rtsp' && form.ingestMode !== 'push'
  const isGb28181 = form.protocol === 'gb28181'
  const showStreamTransport = isRtspPull || isRtmpPull || isGb28181
  const isPushIngest = isRtmpPush || isRtspPush

  const rtmpSettingsQuery = useQuery({
    queryKey: ['rtmp-settings'],
    queryFn: getRtmpSettings,
    enabled: isRtmpPush || form.protocol === 'rtmp',
  })
  const rtmpCfg = rtmpSettingsQuery.data

  const rtspSettingsQuery = useQuery({
    queryKey: ['rtsp-settings'],
    queryFn: getRtspSettings,
    enabled: isRtspPush || form.protocol === 'rtsp',
  })
  const rtspCfg = rtspSettingsQuery.data

  const onvifSettingsQuery = useQuery({
    queryKey: ['onvif-settings'],
    queryFn: getOnvifSettings,
    enabled: form.protocol === 'onvif',
  })
  const onvifCfg = onvifSettingsQuery.data

  const previewUrl = useMemo(() => {
    if (form.protocol === 'hls') return (form.streamUrl ?? '').trim()
    if (isRtmpPush) {
      const host =
        rtmpCfg?.advertiseHost?.trim() ||
        rtmpCfg?.candidateHosts?.[0] ||
        '127.0.0.1'
      const port = rtmpCfg?.listenPort || 1935
      const key =
        (editing?.streamKey || '').trim() || '<保存后自动生成>'
      return `rtmp://${host}:${port}/${key}`
    }
    if (isRtspPush) {
      const host =
        rtspCfg?.advertiseHost?.trim() ||
        rtspCfg?.candidateHosts?.[0] ||
        '127.0.0.1'
      const port = rtspCfg?.listenPort || 8554
      const key =
        (editing?.streamKey || '').trim() || '<保存后自动生成>'
      return `rtsp://${host}:${port}/${key}`
    }
    return buildStreamUrl(form)
  }, [form, isRtmpPush, isRtspPush, rtmpCfg, rtspCfg, editing?.streamKey])

  const displayPushUrl = useMemo(() => {
    const raw =
      createdPushInfo?.publishUrl ||
      (editing?.ingestMode === 'push' ? editing.publishUrl : '') ||
      ''
    if (!raw) return ''
    if (isRtspPush || editing?.protocol === 'rtsp') {
      const host =
        rtspCfg?.advertiseHost?.trim() || rtspCfg?.candidateHosts?.[0] || undefined
      return withAdvertiseHost(raw, host)
    }
    const host =
      rtmpCfg?.advertiseHost?.trim() || rtmpCfg?.candidateHosts?.[0] || undefined
    return withAdvertiseHost(raw, host)
  }, [createdPushInfo?.publishUrl, editing, rtmpCfg, rtspCfg, isRtspPush])

  const testMutation = useMutation({
    mutationFn: (id: number | string) => testCameraStream(id),
    onSuccess: (res) => {
      toast({
        body: res.ok
          ? `测流成功 · ${res.latencyMs}ms · ${res.message}${res.codec ? ` · ${res.codec}` : ''}`
          : `测流失败 · ${res.message}`,
        type: res.ok ? 'info' : 'error',
      })
    },
    onError: (error) => toast({ body: getErrorMessage(error), type: 'error' }),
  })

  const saveMutation = useMutation({
    mutationFn: async () => {
      const payload: CameraInput = {
        ...form,
        // 推流主机/端口由系统协议配置决定，此处不传
        advertiseHost: isPushIngest ? '' : form.advertiseHost,
        ip: isPushIngest ? '' : form.ip,
        port: isPushIngest ? 0 : form.port,
        streamUrl: form.protocol === 'hls' ? form.streamUrl : buildStreamUrl(form),
      }
      const check = validateCameraInput(payload)
      if (!check.ok) throw new Error(check.message)
      if (editing) return updateCamera(editing.id, payload)
      return createCamera(payload)
    },
    onSuccess: async (cam) => {
      if (isPushIngest && cam.publishUrl) {
        setCreatedPushInfo({ publishUrl: cam.publishUrl })
        toast({ body: '已保存；请复制推流地址', type: 'info' })
      } else {
        setCreatedPushInfo(null)
        toast({ body: editing ? '已更新摄像头' : '已添加设备', type: 'info' })
        setDrawerOpen(false)
        setEditing(null)
      }
      await queryClient.invalidateQueries({ queryKey: ['cameras'] })
    },
    onError: (error) =>
      toast({ body: getErrorMessage(error, '保存失败'), type: 'error' }),
  })

  const actionMutation = useMutation({
    mutationFn: async ({
      id,
      action,
    }: {
      id: number | string
      action: 'start' | 'stop' | 'delete'
    }) => {
      if (action === 'start') return startCamera(id)
      if (action === 'stop') return stopCamera(id)
      return deleteCamera(id)
    },
    onSuccess: async (_, vars) => {
      toast({
        body: vars.action === 'delete' ? '已删除' : vars.action === 'start' ? '已启动' : '已停止',
        type: 'info',
      })
      await queryClient.invalidateQueries({ queryKey: ['cameras'] })
    },
    onError: (error) => toast({ body: getErrorMessage(error), type: 'error' }),
  })

  function openCreate() {
    setEditing(null)
    setForm(emptyForm)
    setCreatedPushInfo(null)
    setOnvifDevices([])
    setOnvifProfiles([])
    setDrawerOpen(true)
  }

  function openEdit(camera: Camera) {
    setEditing(camera)
    setCreatedPushInfo(null)
    const protocol =
      camera.protocol ?? inferProtocol(camera.streamUrl) ?? ('rtsp' as CameraProtocol)
    const deviceId = camera.deviceId ?? (protocol === 'gb28181' ? camera.ip : '')
    const ingestMode = (
      camera.protocol === 'rtmp'
        ? (camera.ingestMode ?? 'push')
        : camera.protocol === 'rtsp'
          ? (camera.ingestMode ?? 'pull')
          : (camera.ingestMode ?? 'pull')
    ) as CameraIngestMode
    setForm({
      name: camera.name,
      ip: camera.ip,
      port: camera.port,
      vendor: camera.vendor,
      group: camera.group,
      resolution: camera.resolution,
      fps: camera.fps,
      protocol,
      streamUrl: camera.streamUrl ?? '',
      username: camera.username ?? '',
      password: camera.password ?? '',
      path: camera.path ?? '',
      channel: camera.channel ?? '',
      deviceId,
      ingestMode,
      streamKey: camera.streamKey ?? '',
      allowAnonymous: true,
      advertiseHost: '',
      transport:
        protocol === 'gb28181'
          ? normalizeGb28181Transport(camera.transport)
          : protocol === 'rtsp' || protocol === 'rtmp'
            ? ((camera.transport === 'udp' ? 'udp' : 'tcp') as StreamTransport)
            : '',
      resetPushSecret: false,
    })
    setDrawerOpen(true)
  }

  async function copyText(text: string, label: string) {
    try {
      await navigator.clipboard.writeText(text)
      toast({ body: `已复制${label}`, type: 'info' })
    } catch {
      toast({ body: `复制失败：${text}`, type: 'error' })
    }
  }

  function changeProtocol(next: CameraProtocol) {
    setOnvifDevices([])
    setOnvifProfiles([])
    setForm((f) => {
      const defaults = protocolFieldDefaults(next)
      const keepIp =
        next !== 'hls' &&
        next !== 'gb28181' &&
        !(next === 'rtmp' && (defaults.ingestMode ?? 'push') === 'push') &&
        !(next === 'rtsp' && defaults.ingestMode === 'push')
      return {
        ...f,
        ...defaults,
        name: f.name,
        vendor: f.vendor,
        group: f.group,
        resolution: f.resolution,
        fps: f.fps,
        ip: keepIp ? f.ip || '' : (defaults.ip ?? ''),
        deviceId: next === 'gb28181' ? f.deviceId || f.ip || '' : '',
      }
    })
  }

  async function scanOnvifDevices() {
    setOnvifScanning(true)
    setOnvifDevices([])
    try {
      const list = await discoverOnvifDevices()
      setOnvifDevices(list)
      toast({
        body: list.length ? `发现 ${list.length} 台 ONVIF 设备` : '未发现 ONVIF 设备（需与摄像头同网段）',
        type: 'info',
      })
    } catch (error) {
      toast({ body: getErrorMessage(error), type: 'error' })
    } finally {
      setOnvifScanning(false)
    }
  }

  function applyOnvifDevice(src: OnvifSource) {
    const raw = (src.url ?? '').trim()
    const parsed = parseOnvifUrl(raw)
    if (!parsed) {
      toast({ body: '无法解析设备地址', type: 'error' })
      return
    }
    setOnvifProfiles([])
    const defUser =
      parsed.username ||
      (src.location ?? '').trim() ||
      onvifCfg?.defaultUsername?.trim() ||
      ''
    setForm((f) => ({
      ...f,
      protocol: 'onvif',
      ip: parsed.host,
      port: parsed.port || 80,
      name: f.name.trim() || (src.name || src.info || parsed.host).trim(),
      vendor: f.vendor || (src.info ?? '').trim(),
      channel: f.channel || '1',
      username: defUser || f.username,
      password: parsed.password || f.password,
    }))
  }

  async function loadOnvifProfiles() {
    if (!form.ip.trim()) {
      toast({ body: '请先填写设备 IP', type: 'error' })
      return
    }
    if (!(form.username ?? '').trim()) {
      toast({ body: '请先填写 ONVIF 用户名', type: 'error' })
      return
    }
    setOnvifLoadingProfiles(true)
    setOnvifProfiles([])
    try {
      const src = buildStreamUrl({
        ...form,
        protocol: 'onvif',
        channel: '1',
      })
      // buildStreamUrl 带 subtype；列 Profile 时去掉 query，只传鉴权主机
      const base = src.replace(/\?.*$/, '')
      const list = await listOnvifProfiles(base)
      const media = list.filter((s) => !(s.url ?? '').includes('snapshot'))
      setOnvifProfiles(media)
      toast({
        body: media.length ? `共 ${media.length} 路 Profile` : '未获取到 Profile（请检查账号）',
        type: 'info',
      })
    } catch (error) {
      toast({ body: getErrorMessage(error), type: 'error' })
    } finally {
      setOnvifLoadingProfiles(false)
    }
  }

  function applyOnvifProfile(src: OnvifSource, index: number) {
    setForm((f) => ({
      ...f,
      channel: String(index + 1),
      name: f.name.trim() || (src.name ?? f.name),
    }))
    toast({ body: `已选择 Profile ${index + 1}`, type: 'info' })
  }

  return (
    <PageLayout
      title="设备管理"
      description="多协议接入、筛选、启停与 AI 配置入口"
      actions={
        <HStack gap={2} vAlign="center">
          <Text type="supporting" color="secondary">
            共 {total} 台
          </Text>
          <Button
            label="刷新"
            variant="secondary"
            size="sm"
            isLoading={query.isFetching}
            clickAction={async () => {
              await query.refetch()
            }}
          />
          <Button label="添加设备" variant="primary" size="sm" onClick={openCreate} />
        </HStack>
      }
      toolbar={
        <div className="ff-toolbar">
          <TextInput
            label="名称"
            isLabelHidden
            placeholder="按名称搜索"
            value={name}
            onChange={setName}
            hasClear
          />
          <Selector
            label="协议"
            isLabelHidden
            placeholder="全部协议"
            hasClear
            value={protocolFilter}
            onChange={(v) => setProtocolFilter(v ?? '')}
            options={CAMERA_PROTOCOL_OPTIONS}
          />
          <Selector
            label="状态"
            isLabelHidden
            placeholder="全部状态"
            hasClear
            value={status}
            onChange={(v) => setStatus(v ?? '')}
            options={[
              { label: '在线', value: 'online' },
              { label: '离线', value: 'offline' },
              { label: '运行中', value: 'running' },
              { label: '已停止', value: 'stopped' },
              { label: '异常', value: 'error' },
            ]}
          />
          <Selector
            label="分组"
            isLabelHidden
            placeholder="全部分组"
            hasClear
            value={group}
            onChange={(v) => setGroup(v ?? '')}
            options={groups.map((g) => ({ label: g, value: g }))}
          />
        </div>
      }
    >
      {pendingGb.length > 0 ? (
        <VStack gap={3} className="ff-gb-pending">
          <HStack gap={2} vAlign="center">
            <Badge label={`待确认 ${pendingGb.length}`} variant="error" />
            <Text type="supporting" color="secondary">
              国标设备主动注册，确认后才允许拉流；拒绝后将禁止再次注册。
            </Text>
          </HStack>
          <div className="ff-table-wrap">
            <DataTable
              data={pendingGb}
              idKey="id"
              density="compact"
              isStriped
              emptyLabel="暂无待确认设备"
              columns={[
                {
                  key: 'name',
                  header: '名称',
                  width: proportional(2, { minWidth: 140 }),
                  render: (d) => (
                    <Text type="body" weight="semibold" maxLines={1}>
                      {d.name || d.remote_addr || d.id}
                    </Text>
                  ),
                },
                {
                  key: 'id',
                  header: '国标 ID',
                  width: proportional(2, { minWidth: 180 }),
                  render: (d) => (
                    <Text type="supporting" maxLines={1}>
                      {d.id}
                    </Text>
                  ),
                },
                {
                  key: 'remote_addr',
                  header: '对端地址',
                  width: pixel(160),
                  render: (d) => (
                    <Text type="supporting">{d.remote_addr || '—'}</Text>
                  ),
                },
                {
                  key: 'online',
                  header: '在线',
                  width: pixel(88),
                  render: (d) => (
                    <Badge
                      label={d.online ? '在线' : '离线'}
                      variant={d.online ? 'success' : 'neutral'}
                    />
                  ),
                },
                {
                  key: 'channels',
                  header: '通道',
                  width: pixel(72),
                  align: 'end',
                  render: (d) => (
                    <Text type="supporting">{d.channels || 0}</Text>
                  ),
                },
                {
                  key: 'actions',
                  header: '操作',
                  width: pixel(160),
                  render: (d) => (
                    <HStack gap={2}>
                      <Button
                        label="确认"
                        variant="primary"
                        size="sm"
                        isLoading={confirmGbMut.isPending}
                        onClick={() => confirmGbMut.mutate(d.id)}
                      />
                      <Button
                        label="拒绝"
                        variant="secondary"
                        size="sm"
                        isLoading={rejectGbMut.isPending}
                        onClick={() => rejectGbMut.mutate(d.id)}
                      />
                    </HStack>
                  ),
                },
              ]}
            />
          </div>
        </VStack>
      ) : null}
      <div className="ff-table-wrap">
        <DataTable
          data={tableRows}
          idKey="id"
          density="balanced"
          isStriped
          stickyEdges
          emptyLabel={query.isLoading ? '加载中…' : '暂无摄像头，点击右上角新增'}
          className="ff-device-channel-table"
          columns={
            [
              {
                key: 'name',
                header: '名称',
                width: proportional(2.2, { minWidth: 200 }),
                render: (row) => {
                  if (row.kind === 'channel') {
                    if (!row.channel) {
                      return (
                        <Text
                          type="supporting"
                          color="secondary"
                          className="ff-channel-indent"
                        >
                          暂无通道，请「同步目录」
                        </Text>
                      )
                    }
                    return (
                      <VStack gap={0} className="ff-channel-indent">
                        <Text type="body" maxLines={1}>
                          {row.channel.name?.trim() || `通道 ${channelBizId(row.channel)}`}
                          {row.channel.isDefault ? ' · 默认' : ''}
                        </Text>
                        <Text type="supporting" color="secondary" maxLines={1}>
                          {row.channel.streamUrl || channelBizId(row.channel)}
                        </Text>
                      </VStack>
                    )
                  }
                  const c = row.camera
                  const open = expandedIds.has(String(c.id))
                  const expandable = canExpand(c)
                  return (
                    <HStack gap={2} vAlign="center">
                      {expandable ? (
                        <button
                          type="button"
                          className="ff-expand-btn"
                          aria-label={open ? '收起通道' : '展开通道'}
                          onClick={(e) => {
                            e.stopPropagation()
                            toggleExpand(String(c.id))
                          }}
                        >
                          <Icon
                            icon={open ? ChevronDownIcon : ChevronRightIcon}
                            size="sm"
                            color="secondary"
                          />
                        </button>
                      ) : (
                        <span className="ff-expand-spacer" />
                      )}
                      <Icon icon={VideoCameraIcon} size="sm" color="secondary" />
                      <VStack gap={0}>
                        <Text type="body" weight="semibold" maxLines={1}>
                          {c.name?.trim() ||
                            c.remoteAddr ||
                            c.deviceId ||
                            c.ip ||
                            c.id}
                        </Text>
                        <Text type="supporting" color="secondary" maxLines={1}>
                          {c.protocol === 'gb28181'
                            ? `${c.channelCount ?? 0} 路通道`
                            : c.ingestMode === 'push' &&
                                (c.protocol === 'rtsp' || c.protocol === 'rtmp')
                              ? withAdvertiseHost(c.publishUrl || '') ||
                                c.publishUrl ||
                                c.streamKey ||
                                c.id
                              : c.streamUrl?.trim() || c.id}
                        </Text>
                      </VStack>
                    </HStack>
                  )
                },
              },
              {
                key: 'status',
                header: '状态',
                width: pixel(108),
                render: (row) => {
                  if (row.kind === 'channel') {
                    const st = row.channel?.status?.toUpperCase()
                    if (!row.channel) return null
                    return (
                      <Badge
                        label={st === 'ON' || st === 'ONLINE' ? '在线' : st || '—'}
                        variant={
                          st === 'ON' || st === 'ONLINE' ? 'success' : 'neutral'
                        }
                      />
                    )
                  }
                  return <CameraStatusBadge status={row.camera.status} />
                },
              },
              {
                key: 'lastKeepalive',
                header: '最后心跳时间',
                width: pixel(168),
                render: (row) => {
                  if (row.kind === 'channel') return null
                  return (
                    <Text type="supporting" color="secondary" maxLines={1}>
                      {formatKeepaliveSecond(row.camera.lastKeepalive)}
                    </Text>
                  )
                },
              },
              {
                key: 'protocol',
                header: '协议',
                width: pixel(100),
                render: (row) => {
                  if (row.kind === 'channel') {
                    return (
                      <Text type="supporting" color="secondary">
                        通道
                      </Text>
                    )
                  }
                  const c = row.camera
                  return (
                    <Badge
                      label={CAMERA_PROTOCOL_LABEL[c.protocol] ?? c.protocol}
                      variant={c.protocol === 'gb28181' ? 'teal' : 'neutral'}
                    />
                  )
                },
              },
              {
                key: 'endpoint',
                header: '编号 / 地址',
                width: proportional(1.6, { minWidth: 180 }),
                render: (row) => {
                  if (row.kind === 'channel') {
                    return (
                      <Text type="supporting" maxLines={1}>
                        {row.channel ? channelBizId(row.channel) : '—'}
                      </Text>
                    )
                  }
                  const c = row.camera
                  if (c.protocol === 'hls') {
                    return (
                      <Text type="supporting" color="secondary" maxLines={1}>
                        {c.streamUrl || '—'}
                      </Text>
                    )
                  }
                  if (c.protocol === 'gb28181') {
                    return (
                      <Text type="body" maxLines={1}>
                        {c.deviceId || c.id}
                      </Text>
                    )
                  }
                  if (
                    c.ingestMode === 'push' &&
                    (c.protocol === 'rtsp' || c.protocol === 'rtmp')
                  ) {
                    const url =
                      withAdvertiseHost(c.publishUrl || '') ||
                      c.publishUrl ||
                      (c.streamKey
                        ? `${c.protocol}://${c.ip || '127.0.0.1'}:${c.port}/${c.streamKey}`
                        : '')
                    return (
                      <VStack gap={0}>
                        <Text type="body" maxLines={1}>
                          {url || `${c.ip || '—'}${c.port ? `:${c.port}` : ''}`}
                        </Text>
                        <Text type="supporting" color="secondary" maxLines={1}>
                          推流{c.streamKey ? ` · ${c.streamKey}` : ''}
                        </Text>
                      </VStack>
                    )
                  }
                  return (
                    <VStack gap={0}>
                      <Text type="body" maxLines={1}>
                        {c.ip || '—'}
                        {c.port ? `:${c.port}` : ''}
                      </Text>
                      <Text type="supporting" color="secondary" maxLines={1}>
                        {c.protocol === 'onvif'
                          ? `Profile ${c.channel || '1'}`
                          : c.path || '—'}
                      </Text>
                    </VStack>
                  )
                },
              },
              {
                key: 'group',
                header: '分组',
                width: pixel(112),
                render: (row) => {
                  if (row.kind === 'channel') return null
                  const g = row.camera.group?.trim()
                  return g ? (
                    <Badge label={g} variant="neutral" />
                  ) : (
                    <Text type="supporting" color="secondary">
                      —
                    </Text>
                  )
                },
              },
              {
                key: 'media',
                header: '画面',
                width: pixel(120),
                render: (row) => {
                  if (row.kind === 'channel') return null
                  const c = row.camera
                  return (
                    <Text type="supporting" color="secondary">
                      {[c.resolution, c.fps ? `${c.fps} fps` : '']
                        .filter(Boolean)
                        .join(' · ') || '—'}
                    </Text>
                  )
                },
              },
              {
                key: 'vendor',
                header: '厂商',
                width: pixel(96),
                render: (row) => {
                  if (row.kind === 'channel') {
                    return (
                      <Text type="supporting" color="secondary" maxLines={1}>
                        {row.channel?.manufacturer?.trim() || '—'}
                      </Text>
                    )
                  }
                  return (
                    <Text type="supporting" color="secondary" maxLines={1}>
                      {row.camera.vendor?.trim() || '—'}
                    </Text>
                  )
                },
              },
              {
                key: 'actions',
                header: '操作',
                width: pixel(200),
                render: (row) => {
                  if (row.kind === 'channel') {
                    if (!row.channel) {
                      return row.camera.protocol === 'gb28181' ? (
                        <Button
                          label="同步目录"
                          size="sm"
                          variant="ghost"
                          isLoading={syncCatalogMut.isPending}
                          onClick={() => {
                            const id = row.camera.deviceId || String(row.camera.id)
                            void syncCatalogMut.mutateAsync(id)
                          }}
                        />
                      ) : null
                    }
                    return (
                      <HStack gap={1} vAlign="center" wrap="nowrap">
                        <Button
                          label="预览"
                          size="sm"
                          variant="ghost"
                          onClick={() =>
                            setMonitorDetail({
                              camera: row.camera,
                              channelId: row.channel
                                ? channelBizId(row.channel)
                                : undefined,
                            })
                          }
                        />
                        <Link
                          to="/devices/$id/channels/$channelId/ai"
                          params={{
                            id: String(row.camera.id),
                            channelId: channelBizId(row.channel),
                          }}
                        >
                          <Button label="AI" size="sm" variant="ghost" />
                        </Link>
                      </HStack>
                    )
                  }
                  const camera = row.camera
                  return (
                    <HStack gap={1} vAlign="center" wrap="nowrap">
                      <Button
                        label="预览"
                        size="sm"
                        variant="ghost"
                        onClick={() => openCameraPreview(camera)}
                      />
                      <Button
                        label="编辑"
                        size="sm"
                        variant="ghost"
                        onClick={() => openEdit(camera)}
                      />
                      {!canExpand(camera) ? (
                        <Link
                          to="/devices/$id/channels/$channelId/ai"
                          params={{
                            id: String(camera.id),
                            channelId: camera.channels?.[0]
                              ? channelBizId(camera.channels[0])
                              : camera.channel || String(camera.id),
                          }}
                        >
                          <Button label="AI" size="sm" variant="ghost" />
                        </Link>
                      ) : null}
                      <MoreMenu
                        size="sm"
                        label={`${camera.name} 更多操作`}
                        items={[
                          {
                            label: '测流',
                            onClick: () => {
                              void testMutation.mutateAsync(camera.id)
                            },
                          },
                          ...(camera.protocol === 'gb28181'
                            ? [
                                {
                                  label: '同步目录',
                                  onClick: () => {
                                    const id =
                                      camera.deviceId || String(camera.id)
                                    void syncCatalogMut.mutateAsync(id)
                                  },
                                },
                              ]
                            : []),
                          {
                            label: '启动',
                            onClick: () => {
                              void actionMutation.mutateAsync({
                                id: camera.id,
                                action: 'start',
                              })
                            },
                          },
                          {
                            label: '停止',
                            onClick: () => {
                              void actionMutation.mutateAsync({
                                id: camera.id,
                                action: 'stop',
                              })
                            },
                          },
                          { type: 'divider' as const },
                          {
                            label: '删除',
                            onClick: () => {
                              void actionMutation.mutateAsync({
                                id: camera.id,
                                action: 'delete',
                              })
                            },
                          },
                        ]}
                      />
                    </HStack>
                  )
                },
              },
            ] satisfies DataTableColumn<DeviceTableRow>[]
          }
        />
      </div>

      {total > 0 ? (
        <div className="ff-pagination">
          <PaginationBar
            page={page}
            pageSize={pageSize}
            total={total}
            onPageChange={setPage}
            pageSizeOptions={PAGE_SIZE_OPTIONS}
            onPageSizeChange={setPageSize}
          />
        </div>
      ) : null}

      <SidePanel
        open={drawerOpen}
        title={editing ? '编辑摄像头' : '添加设备'}
        onClose={() => {
          setDrawerOpen(false)
          setCreatedPushInfo(null)
        }}
        footer={
          <>
            <Button label="取消" variant="secondary" onClick={() => setDrawerOpen(false)} />
            <Button
              label="保存"
              variant="primary"
              isLoading={saveMutation.isPending}
              onClick={() => {
                saveMutation.mutate()
              }}
            />
          </>
        }
      >
        <VStack gap={3}>
          <Selector
            label="接入协议"
            isRequired
            value={form.protocol}
            onChange={(v) => changeProtocol((v as CameraProtocol) || 'rtsp')}
            options={CAMERA_PROTOCOL_OPTIONS}
          />
          <TextInput
            label="名称"
            isRequired
            value={form.name}
            onChange={(v) => setForm((f) => ({ ...f, name: v }))}
          />

          {form.protocol === 'rtmp' ? (
            <RadioList
              label="接入方式"
              isRequired
              value={form.ingestMode ?? 'push'}
              onChange={(v) => {
                const mode = (v as CameraIngestMode) || 'push'
                setForm((f) =>
                  mode === 'push'
                    ? {
                        ...f,
                        ingestMode: 'push',
                        ip: '',
                        port: 0,
                        path: '',
                        username: '',
                        password: '',
                        advertiseHost: '',
                      }
                    : {
                        ...f,
                        ingestMode: 'pull',
                        path: f.path || 'live/stream',
                        port: f.port > 0 ? f.port : 1935,
                        streamKey: '',
                        transport: f.transport === 'udp' ? 'udp' : 'tcp',
                      },
                )
              }}
            >
              {RTMP_INGEST_MODE_OPTIONS.map((opt) => (
                <RadioListItem key={opt.value} value={opt.value} label={opt.label} />
              ))}
            </RadioList>
          ) : null}

          {form.protocol === 'rtsp' ? (
            <RadioList
              label="接入方式"
              isRequired
              value={form.ingestMode ?? 'pull'}
              onChange={(v) => {
                const mode = (v as CameraIngestMode) || 'pull'
                setForm((f) =>
                  mode === 'push'
                    ? {
                        ...f,
                        ingestMode: 'push',
                        ip: '',
                        port: 0,
                        path: '',
                        username: '',
                        password: '',
                        advertiseHost: '',
                      }
                    : {
                        ...f,
                        ingestMode: 'pull',
                        path: f.path || 'stream1',
                        port: f.port > 0 ? f.port : 554,
                        streamKey: '',
                        transport: f.transport === 'udp' ? 'udp' : 'tcp',
                      },
                )
              }}
            >
              {RTSP_INGEST_MODE_OPTIONS.map((opt) => (
                <RadioListItem key={opt.value} value={opt.value} label={opt.label} />
              ))}
            </RadioList>
          ) : null}

          {form.protocol === 'hls' ? (
            <TextInput
              label="HLS 流地址"
              isRequired
              placeholder="https://host/live/stream.m3u8"
              value={form.streamUrl ?? ''}
              onChange={(v) => setForm((f) => ({ ...f, streamUrl: v }))}
            />
          ) : form.protocol === 'gb28181' ? (
            <>
              <TextInput
                label="设备国标 ID"
                isRequired
                placeholder="20 位数字，如 34020000001320000001"
                value={form.deviceId ?? ''}
                onChange={(v) =>
                  setForm((f) => ({ ...f, deviceId: v, ip: v }))
                }
              />
              <Text type="supporting" color="secondary">
                保存后会预登记到 GB28181 平台；当前不校验注册密码，摄像头将设备 ID 指向本机 SIP 即可注册。
              </Text>
            </>
          ) : isRtmpPush ? (
            <>
              <div className="ff-rtmp-hint-box">
                <Text type="label">系统 RTMP 配置（只读）</Text>
                {rtmpCfg && !rtmpCfg.enabled ? (
                  <Text type="supporting" color="secondary">
                    RTMP 服务当前未开启，请先到「协议配置 → RTMP」打开服务。
                  </Text>
                ) : null}
                <VStack gap={1}>
                  <Text type="supporting">
                    主机：
                    {rtmpCfg?.advertiseHost?.trim() ||
                      rtmpCfg?.candidateHosts?.[0] ||
                      '（未配置，将使用本机地址）'}
                  </Text>
                  <Text type="supporting">
                    端口：{rtmpCfg?.listenPort || 1935}
                    {rtmpCfg?.runningListen
                      ? ` · 监听中 ${rtmpCfg.runningListen}`
                      : rtmpCfg?.enabled
                        ? ' · 尚未监听'
                        : ''}
                  </Text>
                  <Text type="supporting" style={{ wordBreak: 'break-all' }}>
                    推流地址：{displayPushUrl || previewUrl || rtmpCfg?.publishExample || '保存后生成'}
                  </Text>
                </VStack>

                <HStack gap={2} vAlign="center" wrap="wrap">
                  {displayPushUrl || previewUrl ? (
                    <Button
                      label="复制地址"
                      variant="secondary"
                      size="sm"
                      onClick={() =>
                        void copyText(displayPushUrl || previewUrl, '推流地址')
                      }
                    />
                  ) : null}
                  <Button
                    label="去配置 RTMP"
                    variant="ghost"
                    size="sm"
                    onClick={() =>
                      void navigate({
                        to: '/system/protocols',
                        search: { tab: 'rtmp' },
                      })
                    }
                  />
                </HStack>
              </div>
            </>
          ) : isRtspPush ? (
            <>
              <div className="ff-rtmp-hint-box">
                <Text type="label">系统 RTSP 配置（只读）</Text>
                {rtspCfg && !rtspCfg.enabled ? (
                  <Text type="supporting" color="secondary">
                    RTSP 服务当前未开启，请先到「协议配置 → RTSP」打开服务。
                  </Text>
                ) : null}
                <VStack gap={1}>
                  <Text type="supporting">
                    主机：
                    {rtspCfg?.advertiseHost?.trim() ||
                      rtspCfg?.candidateHosts?.[0] ||
                      '（未配置，将使用本机地址）'}
                  </Text>
                  <Text type="supporting">
                    端口：{rtspCfg?.listenPort || 8554}
                    {rtspCfg?.runningListen
                      ? ` · 监听中 ${rtspCfg.runningListen}`
                      : rtspCfg?.enabled
                        ? ' · 尚未监听'
                        : ''}
                  </Text>
                  <Text type="supporting" style={{ wordBreak: 'break-all' }}>
                    推流地址：{displayPushUrl || previewUrl || rtspCfg?.playExample || '保存后生成'}
                  </Text>
                </VStack>

                <HStack gap={2} vAlign="center" wrap="wrap">
                  {displayPushUrl || previewUrl ? (
                    <Button
                      label="复制地址"
                      variant="secondary"
                      size="sm"
                      onClick={() =>
                        void copyText(displayPushUrl || previewUrl, '推流地址')
                      }
                    />
                  ) : null}
                  <Button
                    label="去配置 RTSP"
                    variant="ghost"
                    size="sm"
                    onClick={() =>
                      void navigate({
                        to: '/system/protocols',
                        search: { tab: 'rtsp' },
                      })
                    }
                  />
                </HStack>
              </div>
            </>
          ) : (
            <>
              <TextInput
                label={form.protocol === 'rtmp' ? '主机' : 'IP'}
                isRequired
                value={form.ip}
                onChange={(v) => setForm((f) => ({ ...f, ip: v }))}
              />
              <TextInput
                label="端口"
                isRequired
                value={String(form.port)}
                onChange={(v) => setForm((f) => ({ ...f, port: Number(v) || 0 }))}
              />
            </>
          )}

          {form.protocol === 'onvif' ? (
            <div className="ff-rtmp-hint-box">
              <Text type="label">ONVIF 拉流（与 RTSP 拉流类似）</Text>
              <Text type="supporting" color="secondary">
                填写摄像头 ONVIF 地址与账号；也可扫描同网段设备。播放时 GetStreamUri
                解析为 RTSP。
              </Text>
              <HStack gap={2} vAlign="center" wrap="wrap">
                <Button
                  label={onvifScanning ? '扫描中…' : '扫描局域网'}
                  variant="secondary"
                  size="sm"
                  isDisabled={onvifScanning}
                  onClick={() => void scanOnvifDevices()}
                />
                <Button
                  label={onvifLoadingProfiles ? '拉取中…' : '拉取 Profiles'}
                  variant="secondary"
                  size="sm"
                  isDisabled={onvifLoadingProfiles}
                  onClick={() => void loadOnvifProfiles()}
                />
                <Button
                  label="去配置 ONVIF"
                  variant="ghost"
                  size="sm"
                  onClick={() =>
                    void navigate({
                      to: '/system/protocols',
                      search: { tab: 'onvif' },
                    })
                  }
                />
              </HStack>
              {onvifDevices.length > 0 ? (
                <VStack gap={1}>
                  <Text type="supporting">发现的设备（点击填入）</Text>
                  {onvifDevices.map((d) => (
                    <Button
                      key={d.url || d.name}
                      label={`${d.name || d.url || '设备'}${d.info ? ` · ${d.info}` : ''}`}
                      variant="ghost"
                      size="sm"
                      onClick={() => applyOnvifDevice(d)}
                    />
                  ))}
                </VStack>
              ) : null}
              {onvifProfiles.length > 0 ? (
                <VStack gap={1}>
                  <Text type="supporting">Media Profiles（点击选择）</Text>
                  {onvifProfiles.map((p, i) => (
                    <Button
                      key={p.url || `${p.name}-${i}`}
                      label={`${i + 1}. ${p.name || p.url || 'profile'}`}
                      variant={String(form.channel) === String(i + 1) ? 'primary' : 'ghost'}
                      size="sm"
                      onClick={() => applyOnvifProfile(p, i)}
                    />
                  ))}
                </VStack>
              ) : null}
              {onvifCfg && !onvifCfg.serverEnabled ? (
                <Text type="supporting" color="secondary">
                  本机 ONVIF 服务端当前关闭（不影响拉远端摄像头）。
                </Text>
              ) : null}
            </div>
          ) : null}

          {createdPushInfo ? (
            <VStack gap={2}>
              <Text type="label">推流信息（请立即复制）</Text>
              {createdPushInfo.publishUrl ? (
                <HStack gap={2} vAlign="center">
                  <Text type="supporting" style={{ wordBreak: 'break-all', flex: 1 }}>
                    {createdPushInfo.publishUrl}
                  </Text>
                  <Button
                    label="复制地址"
                    variant="secondary"
                    size="sm"
                    onClick={() => void copyText(createdPushInfo.publishUrl!, '推流地址')}
                  />
                </HStack>
              ) : null}
              <Button
                label="完成"
                variant="primary"
                size="sm"
                onClick={() => {
                  setCreatedPushInfo(null)
                  setDrawerOpen(false)
                  setEditing(null)
                }}
              />
            </VStack>
          ) : null}

          {showStreamTransport ? (
            <VStack gap={1}>
              {isGb28181 ? (
                <RadioList
                  label="传输模式"
                  value={normalizeGb28181Transport(form.transport)}
                  onChange={(v) =>
                    setForm((f) => ({
                      ...f,
                      transport: (v as Gb28181Transport) || 'tcp-passive',
                    }))
                  }
                >
                  {GB28181_TRANSPORT_OPTIONS.map((opt) => (
                    <RadioListItem key={opt.value} value={opt.value} label={opt.label} />
                  ))}
                </RadioList>
              ) : (
                <RadioList
                  label="传输模式"
                  value={(form.transport === 'udp' ? 'udp' : 'tcp') as StreamTransport}
                  onChange={(v) =>
                    setForm((f) => ({
                      ...f,
                      transport: ((v as StreamTransport) || 'tcp') as StreamTransport,
                    }))
                  }
                >
                  {STREAM_TRANSPORT_OPTIONS.map((opt) => (
                    <RadioListItem key={opt.value} value={opt.value} label={opt.label} />
                  ))}
                </RadioList>
              )}
              {isGb28181 ? (
                <Text type="supporting" color="secondary">
                  TCP 被动：平台监听，设备连入；TCP 主动：平台连设备；AUTO：跟随网关配置。
                </Text>
              ) : isRtmpPull ? (
                <Text type="supporting" color="secondary">
                  RTMP 协议本身仅 TCP；选 UDP 时仍按 TCP 拉流。
                </Text>
              ) : null}
            </VStack>
          ) : null}

          <details className="ff-form-more">
            <summary>更多选项（可选）</summary>
            <div className="ff-form-more-body">
              <VStack gap={3}>
              {isPushIngest ? null : form.protocol === 'gb28181' ? (
                <TextInput
                  label="通道国标 ID"
                  placeholder="可选，默认与设备 ID 相同"
                  value={form.channel ?? ''}
                  onChange={(v) => setForm((f) => ({ ...f, channel: v }))}
                />
              ) : form.protocol === 'hls' ? null : (
                <>
                  {form.protocol === 'onvif' ? (
                    <TextInput
                      label="Profile 序号"
                      placeholder="1=主码流，2=子码流…"
                      value={form.channel ?? '1'}
                      onChange={(v) => setForm((f) => ({ ...f, channel: v }))}
                    />
                  ) : (
                    <TextInput
                      label={form.protocol === 'rtmp' ? '应用路径' : '流路径'}
                      placeholder={form.protocol === 'rtmp' ? 'live/stream' : 'stream1'}
                      value={form.path ?? ''}
                      onChange={(v) => setForm((f) => ({ ...f, path: v }))}
                    />
                  )}
                  <TextInput
                    label="用户名"
                    value={form.username ?? ''}
                    onChange={(v) => setForm((f) => ({ ...f, username: v }))}
                  />
                  <TextInput
                    label="密码"
                    type="password"
                    value={form.password ?? ''}
                    onChange={(v) => setForm((f) => ({ ...f, password: v }))}
                  />
                </>
              )}

              <TextInput
                label="厂商"
                value={form.vendor}
                onChange={(v) => setForm((f) => ({ ...f, vendor: v }))}
              />
              <TextInput
                label="分组"
                value={form.group}
                onChange={(v) => setForm((f) => ({ ...f, group: v }))}
              />
              <TextInput
                label="分辨率"
                value={form.resolution}
                onChange={(v) => setForm((f) => ({ ...f, resolution: v }))}
              />
              <TextInput
                label="FPS"
                value={String(form.fps)}
                onChange={(v) => setForm((f) => ({ ...f, fps: Number(v) || 0 }))}
              />

              {!isPushIngest ? (
                <VStack gap={1}>
                  <Text type="label">流地址预览</Text>
                  <Text type="supporting" color="secondary" style={{ wordBreak: 'break-all' }}>
                    {previewUrl || '填写上方字段后自动生成'}
                  </Text>
                  <Text type="supporting" color="secondary">
                    保存后用于测流 / 预览
                  </Text>
                </VStack>
              ) : null}
              </VStack>
            </div>
          </details>
        </VStack>
      </SidePanel>

      {monitorDetail ? (
        <MonitorDetail
          camera={monitorDetail.camera}
          channelId={monitorDetail.channelId}
          onClose={() => setMonitorDetail(null)}
        />
      ) : null}
    </PageLayout>
  )
}
