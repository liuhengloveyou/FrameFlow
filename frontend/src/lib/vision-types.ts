/** 与后端 vision.VisionBBox / VisionFramePayload 对齐（归一化 0~1） */
export type VisionBox = {
  x: number
  y: number
  w: number
  h: number
  label?: string
  score?: number
  text?: string
  /** 后端航迹 ID：同一目标跨帧稳定，用于平滑渲染的稳定 key */
  track_id?: number
  /** 连续丢失拍数：>0 表示该框是航迹保持中的外推位置（非本拍真实检测） */
  miss?: number
}

export type VisionFramePayload = {
  type?: string
  v?: number
  stream?: string
  event?: string
  unix_ms?: number
  boxes?: VisionBox[]
  text?: string
}

export function parseVisionPayload(raw: unknown): VisionBox[] {
  if (!raw || typeof raw !== 'object') return []
  const boxes = (raw as VisionFramePayload).boxes
  if (!Array.isArray(boxes)) return []
  return boxes.filter(
    (b) =>
      b &&
      typeof b.x === 'number' &&
      typeof b.y === 'number' &&
      typeof b.w === 'number' &&
      typeof b.h === 'number',
  )
}
