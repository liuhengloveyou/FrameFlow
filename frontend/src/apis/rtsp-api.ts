import axios from 'axios'

const nativeHttp = axios.create({
  baseURL: '',
  timeout: 30000,
  withCredentials: true,
})

export type RtspSettings = {
  enabled: boolean
  listenPort: number
  advertiseHost: string
  username: string
  hasPassword: boolean
  defaultQuery: string
  playExample: string
  runningListen: string
  restartHint?: string
  candidateHosts: string[]
}

export type RtspSettingsPatch = {
  enabled?: boolean
  listenPort?: number
  advertiseHost?: string
  username?: string
  /** 传空字符串清空密码；不传则不改 */
  password?: string
  defaultQuery?: string
}

export type PortCheckResult = {
  port: number
  available: boolean
  message?: string
}

export async function getRtspSettings(): Promise<RtspSettings> {
  const { data } = await nativeHttp.get<RtspSettings>('/api/rtsp/settings')
  return data
}

export async function updateRtspSettings(
  patch: RtspSettingsPatch,
): Promise<RtspSettings> {
  const { data } = await nativeHttp.put<RtspSettings>('/api/rtsp/settings', patch)
  return data
}

export async function checkRtspPort(port: number): Promise<PortCheckResult> {
  const { data } = await nativeHttp.get<PortCheckResult>('/api/rtsp/port-check', {
    params: { port },
  })
  return data
}
