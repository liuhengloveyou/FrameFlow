import axios from 'axios'
import {
  buildStreamUrl,
  validateCameraInput,
} from '@/lib/camera-protocol'
import type { Camera, CameraChannel, CameraInput, CameraProtocol, PageResult, StreamTestResult } from '@/types/api'

/** FrameFlow 原生摄像头 API（/api/cameras） */
const nativeHttp = axios.create({
  baseURL: '',
  timeout: 30000,
  withCredentials: true,
})

export type CameraQuery = {
  page?: number
  pageSize?: number
  name?: string
  status?: string
  group?: string
  protocol?: string
}

function normalizeInput(input: CameraInput | Partial<CameraInput>): CameraInput {
  const protocol = (input.protocol ?? 'rtsp') as CameraProtocol
  const deviceId = (input.deviceId ?? '').trim()
  const ingestMode =
    input.ingestMode ?? (protocol === 'rtmp' ? 'push' : 'pull')
  const isPush =
    ingestMode === 'push' && (protocol === 'rtmp' || protocol === 'rtsp')
  const merged: CameraInput = {
    name: input.name ?? '',
    ip:
      protocol === 'gb28181'
        ? deviceId || (input.ip ?? '')
        : isPush
          ? ''
          : (input.ip ?? ''),
    port: isPush ? 0 : (input.port ?? 0),
    vendor: input.vendor ?? '',
    group: input.group ?? '',
    resolution: input.resolution ?? '1920x1080',
    fps: input.fps ?? 25,
    protocol,
    username: input.username,
    password: input.password,
    path: input.path,
    channel: input.channel,
    deviceId,
    streamUrl: input.streamUrl,
    ingestMode,
    streamKey: isPush ? '' : input.streamKey,
    allowAnonymous: isPush ? true : input.allowAnonymous,
    pushSecret: undefined,
    resetPushSecret: false,
    advertiseHost: isPush ? '' : input.advertiseHost,
    transport: input.transport,
  }
  const streamUrl =
    protocol === 'hls'
      ? (merged.streamUrl ?? '').trim()
      : isPush
        ? ''
        : buildStreamUrl(merged)
  return { ...merged, streamUrl }
}

export async function listCameras(query: CameraQuery = {}): Promise<PageResult<Camera>> {
  const { data } = await nativeHttp.get<PageResult<Camera>>('/api/cameras', {
    params: {
      name: query.name || undefined,
      status: query.status || undefined,
      group: query.group || undefined,
      protocol: query.protocol || undefined,
      page: query.page ?? 1,
      pageSize: query.pageSize ?? 20,
    },
  })
  return {
    items: data.items ?? [],
    total: data.total ?? 0,
    page: data.page ?? query.page ?? 1,
    pageSize: data.pageSize ?? query.pageSize ?? 20,
  }
}

export async function getCamera(id: number | string): Promise<Camera> {
  const { data } = await nativeHttp.get<Camera>(`/api/cameras/${id}`)
  return data
}

export async function createCamera(input: CameraInput): Promise<Camera> {
  const payload = normalizeInput(input)
  const check = validateCameraInput(payload)
  if (!check.ok) throw new Error(check.message)
  const { data } = await nativeHttp.post<Camera>('/api/cameras', payload)
  return data
}

export async function updateCamera(
  id: number | string,
  input: Partial<CameraInput>,
): Promise<Camera> {
  const payload = normalizeInput({
    ...(input as CameraInput),
    name: input.name ?? '',
    protocol: (input.protocol ?? 'rtsp') as CameraProtocol,
  })
  const check = validateCameraInput(payload)
  if (!check.ok) throw new Error(check.message)
  const { data } = await nativeHttp.put<Camera>(`/api/cameras/${id}`, payload)
  return data
}

export async function deleteCamera(id: number | string): Promise<void> {
  await nativeHttp.delete(`/api/cameras/${id}`)
}

export async function startCamera(id: number | string): Promise<Camera> {
  const { data } = await nativeHttp.post<Camera>(`/api/cameras/${id}/start`)
  return data
}

export async function stopCamera(id: number | string): Promise<Camera> {
  const { data } = await nativeHttp.post<Camera>(`/api/cameras/${id}/stop`)
  return data
}

export async function testCameraStream(id: number | string): Promise<StreamTestResult> {
  const { data } = await nativeHttp.post<StreamTestResult>(`/api/cameras/${id}/test-stream`)
  return data
}

export async function listCameraChannels(id: number | string): Promise<CameraChannel[]> {
  const { data } = await nativeHttp.get<CameraChannel[]>(`/api/cameras/${id}/channels`)
  return Array.isArray(data) ? data : []
}
