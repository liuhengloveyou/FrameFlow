import type { RecordingSegment } from '@/types/api'
import {
  parseLocalDateTimeMs,
  type PlaybackSegment,
} from '@/features/preview/playback-utils'

/** RecordingSegment → PlaybackTimeline 可用片段 */
export function toPlaybackSegments(items: RecordingSegment[]): PlaybackSegment[] {
  return items
    .map((r) => {
      const startMs = parseLocalDateTimeMs(r.startAt)
      const endMs = parseLocalDateTimeMs(r.endAt)
      return {
        id: r.id,
        startMs,
        endMs,
        label:
          r.source === 'device'
            ? '设备侧'
            : r.source === 'platform'
              ? '平台本地'
              : r.cameraName,
        streamUrl: undefined,
      } satisfies PlaybackSegment
    })
    .filter((s) => s.startMs > 0 && s.endMs > s.startMs)
    .sort((a, b) => a.startMs - b.startMs)
}

export function localDateKey(d = new Date()) {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}
