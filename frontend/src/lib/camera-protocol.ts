import type {
  CameraIngestMode,
  CameraInput,
  CameraProtocol,
  Gb28181Transport,
  StreamTransport,
} from '@/types/api'

export const CAMERA_PROTOCOL_OPTIONS: { label: string; value: CameraProtocol }[] = [
  { label: 'RTSP', value: 'rtsp' },
  { label: 'RTMP', value: 'rtmp' },
  { label: 'ONVIF', value: 'onvif' },
  { label: 'GB28181', value: 'gb28181' },
]

export const CAMERA_PROTOCOL_LABEL: Record<CameraProtocol, string> = {
  rtsp: 'RTSP',
  rtmp: 'RTMP',
  onvif: 'ONVIF',
  hls: 'HLS',
  gb28181: 'GB28181',
}

export const RTMP_INGEST_MODE_OPTIONS: { label: string; value: CameraIngestMode }[] = [
  { label: '推流（OBS / 摄像头推到本机）', value: 'push' },
  { label: '拉流（平台去拉远端）', value: 'pull' },
]

/** RTSP 接入方式（与 RTMP 文案一致，便于设备页复用） */
export const RTSP_INGEST_MODE_OPTIONS = RTMP_INGEST_MODE_OPTIONS

export const RTSP_TRANSPORT_OPTIONS: { label: string; value: StreamTransport }[] = [
  { label: 'TCP', value: 'tcp' },
  { label: 'UDP', value: 'udp' },
]

/** GB28181 流传输模式（与国标 INVITE setup 对应） */
export const GB28181_TRANSPORT_OPTIONS: { label: string; value: Gb28181Transport }[] = [
  { label: 'TCP 被动', value: 'tcp-passive' },
  { label: 'TCP 主动', value: 'tcp-active' },
  { label: 'UDP', value: 'udp' },
  { label: 'AUTO', value: 'auto' },
]

/** @deprecated 使用 RTSP_TRANSPORT_OPTIONS */
export const STREAM_TRANSPORT_OPTIONS = RTSP_TRANSPORT_OPTIONS

export function normalizeGb28181Transport(raw?: string): Gb28181Transport {
  const t = (raw ?? '').trim().toLowerCase()
  if (t === 'tcp-active' || t === 'active') return 'tcp-active'
  if (t === 'tcp' || t === 'tcp-passive' || t === 'passive') return 'tcp-passive'
  if (t === 'auto') return 'auto'
  return 'udp'
}
/** 拼推流展示 URL 时用的主机：优先表单覆盖，否则当前浏览器访问主机 */
export function resolveAdvertiseHost(override?: string): string {
  const custom = (override ?? '').trim()
  if (custom) return custom
  if (typeof window !== 'undefined') {
    const host = window.location.hostname.trim()
    if (host) return host
  }
  return '127.0.0.1'
}

