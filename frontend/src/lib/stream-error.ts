/**
 * 拉流 / 国标错误码目录。
 * 与后端 pkg/fferrors（go-errors）42xxx 对齐；Error() 形如 "42001 设备离线"。
 * WebSocket 常见前缀：`webrtc/offer: 42001 …`
 */

export type StreamErrorDef = {
  code: number
  message: string
}

/** 与 pkg/fferrors 国标段一致 */
export const StreamErrors = {
  DEVICE_OFFLINE: {
    code: 42001,
    message: '设备离线',
  },
  DEVICE_CHANNEL_NOT_FOUND: {
    code: 42002,
    message: '设备或通道不存在',
  },
  INVITE_NOT_ALLOWED: {
    code: 42003,
    message: '设备尚未确认接入，请先在设备管理中确认',
  },
  SIP_NOT_STARTED: {
    code: 42004,
    message: '国标服务未就绪',
  },
  NO_RTP: {
    code: 42005,
    message: '已接通但未收到码流，请稍后重试或检查媒体地址',
  },
  DEVICE_TIMEOUT: {
    code: 42006,
    message: '设备响应超时，可能尚未就绪，请稍后重试',
  },
  PLAYBACK_RANGE_INVALID: {
    code: 42007,
    message: '回放时间范围无效',
  },
  GATEWAY_DISABLED: {
    code: 42008,
    message: '国标服务未启用',
  },
  MEDIA_PORT_EXHAUSTED: {
    code: 42009,
    message: '媒体端口不足，请稍后重试',
  },
  CONNECT_TIMEOUT: {
    code: 42010,
    message: '等待码流超时，摄像头可能尚未就绪，请稍后重试',
  },
  DEVICE_BUSY: {
    code: 42011,
    message: '设备忙，暂时无法接受拉流，请稍后重试',
  },
  STREAM_NOT_FOUND: {
    code: 40400,
    message: '流尚未就绪或不存在',
  },
  NO_PRODUCERS: {
    code: 40401,
    message: '上游未接通，流尚未就绪',
  },
  NO_MATCHING_MEDIA: {
    code: 40402,
    message: '尚未收到可用视频轨，请稍后重试',
  },
  PLAY_FAILED: {
    code: 42999,
    message: '媒体播放失败',
  },
} as const satisfies Record<string, StreamErrorDef>

const BY_CODE = new Map<number, string>(
  Object.values(StreamErrors).map((e) => [e.code, e.message]),
)

/** 从 `webrtc/offer: 42001 xxx` / `42001 xxx` 中提取错误码 */
export function parseStreamErrorCode(raw: string): number | null {
  const text = String(raw || '')
  const m = text.match(/(?:^|:\s*|\/)\s*(-?\d{4,5})\s+/)
  if (m) return Number(m[1])
  const loose = text.match(/\b(42\d{3}|4040[0-2]|400\d{2})\b/)
  if (loose) return Number(loose[1])
  return null
}

/** 将后端/WebSocket 错误转成面向用户的中文提示（优先按错误码） */
export function humanizeStreamError(raw: string): string {
  const text = String(raw || '').trim()
  if (!text) return '拉流失败'

  const code = parseStreamErrorCode(text)
  if (code != null) {
    const known = BY_CODE.get(code)
    if (known) return known
    // 未知码：去掉前缀与码，保留后端 Message
    const stripped = text
      .replace(/^[\w/-]+:\s*/, '')
      .replace(/^-?\d{4,5}\s+/, '')
      .trim()
    if (stripped) return stripped
  }

  const lower = text.toLowerCase()
  if (lower.includes('offline') || /device .+ offline/.test(lower)) {
    return StreamErrors.DEVICE_OFFLINE.message
  }
  if (
    lower.includes('invite not allowed') ||
    lower.includes('until confirmed')
  ) {
    return StreamErrors.INVITE_NOT_ALLOWED.message
  }
  if (lower.includes('486') || lower.includes('busy here')) {
    return StreamErrors.DEVICE_BUSY.message
  }
  if (lower.includes('no rtp') || lower.includes('media_ip reachable')) {
    return StreamErrors.NO_RTP.message
  }
  if (lower.includes('timeout') || lower.includes('i/o timeout')) {
    return StreamErrors.DEVICE_TIMEOUT.message
  }
  if (lower.includes('stream not found') || lower.includes('not found')) {
    return StreamErrors.STREAM_NOT_FOUND.message
  }
  if (lower.includes('connection refused') || lower.includes('econnrefused')) {
    return '无法连接媒体服务，请确认后台已启动'
  }

  return (
    text
      .replace(/^webrtc\/offer:\s*/i, '')
      .replace(/^mse:\s*/i, '')
      .replace(/^error:\s*/i, '')
      .replace(/^-?\d{4,5}\s+/, '')
      .trim() || '拉流失败'
  )
}

/** 连接超时（无画面）提示，与 ErrStreamConnectTimeout 一致 */
export const STREAM_CONNECT_TIMEOUT_HINT =
  StreamErrors.CONNECT_TIMEOUT.message

/** 国标设备是否尚未具备拉流条件 */
export function isGbDeviceNotReady(camera: {
  protocol?: string
  status?: string
  online?: boolean
  enrollment?: string
}): boolean {
  if (camera.protocol !== 'gb28181') return false
  if (
    camera.enrollment === 'pending' ||
    camera.enrollment === 'rejected' ||
    camera.enrollment === 'disabled'
  ) {
    return true
  }
  if (camera.online === false) return true
  if (camera.status === 'offline' || camera.status === 'stopped') return true
  return false
}

export function gbDeviceNotReadyHint(camera: {
  enrollment?: string
}): string {
  if (camera.enrollment === 'pending') {
    return StreamErrors.INVITE_NOT_ALLOWED.message
  }
  if (camera.enrollment === 'rejected' || camera.enrollment === 'disabled') {
    return '设备未启用，无法拉流'
  }
  return StreamErrors.DEVICE_OFFLINE.message
}
