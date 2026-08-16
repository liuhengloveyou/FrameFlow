import type { Camera } from '@/types/api'
import { channelBizId } from '@/lib/preview-source'

/**
 * 解析 AI 绑定用的流/通道 ID。
 * 优先显式通道；否则默认通道 → camera.channel → 设备 id（单流设备）。
 */
export function resolveAiChannelId(
  camera: Camera,
  channelId?: string | null,
): string {
  const explicit = channelId?.trim()
  if (explicit) return explicit
  const def =
    camera.channels?.find((c) => c.isDefault) ?? camera.channels?.[0]
  if (def) return channelBizId(def)
  if (camera.channel?.trim()) return camera.channel.trim()
  return String(camera.id)
}

/** 本地缓存 / 查询 key：cameraId::channelId */
export function aiStreamStoreKey(cameraId: string, channelId: string): string {
  return `${cameraId}::${channelId}`
}
