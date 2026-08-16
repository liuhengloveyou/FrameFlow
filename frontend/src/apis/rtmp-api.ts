import axios from 'axios'

const nativeHttp = axios.create({
  baseURL: '',
  timeout: 30000,
  withCredentials: true,
})

export type RtmpSettings = {
  enabled: boolean
  listenPort: number
  advertiseHost: string
  publishExample: string
  runningListen: string
  restartHint?: string
  candidateHosts: string[]
  pushTokenQuery: string
}

export type RtmpSettingsPatch = {
  enabled?: boolean
  listenPort?: number
  advertiseHost?: string
}

export type PortCheckResult = {
  port: number
  available: boolean
  message?: string
}

export async function getRtmpSettings(): Promise<RtmpSettings> {
  const { data } = await nativeHttp.get<RtmpSettings>('/api/rtmp/settings')
  return data
}

export async function updateRtmpSettings(
  patch: RtmpSettingsPatch,
): Promise<RtmpSettings> {
  const { data } = await nativeHttp.put<RtmpSettings>('/api/rtmp/settings', patch)
  return data
}

export async function checkRtmpPort(port: number): Promise<PortCheckResult> {
  const { data } = await nativeHttp.get<PortCheckResult>('/api/rtmp/port-check', {
    params: { port },
  })
  return data
}
