import type { Gb28181RecordItem } from '@/apis/gb28181-api'
import type { ISODateString } from '@astryxdesign/core/Calendar'

export type PlaybackSegment = {
  id: string
  startMs: number
  endMs: number
  label: string
  streamUrl?: string
  /** 国标原始条目；平台本地录像可无 */
  raw?: Gb28181RecordItem
}

function parseTimeMs(s: string): number {
  return parseLocalDateTimeMs(s)
}

/** 解析国标/本地墙钟或 ISO 时间为毫秒时间戳 */
export function parseLocalDateTimeMs(s: string): number {
  const raw = s.trim()
  const m = raw.match(
    /^(\d{4})-(\d{2})-(\d{2})[T ](\d{2}):(\d{2}):(\d{2})/,
  )
  if (m) {
    return new Date(
      Number(m[1]),
      Number(m[2]) - 1,
      Number(m[3]),
      Number(m[4]),
      Number(m[5]),
      Number(m[6]),
    ).getTime()
  }
  const t = Date.parse(raw)
  return Number.isNaN(t) ? 0 : t
}

export function normalizeRecords(
  items: Gb28181RecordItem[],
): PlaybackSegment[] {
  return items
    .map((raw, i) => {
      const startMs = parseTimeMs(raw.start_time)
      const endMs = parseTimeMs(raw.end_time)
      return {
        id: `${raw.start_time}_${raw.end_time}_${i}`,
        startMs,
        endMs,
        label: raw.name?.trim() || `片段 ${i + 1}`,
        streamUrl: raw.stream_url,
        raw,
      }
    })
    .filter((s) => s.startMs > 0 && s.endMs > s.startMs)
    .sort((a, b) => a.startMs - b.startMs)
}

/** 按本地日历日拼查询区间（国标 RecordInfo 用墙钟时间，勿 toISOString） */
export function dayBounds(day: string): { start: string; end: string } {
  // day = YYYY-MM-DD
  return {
    start: `${day}T00:00:00`,
    end: `${day}T23:59:59`,
  }
}

export function formatDay(d = new Date()): ISODateString {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}` as ISODateString
}

export function formatClock(ms: number): string {
  const d = new Date(ms)
  const hh = String(d.getHours()).padStart(2, '0')
  const mm = String(d.getMinutes()).padStart(2, '0')
  const ss = String(d.getSeconds()).padStart(2, '0')
  return `${hh}:${mm}:${ss}`
}

/** 到片段区间的距离（落在区间内为 0） */
export function distToSegment(seg: PlaybackSegment, tMs: number): number {
  if (tMs < seg.startMs) return seg.startMs - tMs
  if (tMs > seg.endMs) return tMs - seg.endMs
  return 0
}

/**
 * 为事件时刻选录像片段：
 * 1) 覆盖该时刻的片段中取最短（避免命中跨天/整段长录像）
 * 2) 否则取最近片段（默认 10 分钟内）
 */
export function findSegmentForTime(
  segments: PlaybackSegment[],
  tMs: number,
  maxGapMs = 10 * 60_000,
): PlaybackSegment | null {
  if (!segments.length || !Number.isFinite(tMs)) return null

  const containing = segments.filter(
    (s) => tMs >= s.startMs && tMs <= s.endMs,
  )
  if (containing.length === 1) return containing[0]
  if (containing.length > 1) {
    return containing.reduce((a, b) =>
      b.endMs - b.startMs < a.endMs - a.startMs ? b : a,
    )
  }

  let best: PlaybackSegment | null = null
  let bestDist = Number.POSITIVE_INFINITY
  for (const s of segments) {
    const d = distToSegment(s, tMs)
    if (d < bestDist) {
      best = s
      bestDist = d
    }
  }
  if (!best || bestDist > maxGapMs) return null
  return best
}

export function formatDuration(ms: number): string {
  const sec = Math.max(0, Math.round(ms / 1000))
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = sec % 60
  if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
  return `${m}:${String(s).padStart(2, '0')}`
}

export function findSegmentAt(
  segments: PlaybackSegment[],
  tMs: number,
): PlaybackSegment | null {
  // 仅匹配覆盖时刻的片段；多段重叠时取最短
  return findSegmentForTime(segments, tMs, 0)
}