/** 将 publishUrl 中的主机换成当前可访问地址（便于复制给局域网设备） */
export function withAdvertiseHost(publishUrl: string, host?: string): string {
  const url = (publishUrl ?? '').trim()
  if (!url) return ''
  const h = resolveAdvertiseHost(host)
  return url.replace(/^(rtmps?:\/\/|rtsps?:\/\/)[^/:?#]+/i, `$1${h}`)
}

export const DEFAULT_PORT: Record<CameraProtocol, number> = {
  rtsp: 554,
  rtmp: 1935,
  onvif: 80,
  hls: 80,
  gb28181: 5060,
}

export const DEFAULT_PATH: Record<Exclude<CameraProtocol, 'hls' | 'gb28181'>, string> = {
  rtsp: 'stream1',
  rtmp: 'live/stream',
  onvif: '',
}

function encodeAuth(username?: string, password?: string): string {
  const user = (username ?? '').trim()
  if (!user) return ''
  const pass = password ?? ''
  return `${encodeURIComponent(user)}:${encodeURIComponent(pass)}@`
}

function normalizePath(path?: string): string {
  const p = (path ?? '').trim().replace(/^\/+/, '')
  return p
}

/** 由表单字段组装流地址（拉流）；推流模式返回空，由服务端下发 publishUrl */
export function buildStreamUrl(
  input: Pick<
    CameraInput,
    | 'protocol'
    | 'ip'
    | 'port'
    | 'username'
    | 'password'
    | 'path'
    | 'channel'
    | 'streamUrl'
    | 'deviceId'
    | 'ingestMode'
    | 'transport'
  >,
): string {
  const protocol = input.protocol
  if (protocol === 'hls') {
    return (input.streamUrl ?? '').trim()
  }

  if (protocol === 'gb28181') {
    const deviceId = (input.deviceId ?? input.ip ?? '').trim()
    if (!deviceId) return ''
    const channelId = (input.channel ?? '').trim() || deviceId
    const tr = normalizeGb28181Transport(input.transport)
    return `gb28181://${deviceId}/${channelId}?transport=${tr}`
  }

  if (protocol === 'rtmp' && (input.ingestMode ?? 'push') === 'push') {
    return ''
  }

  if (protocol === 'rtsp' && input.ingestMode === 'push') {
    return ''
  }

  const host = (input.ip ?? '').trim()
  if (!host) return ''

  const port = input.port > 0 ? input.port : DEFAULT_PORT[protocol]
  const auth = encodeAuth(input.username, input.password)

  if (protocol === 'rtsp') {
    const path = normalizePath(input.path) || DEFAULT_PATH.rtsp
    const tr = (input.transport ?? 'tcp').toLowerCase() === 'udp' ? 'udp' : 'tcp'
    return `rtsp://${auth}${host}:${port}/${path}#transport=${tr}`
  }

  if (protocol === 'rtmp') {
    const path = normalizePath(input.path) || DEFAULT_PATH.rtmp
    return `rtmp://${auth}${host}:${port}/${path}`
  }

  // onvif：channel 存 1-based Profile 序号；URL 使用 0-based subtype
  const channel = (input.channel ?? '').trim() || '1'
  let subtype = channel
  const n = Number.parseInt(channel, 10)
  if (Number.isFinite(n) && n >= 1) {
    subtype = String(n - 1)
  }
  return `onvif://${auth}${host}:${port}?subtype=${encodeURIComponent(subtype)}`
}

/** 从 URL scheme 推断协议 */
export function inferProtocol(url?: string): CameraProtocol | undefined {
  if (!url) return undefined
  const lower = url.trim().toLowerCase()
  if (lower.startsWith('rtsp://') || lower.startsWith('rtsps://') || lower.startsWith('rtsp-ingest://'))
    return 'rtsp'
  if (lower.startsWith('rtmp://') || lower.startsWith('rtmps://') || lower.startsWith('rtmp-ingest://'))
    return 'rtmp'
  if (lower.startsWith('onvif://')) return 'onvif'
  if (lower.startsWith('gb28181://')) return 'gb28181'
  if (lower.startsWith('http://') || lower.startsWith('https://')) return 'hls'
  return undefined
}

export type CameraValidationResult = { ok: true } | { ok: false; message: string }

/** 保存前校验必填与 scheme 一致性 */
export function validateCameraInput(input: CameraInput): CameraValidationResult {
  if (!input.name.trim()) {
    return { ok: false, message: '请填写设备名称' }
  }

  if (input.protocol === 'hls') {
    const url = (input.streamUrl ?? '').trim()
    if (!url) {
      return { ok: false, message: '请填写 HLS 流地址（以 .m3u8 结尾）' }
    }
    if (!/^https?:\/\//i.test(url)) {
      return { ok: false, message: 'HLS 地址须以 http:// 或 https:// 开头' }
    }
    if (!/\.m3u8(\?|$)/i.test(url)) {
      return { ok: false, message: 'HLS 地址须包含 .m3u8' }
    }
    return { ok: true }
  }

  if (input.protocol === 'gb28181') {
    const deviceId = (input.deviceId ?? input.ip ?? '').trim()
    if (!deviceId) {
      return { ok: false, message: '请填写设备国标 ID（20 位数字）' }
    }
    if (!/^\d{20}$/.test(deviceId)) {
      return { ok: false, message: '设备国标 ID 须为 20 位数字' }
    }
    const channelId = (input.channel ?? '').trim()
    if (channelId && !/^\d{20}$/.test(channelId)) {
      return { ok: false, message: '通道国标 ID 须为 20 位数字（可留空，默认与设备 ID 相同）' }
    }
    return { ok: true }
  }

  const isPush =
    (input.protocol === 'rtmp' || input.protocol === 'rtsp') &&
    (input.ingestMode ?? (input.protocol === 'rtmp' ? 'push' : 'pull')) === 'push'

  // 推流：主机/端口由系统协议配置决定，设备侧只校验名称
  if (isPush) {
    return { ok: true }
  }

  const protoLabel = CAMERA_PROTOCOL_LABEL[input.protocol] || input.protocol
  const isPullStream = input.protocol === 'rtsp' || input.protocol === 'rtmp'

  if (!input.ip.trim()) {
    return {
      ok: false,
      message: isPullStream
        ? `${protoLabel} 拉流需填写摄像头主机 / IP；若设备推到本机，请将接入方式改为「推流」`
        : `请填写 ${protoLabel} 主机 / IP`,
    }
  }
  if (!input.port || input.port <= 0) {
    return {
      ok: false,
      message: isPullStream
        ? `${protoLabel} 拉流需填写有效端口（RTSP 常见 554，RTMP 常见 1935）`
        : '请填写有效端口',
    }
  }

  if (input.protocol === 'rtsp' || input.protocol === 'rtmp') {
    if (!normalizePath(input.path) && !DEFAULT_PATH[input.protocol]) {
      return {
        ok: false,
        message:
          input.protocol === 'rtsp'
            ? '请填写流路径（如 stream1）'
            : '请填写应用路径（如 live/stream）',
      }
    }
  }

  if (input.protocol === 'onvif' && !(input.channel ?? '').trim()) {
    return { ok: false, message: '请填写 ONVIF Profile 序号（1=主码流）' }
  }

  const built = buildStreamUrl(input)
  const inferred = inferProtocol(built)
  if (inferred && inferred !== input.protocol) {
    return {
      ok: false,
      message: `流地址协议与所选不一致（当前期望 ${CAMERA_PROTOCOL_LABEL[input.protocol]}）`,
    }
  }

  return { ok: true }
}

/** 切换协议时的字段默认值（保留名称/分组/厂商等） */
export function protocolFieldDefaults(protocol: CameraProtocol): Partial<CameraInput> {
  if (protocol === 'hls') {
    return {
      protocol,
      ip: '',
      port: DEFAULT_PORT.hls,
      path: '',
      channel: '',
      deviceId: '',
      username: '',
      password: '',
      streamUrl: 'https://example.com/live/stream.m3u8',
      ingestMode: 'pull',
      streamKey: '',
      allowAnonymous: true,
      advertiseHost: '',
      transport: '',
    }
  }
  if (protocol === 'gb28181') {
    return {
      protocol,
      ip: '',
      port: DEFAULT_PORT.gb28181,
      path: '',
      channel: '',
      deviceId: '',
      username: '',
      password: '',
      streamUrl: '',
      ingestMode: 'pull',
      streamKey: '',
      allowAnonymous: true,
      advertiseHost: '',
      transport: 'tcp-passive',
    }
  }
  if (protocol === 'rtmp') {
    return {
      protocol,
      ip: '',
      port: 0,
      path: '',
      channel: '',
      deviceId: '',
      streamUrl: '',
      // 默认推流：主机/端口走系统「RTMP 接入」配置
      ingestMode: 'push',
      streamKey: '',
      allowAnonymous: true,
      advertiseHost: '',
      transport: 'tcp',
    }
  }
  if (protocol === 'rtsp') {
    return {
      protocol,
      ip: '',
      port: DEFAULT_PORT.rtsp,
      path: DEFAULT_PATH.rtsp,
      channel: '',
      deviceId: '',
      streamUrl: '',
      ingestMode: 'pull',
      streamKey: '',
      allowAnonymous: true,
      advertiseHost: '',
      transport: 'tcp',
    }
  }
  return {
    protocol,
    port: DEFAULT_PORT[protocol],
    path: protocol === 'onvif' ? '' : '',
    channel: protocol === 'onvif' ? '1' : '',
    deviceId: '',
    streamUrl: '',
    ingestMode: 'pull',
    streamKey: '',
    allowAnonymous: true,
    advertiseHost: '',
    transport: '',
  }
}
