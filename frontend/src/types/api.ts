export type PageResult<T> = {
  items: T[]
  total: number
  page: number
  pageSize: number
}

export type CameraStatus = 'online' | 'offline' | 'error' | 'running' | 'stopped'

/** 摄像头接入协议 */
export type CameraProtocol = 'rtsp' | 'rtmp' | 'onvif' | 'hls' | 'gb28181'

/** 接入方向：平台拉远端 / 设备推到平台 */
export type CameraIngestMode = 'pull' | 'push'

/** 流传输模式（RTSP / RTMP 拉流：tcp | udp） */
export type StreamTransport = 'tcp' | 'udp'
/** @deprecated 使用 StreamTransport */
export type RtspTransport = StreamTransport

/** GB28181 媒体传输：TCP 被动 / TCP 主动 / UDP / AUTO */
export type Gb28181Transport = 'tcp-passive' | 'tcp-active' | 'udp' | 'auto'

export type Camera = {
  id: number
  name: string
  ip: string
  port: number
  vendor: string
  group: string
  resolution: string
  fps: number
  status: CameraStatus
  protocol: CameraProtocol
  streamUrl?: string
  username?: string
  password?: string
  /** RTSP path / RTMP app/stream */
  path?: string
  /** 默认通道（主预览）；ONVIF / GB28181 */
  channel?: string
  /** GB28181 设备国标 ID（20 位） */
  deviceId?: string
  updatedAt: string
  createdAt?: string
  /** pull|push；RTMP 推流接入时为 push */
  ingestMode?: CameraIngestMode
  streamKey?: string
  publishUrl?: string
  pushTokenQuery?: string
  /** 仅创建/重置密钥时返回 */
  pushSecret?: string
  allowAnonymous?: boolean
  /** 国标准入：pending|active|rejected|disabled */
  enrollment?: string
  online?: boolean
  transport?: string
  remoteAddr?: string
  contactUri?: string
  registeredAt?: string
  lastKeepalive?: string
  protocolMeta?: string
  /** 通道子表（设备下一机多流） */
  channels?: CameraChannel[]
  channelCount?: number
}

/** 设备下可播放通道 */
export type CameraChannel = {
  id: number
  /** 国标通道编码；非国标为逻辑通道号 */
  channelId: string
  deviceId: string
  name?: string
  manufacturer?: string
  model?: string
  status?: string
  streamUrl?: string
  isDefault?: boolean
}

/** 地图图层 */
export type MapLayer = 'campus' | 'building' | 'parking'

/** 地图元素：楼栋 / 道路 / 摄像头点位 */
export type MapElementKind = 'building' | 'road' | 'camera'

export type MapElement = {
  id: string
  kind: MapElementKind
  layer: MapLayer
  label: string
  /** 左上角 X，百分比 0~100 */
  x: number
  /** 左上角 Y，百分比 0~100；摄像头点位为中心点 */
  y: number
  /** 楼栋 / 道路宽度百分比 */
  width?: number
  /** 楼栋 / 道路高度百分比 */
  height?: number
  /** 摄像头点位绑定的设备 ID */
  cameraId?: string
}

export type CameraInput = {
  name: string
  ip: string
  port: number
  vendor: string
  group: string
  resolution: string
  fps: number
  protocol: CameraProtocol
  streamUrl?: string
  username?: string
  password?: string
  path?: string
  channel?: string
  /** GB28181 设备国标 ID */
  deviceId?: string
  ingestMode?: CameraIngestMode
  streamKey?: string
  allowAnonymous?: boolean
  pushSecret?: string
  resetPushSecret?: boolean
  /** 推流 URL 展示用主机名 */
  advertiseHost?: string // 推流展示 URL 主机，可选；默认用当前浏览器主机名
  /** RTSP/RTMP 拉流：tcp|udp；GB28181：tcp-passive|tcp-active|udp|auto */
  transport?: string
}

export type ModelLicenseStatus = 'authorized' | 'expired' | 'disabled' | 'unavailable'

export type ModelFormat = 'ONNX' | 'RKNN' | 'TensorRT'

export type ModelItem = {
  id: string
  name: string
  description: string
  tags: string[]
  formats: ModelFormat[]
  licenseStatus: ModelLicenseStatus
  modelStatus: string
  version: string
  publishedAt: string
  localVersion: string | null
  localUpdatedAt: string | null
  /** 运行缩放倍数 */
  runtimeScale?: number
  /** 本地文件大小字节（接入时记录） */
  fileSizeBytes?: number
  fileName?: string
  /** 服务端本地权重路径 */
  localPath?: string
  /** 类别标签文件路径 */
  labelsPath?: string
}

