import axios from 'axios'

/** FrameFlow 原生 ONVIF（发现 / Profile / 协议配置） */
const nativeHttp = axios.create({
  baseURL: '',
  timeout: 20000,
  withCredentials: true,
})

export type OnvifSource = {
  id?: string
  name?: string
  info?: string
  url?: string
  /** 发现结果可携带协议配置中的默认用户名 */
  location?: string
}

export type OnvifDiscoverResult = {
  sources: OnvifSource[]
}

export type OnvifSettings = {
  serverEnabled: boolean
  defaultUsername: string
  hasDefaultPassword: boolean
  discoverTimeoutSec: number
  clientExample?: string
  serverPath?: string
}

export type OnvifSettingsPatch = {
  serverEnabled?: boolean
  defaultUsername?: string
  /** 传空字符串清空；不传则不改 */
  defaultPassword?: string
  discoverTimeoutSec?: number
}

/** 局域网 WS-Discovery 扫描 ONVIF 设备 */
export async function discoverOnvifDevices(): Promise<OnvifSource[]> {
  const { data } = await nativeHttp.get<OnvifDiscoverResult>('/api/onvif')
  return data.sources ?? []
}

/** 列出设备 Media Profiles。src 形如 onvif://user:pass@host:port */
export async function listOnvifProfiles(src: string): Promise<OnvifSource[]> {
  const { data } = await nativeHttp.get<OnvifDiscoverResult>('/api/onvif', {
    params: { src },
  })
  return data.sources ?? []
}

export async function getOnvifSettings(): Promise<OnvifSettings> {
  const { data } = await nativeHttp.get<OnvifSettings>('/api/onvif/settings')
  return data
}

export async function updateOnvifSettings(
  patch: OnvifSettingsPatch,
): Promise<OnvifSettings> {
  const { data } = await nativeHttp.put<OnvifSettings>('/api/onvif/settings', patch)
  return data
}

/** 从 onvif:// URL 解析主机与端口 */
export function parseOnvifUrl(raw: string): {
  host: string
  port: number
  username: string
  password: string
} | null {
  try {
    const u = new URL(raw.replace(/^onvif:/i, 'http:'))
    const port = u.port ? Number(u.port) : 80
    return {
      host: u.hostname,
      port: Number.isFinite(port) && port > 0 ? port : 80,
      username: decodeURIComponent(u.username || ''),
      password: decodeURIComponent(u.password || ''),
    }
  } catch {
    return null
  }
}

/** 从 profile 列表 URL（含 ?subtype=）提取 1-based 通道号 */
export function subtypeToChannel(profileUrl: string): string {
  try {
    const u = new URL(profileUrl.replace(/^onvif:/i, 'http:'))
    const subtype = u.searchParams.get('subtype') ?? '0'
    const n = Number.parseInt(subtype, 10)
    if (Number.isFinite(n) && n >= 0) {
      return String(n + 1)
    }
    return '1'
  } catch {
    return '1'
  }
}
