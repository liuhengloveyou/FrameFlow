import axios from 'axios'

/** FrameFlow 原生国标 API */
const nativeHttp = axios.create({
  baseURL: '',
  timeout: 30000,
  withCredentials: true,
})

export type Gb28181Device = {
  id: string
  name?: string
  status?: 'pending' | 'active' | 'rejected' | 'disabled' | string
  online: boolean
  remote_addr?: string
  transport?: string
  channels: number
  stream_url?: string
}

export type Gb28181Channel = {
  id: string
  device_id: string
  name?: string
  manufacturer?: string
  model?: string
  status?: string
}

export type Gb28181Status = {
  listen?: string
  serial?: string
  realm?: string
  sip_ip?: string
  media_ip?: string
  db?: string
  allow_unknown?: boolean
  devices?: number
  online?: number
  pending?: number
}

export async function getGb28181Status(): Promise<Gb28181Status> {
  const { data } = await nativeHttp.get<Gb28181Status>('/api/gb28181')
  return data
}

export async function listGb28181Devices(): Promise<Gb28181Device[]> {
  const { data } = await nativeHttp.get<Gb28181Device[]>('/api/gb28181/devices')
  return Array.isArray(data) ? data : []
}

export async function addGb28181Device(input: {
  id: string
  name?: string
  password?: string
}): Promise<Gb28181Device> {
  const { data } = await nativeHttp.post<Gb28181Device>('/api/gb28181/devices', input)
  return data
}

export async function updateGb28181Device(
  id: string,
  input: { name?: string; password?: string },
): Promise<Gb28181Device> {
  const { data } = await nativeHttp.put<Gb28181Device>(`/api/gb28181/devices/${id}`, input)
  return data
}

export async function deleteGb28181Device(id: string): Promise<void> {
  await nativeHttp.delete(`/api/gb28181/devices/${id}`)
}

export async function confirmGb28181Device(id: string): Promise<Gb28181Device> {
  const { data } = await nativeHttp.post<Gb28181Device>(
    `/api/gb28181/devices/${id}/confirm`,
  )
  return data
}

export async function rejectGb28181Device(id: string): Promise<Gb28181Device> {
  const { data } = await nativeHttp.post<Gb28181Device>(
    `/api/gb28181/devices/${id}/reject`,
  )
  return data
}

export async function listGb28181Channels(deviceId: string): Promise<Gb28181Channel[]> {
  const { data } = await nativeHttp.get<Gb28181Channel[]>(
    `/api/gb28181/devices/${deviceId}/channels`,
  )
  return data
}

/** 手动向设备发起 Catalog 查询并同步通道 */
export async function syncGb28181Catalog(deviceId: string): Promise<void> {
  await nativeHttp.post(`/api/gb28181/devices/${deviceId}/catalog`)
}

export type PtzCmd =
  | 'left'
  | 'right'
  | 'up'
  | 'down'
  | 'zoomin'
  | 'zoomout'
  | 'stop'

/** 国标云台控制 */
export async function postPtz(
  deviceId: string,
  channelId: string,
  body: { cmd: PtzCmd; speed?: number },
): Promise<{ status: string; ptz: string }> {
  const { data } = await nativeHttp.post<{ status: string; ptz: string }>(
    `/api/gb28181/devices/${deviceId}/channels/${channelId}/ptz`,
    {
      cmd: body.cmd,
      speed: body.speed ?? 0,
    },
  )
  return data
}

export type Gb28181RecordItem = {
  device_id?: string
  name?: string
  start_time: string
  end_time: string
  type?: string
  file_path?: string
  file_size?: string
  stream_url?: string
}

/** 查询通道录像目录（RecordInfo）；超时较长 */
export async function listGb28181Records(
  deviceId: string,
  channelId: string,
  range: { start: string; end: string },
  opts?: { signal?: AbortSignal },
): Promise<Gb28181RecordItem[]> {
  const { data } = await nativeHttp.get<Gb28181RecordItem[]>(
    `/api/gb28181/devices/${deviceId}/channels/${channelId}/records`,
    {
      params: { start: range.start, end: range.end },
      timeout: 120_000,
      signal: opts?.signal,
    },
  )
  return Array.isArray(data) ? data : []
}

/** 拼回放流地址（走现有 /api/ws?src=…，由后端 Dial 识别 start/end） */
export function buildPlaybackStreamUrl(
  deviceId: string,
  channelId: string,
  startUnix: number,
  endUnix: number,
): string {
  return `gb28181://${deviceId}/${channelId}?start=${startUnix}&end=${endUnix}`
}

export type Gb28181Settings = {
  enabled: boolean
  listenPort: number
  serial: string
  realm: string
  password?: string
  hasPassword: boolean
  sipIp: string
  mediaIp: string
  mediaPort: string
  transport: string
  allowUnknown: boolean
  runningListen?: string
  resolvedSipIp?: string
  resolvedMediaIp?: string
  restartHint?: string
}

export type Gb28181SettingsPatch = {
  enabled?: boolean
  listenPort?: number
  serial?: string
  realm?: string
  password?: string
  sipIp?: string
  mediaIp?: string
  mediaPort?: string
  transport?: string
  allowUnknown?: boolean
}

export type PortCheckResult = {
  port: number
  available: boolean
  message?: string
}

export async function getGb28181Settings(): Promise<Gb28181Settings> {
  const { data } = await nativeHttp.get<Gb28181Settings>('/api/gb28181/settings')
  return data
}

export async function updateGb28181Settings(
  patch: Gb28181SettingsPatch,
): Promise<Gb28181Settings> {
  const { data } = await nativeHttp.put<Gb28181Settings>(
    '/api/gb28181/settings',
    patch,
  )
  return data
}

export async function checkGb28181Port(port: number): Promise<PortCheckResult> {
  const { data } = await nativeHttp.get<PortCheckResult>('/api/gb28181/port-check', {
    params: { port },
  })
  return data
}