export type ModelRegisterInput = {
  name: string
  description: string
  version: string
  formats: ModelFormat[]
  tags: string[]
  /** 选择的本地文件名 */
  fileName?: string
  fileSizeBytes?: number
  /** 服务端可访问的本地路径（可选，默认 models/{fileName}） */
  localPath?: string
  labelsPath?: string
}

export type AlgorithmInferTestResult = {
  ok: boolean
  latencyMs: number
  message: string
  detections: Array<{ cls: string; conf: number; bbox: [number, number, number, number] }>
  /** image | video */
  sampleKind?: string
  /** 实际用于推理的 JPEG（视频为抽帧），base64 无 data: 前缀 */
  previewJpegBase64?: string
}

export type ModelUpdateRecord = {
  id: string
  modelName: string
  currentVersion: string
  targetVersion: string
  operatedAt: string
  status: 'success' | 'failed' | 'running'
  message: string
}

export type WorkflowStatus = 'idle' | 'running' | 'stopped' | 'error'

export type WorkflowSummary = {
  id: string
  name: string
  description: string
  status: WorkflowStatus
  nodeCount: number
  edgeCount: number
  callCount: number
  successCount: number
  successRate: number
  lastRunAt: string | null
  updatedAt: string
}

export type NodeFieldType = 'string' | 'number' | 'boolean' | 'json' | 'select' | 'camera_info'

export type NodeFieldSpec = {
  key: string
  label: string
  type: NodeFieldType
  required?: boolean
  description?: string
  options?: Array<{ label: string; value: string }>
  min?: number
  max?: number
  step?: number
}

export type NodeSpec = {
  type: string
  label: string
  category: string
  description: string
  isTrigger?: boolean
  inputs: NodeFieldSpec[]
  outputs: NodeFieldSpec[]
}

export type WorkflowNodeData = {
  type: string
  label: string
  description: string
  config: Record<string, unknown>
  runState?: 'idle' | 'running' | 'done' | 'error'
  /** 前端校验：配置不完整时高亮节点 */
  configInvalid?: boolean
}

export type WorkflowDetail = WorkflowSummary & {
  nodes: Array<{
    id: string
    type: string
    position: { x: number; y: number }
    data: WorkflowNodeData
  }>
  edges: Array<{
    id: string
    source: string
    target: string
  }>
}

export type DebugEventKind =
  | 'workflow_start'
  | 'workflow_complete'
  | 'node_start'
  | 'node_complete'
  | 'node_error'
  | 'node_output'

export type DebugEvent = {
  id: string
  kind: DebugEventKind
  nodeId?: string
  message: string
  at: string
  payload?: unknown
}

export type AlgorithmRunStatus = 'uninitialized' | 'running' | 'stopped' | 'error'

export type Point2D = { x: number; y: number }

export type AlgorithmRuleField = {
  key: string
  label: string
  type: 'number' | 'boolean' | 'string' | 'select'
  /** 表单分组标题，如 检测 / 跟踪 / 告警 */
  group?: string
  description?: string
  options?: Array<{ label: string; value: string }>
  /** number 字段下限（含） */
  min?: number
  /** number 字段上限（含） */
  max?: number
  step?: number
  /** 保存时是否必填；区域类算法默认可在页面侧要求至少 1 个区域 */
  required?: boolean
}

export type CameraAlgorithm = {
  id: string
  algorithmId: string
  name: string
  description: string
  status: AlgorithmRunStatus
  modelId?: string
  /** 绑定的流/通道 ID */
  channelId?: string
  rules: Record<string, unknown>
  ruleSchema: AlgorithmRuleField[]
  /** 归一化坐标 0~1 的多边形区域 */
  regions: Point2D[][]
}

export type AlarmLevel = 'info' | 'warn' | 'critical'
export type AlarmStatus = 'open' | 'ack' | 'closed'

