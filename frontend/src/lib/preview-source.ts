/** Resolve go2rtc stream source for a camera (+ optional channel). */
import type { Camera, CameraChannel } from '@/types/api'

export type StreamProfile = 'main' | 'sub'

export type PreviewSlotSource = {
  cameraId: string
  /** 国标通道号或通道主键；缺省则用设备默认流 */
  channelId?: string
}

const SLOT_SEP = '::'

export function encodeSlotKey(src: PreviewSlotSource): string {
  if (src.channelId) return `${src.cameraId}${SLOT_SEP}${src.channelId}`
  return src.cameraId
}

export function parseSlotKey(key: string): PreviewSlotSource {
  const i = key.indexOf(SLOT_SEP)
  if (i >= 0) {
    return { cameraId: key.slice(0, i), channelId: key.slice(i + SLOT_SEP.length) }
  }
  return { cameraId: key }
}

/** Named stream key registered in go2rtc (camera id or camera/channel). */
export function streamNameFor(cameraId: string, channelId?: string): string {
  if (!channelId || channelId === cameraId) return cameraId
  return `${cameraId}/${channelId}`
}

/** 通道业务键：国标通道编码优先，否则数字主键。 */
export function channelBizId(ch: CameraChannel): string {
  return ch.channelId || String(ch.id)
}

export function resolveChannel(
  camera: Camera,
  channelId?: string,
): CameraChannel | undefined {
  const list = camera.channels ?? []
  if (channelId) {
    return list.find(
      (c) => String(c.id) === channelId || c.channelId === channelId,
    )
  }
  return list.find((c) => c.isDefault) ?? list[0]
}

/**
 * 解析 go2rtc 的 src：
 * - 浏览器只连 FrameFlow（WebRTC / MSE）
 * - FrameFlow 再按 streamUrl 协议拉设备（gb28181 / rtsp / …）
 * - 国标预览默认辅码流（stream=sub），可切主码流
 */
export function resolveStreamSrc(
  camera: Camera,
  channelId?: string,
  stream: StreamProfile = 'sub',
): string {
  const ch = resolveChannel(camera, channelId)
  const deviceId = camera.deviceId || String(camera.id)
  const gbChannel = ch ? channelBizId(ch) : camera.channel || channelId

  if (camera.protocol === 'gb28181') {
    const profile = stream === 'main' ? 'main' : 'sub'
    if (gbChannel) {
      const base = ch?.streamUrl || `gb28181://${deviceId}/${gbChannel}`
      return withStreamQuery(base, profile)
    }
    if (camera.streamUrl?.startsWith('gb28181://')) {
      return withStreamQuery(camera.streamUrl, profile)
    }
    return withStreamQuery(`gb28181://${deviceId}/${deviceId}`, profile)
  }

  // RTMP / RTSP 推流：预览 src 必须是 streams 注册名（stream_key）
  if (
    (camera.protocol === 'rtmp' || camera.protocol === 'rtsp') &&
    camera.ingestMode === 'push'
  ) {
    const key = (camera.streamKey || '').trim()
    if (key) return key
    const marker = (ch?.streamUrl || camera.streamUrl || '').trim()
    if (marker.startsWith('rtmp-ingest://')) {
      return marker.slice('rtmp-ingest://'.length)
    }
    if (marker.startsWith('rtsp-ingest://')) {
      return marker.slice('rtsp-ingest://'.length)
    }
  }

  if (ch?.streamUrl) return ch.streamUrl
  if (camera.streamUrl) return camera.streamUrl
  return String(camera.id)
}

function withStreamQuery(rawURL: string, stream: StreamProfile): string {
  // gb28181:// 在部分环境下 URL 解析不稳定，用字符串拼接更稳妥
  const cleaned = rawURL.replace(/([?&])stream=[^&]*/g, '$1').replace(/[?&]$/, '')
  const sep = cleaned.includes('?') ? '&' : '?'
  return `${cleaned}${sep}stream=${stream}`
}

export function buildWsSrc(streamSrc: string): string {
  return `/api/ws?src=${encodeURIComponent(streamSrc)}`
}
