import type { PreviewSlotSource } from '@/lib/preview-source'

const SEED_KEY = 'ff-preview-seed'

/** 跨页跳转实时监控时的种子（不放进 URL）。 */
export type PreviewSeed = PreviewSlotSource & {
  /** 为 true 时只填充该摄像头的全部通道流 */
  cameraOnly?: boolean
  /** 打开监控详情（全屏） */
  openDetail?: boolean
  /** 进入录像回放模式（国标） */
  openPlayback?: boolean
  /** 回放定位时刻（ISO） */
  seekAt?: string
}

/** 从其它页跳转预览时写入，不放进 URL（一屏多路由宫格状态管理）。 */
export function seedPreviewSource(src: PreviewSeed) {
  try {
    sessionStorage.setItem(SEED_KEY, JSON.stringify(src))
  } catch {
    /* ignore quota / private mode */
  }
}

/** 预览页启动时读取一次并清除。 */
export function takePreviewSeed(): PreviewSeed | null {
  try {
    const raw = sessionStorage.getItem(SEED_KEY)
    if (!raw) return null
    sessionStorage.removeItem(SEED_KEY)
    const parsed = JSON.parse(raw) as PreviewSeed
    if (!parsed?.cameraId || typeof parsed.cameraId !== 'string') return null
    return {
      cameraId: parsed.cameraId,
      channelId:
        typeof parsed.channelId === 'string' && parsed.channelId
          ? parsed.channelId
          : undefined,
      cameraOnly: parsed.cameraOnly === true,
      openDetail: parsed.openDetail === true,
      openPlayback: parsed.openPlayback === true,
      seekAt:
        typeof parsed.seekAt === 'string' && parsed.seekAt.trim()
          ? parsed.seekAt.trim()
          : undefined,
    }
  } catch {
    return null
  }
}