export type AlarmRecord = {
  id: number
  title: string
  level: AlarmLevel
  status: AlarmStatus
  cameraId: string
  cameraName: string
  /** 摄像头分组，用于联动规则匹配 */
  cameraGroup?: string
  algorithmName: string
  workflowId?: string
  workflowName?: string
  /** 由联动规则自动匹配时写入 */
  linkageRuleId?: number
  linkageRuleName?: string
  message: string
  occurredAt: string
  /** 关联结果文件 ID（抓拍 / 短视频等） */
  evidenceFileIds?: string[]
  /** Mock 证据说明（无真实媒体时展示） */
  evidenceNote?: string
}

/** 预警联动规则：条件匹配后自动绑定工作流 */
export type AlarmLinkageRule = {
  id: number
  name: string
  enabled: boolean
  /** 匹配级别；空数组表示全部级别 */
  levels: AlarmLevel[]
  /** 算法名称（精确匹配）；空表示全部算法 */
  algorithmName?: string
  /** 摄像头分组；空表示全部分组 */
  cameraGroup?: string
  workflowId: string
  workflowName: string
  /** 数字越小优先级越高 */
  priority: number
  updatedAt: string
}

export type AlarmLinkageRuleInput = {
  name: string
  enabled: boolean
  levels: AlarmLevel[]
  algorithmName?: string
  cameraGroup?: string
  workflowId: string
  workflowName: string
  priority: number
}

export type StreamTestResult = {
  ok: boolean
  latencyMs: number
  codec?: string
  resolution?: string
  message: string
}

export type AlgorithmParams = {
  /** @deprecated 检测阈值在绑定 rules 上配置 */
  confThreshold?: number
  iouThreshold?: number
  maxDetections?: number
}

export type AlgorithmCapability = {
  id: string
  name: string
  description: string
  category: string
  modelId: string
  modelName: string
  status: 'ready' | 'draft' | 'disabled'
  boundCameraCount: number
  tags: string[]
  params: AlgorithmParams
  updatedAt: string
}

export type AlgorithmInput = {
  name: string
  description: string
  category: string
  modelId: string
  tags: string[]
  status: AlgorithmCapability['status']
  params?: AlgorithmParams
}

/** 事件核查：Detection ≠ Alert */
export type ReviewEventKind = 'alert' | 'detection' | 'motion'
export type ReviewEventStatus = 'unreviewed' | 'reviewed'

/** 核查详情叠框（归一化 0~1） */
export type ReviewBox = {
  x: number
  y: number
  w: number
  h: number
  label?: string
}

export type ReviewEvent = {
  id: string
  kind: ReviewEventKind
  status: ReviewEventStatus
  title: string
  cameraId: string
  cameraName: string
  label: string
  objectClass?: string
  objectColor?: string
  ocrText?: string
  score?: number
  occurredAt: string
  thumbUrl?: string
  evidenceFileIds?: string[]
  message?: string
  /** 事件短切片时长（秒），Mock 预览用 */
  clipSec?: number
  boxes?: ReviewBox[]
  /** 检测提升为警报后的关联 ID */
  promotedAlertId?: string
}

export type ReviewKindCounts = Record<ReviewEventKind, number> & {
  unreviewed: number
}

export type RecordingMode = 'continuous' | 'event'

export type RecordingPolicy = {
  cameraId: string
  cameraName: string
  mode: RecordingMode
  enabled: boolean
  /** 事件触发：预录秒数 */
  preSeconds: number
  /** 事件触发：延录秒数 */
  postSeconds: number
  updatedAt: string
}

export type RecordingSegment = {
  id: string
  cameraId: string
  cameraName: string
  protocol: 'rtsp' | 'gb28181' | 'other'
  startAt: string
  endAt: string
  durationSec: number
  sizeBytes?: number
  source: 'platform' | 'device'
}

export type SearchHit = {
  id: string
  eventId: string
  title: string
  cameraId: string
  cameraName: string
  objectClass?: string
  objectColor?: string
  ocrText?: string
  status: ReviewEventStatus
  occurredAt: string
  thumbUrl?: string
  evidenceFileIds?: string[]
}

export type NotifyChannelKind = 'email' | 'webhook' | 'im'

export type NotifyChannel = {
  id: string
  name: string
  kind: NotifyChannelKind
  enabled: boolean
  /** 邮箱 / Webhook URL / IM webhook */
  endpoint: string
  updatedAt: string
}

export type NotifyDeliveryStatus = 'success' | 'failed' | 'pending'

export type NotifyDelivery = {
  id: string
  channelId: string
  channelName: string
  kind: NotifyChannelKind
  title: string
  status: NotifyDeliveryStatus
  detail: string
  sentAt: string
}
