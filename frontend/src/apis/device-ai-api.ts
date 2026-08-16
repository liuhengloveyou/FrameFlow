import { http } from '@/lib/http'
import type { CameraAlgorithm, Point2D } from '@/types/api'
import {
  hasAiConfigErrors,
  validateAiConfig,
} from '@/lib/validate-ai-config'

/** 设备 AI：通道算法绑定走真实 /api/v1 */

export async function listCameraAlgorithms(
  cameraId: number | string,
  channelId: string,
): Promise<CameraAlgorithm[]> {
  const { data } = await http.get(
    `/cameras/${cameraId}/channels/${encodeURIComponent(channelId)}/algorithms`,
  )
  return data.data
}

export async function algorithmAction(
  cameraId: number | string,
  channelId: string,
  bindId: string,
  action: 'init' | 'start' | 'stop' | 'release',
): Promise<CameraAlgorithm> {
  const { data } = await http.post(
    `/cameras/${cameraId}/channels/${encodeURIComponent(channelId)}/algorithms/${bindId}/${action}`,
  )
  return data.data
}

export async function saveCameraAlgorithm(
  cameraId: number | string,
  channelId: string,
  bindId: string,
  patch: { rules?: Record<string, unknown>; regions?: Point2D[][] },
  ruleSchema?: CameraAlgorithm['ruleSchema'],
): Promise<CameraAlgorithm> {
  if (ruleSchema && patch.rules) {
    const errors = validateAiConfig(
      ruleSchema,
      patch.rules,
      patch.regions ?? [],
      { requireRegions: false },
    )
    if (hasAiConfigErrors(errors)) {
      throw new Error(
        errors.regions ?? Object.values(errors.fields)[0] ?? '参数无效',
      )
    }
  }
  const { data } = await http.put(
    `/cameras/${cameraId}/channels/${encodeURIComponent(channelId)}/algorithms/${bindId}`,
    patch,
  )
  return data.data
}

export async function bindCameraAlgorithm(
  cameraId: number | string,
  channelId: string,
  algorithmId: string,
): Promise<CameraAlgorithm> {
  const { data } = await http.post(
    `/cameras/${cameraId}/channels/${encodeURIComponent(channelId)}/algorithms`,
    { algorithmId },
  )
  return data.data
}

export async function unbindCameraAlgorithm(
  cameraId: number | string,
  channelId: string,
  bindId: string,
): Promise<void> {
  await http.delete(
    `/cameras/${cameraId}/channels/${encodeURIComponent(channelId)}/algorithms/${bindId}`,
  )
}
