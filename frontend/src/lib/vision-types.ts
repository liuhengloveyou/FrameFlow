/** 与后端 vision.VisionBBox / VisionFramePayload 对齐（归一化 0~1） */
export type VisionBox = {
  x: number
  y: number
  w: number
  h: number
  label?: string
  score?: number
  text?: string
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
