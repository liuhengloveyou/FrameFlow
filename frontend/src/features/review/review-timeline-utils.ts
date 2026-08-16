import type { ReviewEvent, ReviewEventKind } from '@/types/api'

export type TimelineZoomLevel = {
  /** 每格秒数 */
  segmentDuration: number
  /** 每隔多少分钟显示一次时间标签（对齐整分） */
  timestampSpread: number
  label: string
}

/** 对齐 Frigate EventView：默认 1 分钟格，可放大到 30s / 10s */
export const REVIEW_ZOOM_LEVELS: TimelineZoomLevel[] = [
  { segmentDuration: 60, timestampSpread: 15, label: '1 分钟' },
  { segmentDuration: 30, timestampSpread: 5, label: '30 秒' },
  { segmentDuration: 10, timestampSpread: 1, label: '10 秒' },
]

export type TickSize = 5 | 8 | 12

export type TimelineSegment = {
  /** 段起点 Unix 秒（含） */
  startSec: number
  /** 当前筛选类型对应的严重级别条（无则 null） */
  primaryKind: ReviewEventKind | null
  /** 是否已核查（该段内存在已核查事件则降饱和，贴合 Frigate） */
  reviewed: boolean
  /** 各类型计数 */
  counts: Record<ReviewEventKind, number>
  total: number
  /** 密度 1–3，影响色条粗细 */
  density: 1 | 2 | 3
  jumpEventId: string | null
  /** 悬停预览用代表事件 */
  previewEvent: ReviewEvent | null
  showLabel: boolean
  label: string
  tickSize: TickSize
  /** 连续色条圆角：视觉上方=更新的时间邻段 */
  roundTop: boolean
  roundBottom: boolean
}

const KIND_RANK: Record<ReviewEventKind, number> = {
  alert: 3,
  detection: 2,
  motion: 1,
}

export const SEGMENT_PX = 8

/** 近 24 小时窗口（与 Frigate 核查页一致） */
export function timelineBoundsMs(now = new Date(), hours = 24) {
  const endMs = now.getTime()
  const startMs = endMs - hours * 3600 * 1000
  return { startMs, endMs }
}

/** @deprecated 使用 timelineBoundsMs */
export function dayBoundsMs(now = new Date()) {
  return timelineBoundsMs(now, 24)
}

export function alignToSegment(sec: number, segmentDuration: number, floor = true) {
  const n = Math.floor(sec / segmentDuration) * segmentDuration
  return floor ? n : n + segmentDuration
}

export function eventStartSec(e: ReviewEvent) {
  return Math.floor(new Date(e.occurredAt).getTime() / 1000)
}

export function eventEndSec(e: ReviewEvent) {
  const start = eventStartSec(e)
  const clip = Math.max(15, e.clipSec ?? 30)
  return start + clip
}

export function tickSizeFor(sec: number, timestampSpread: number): TickSize {
  const d = new Date(sec * 1000)
  const m = d.getMinutes()
  const s = d.getSeconds()
  if (s !== 0) return 5
  if (m % timestampSpread === 0) return 12
  if (m % (timestampSpread === 15 ? 5 : 1) === 0) return 8
  return 5
}

function formatSegmentLabel(sec: number) {
  const d = new Date(sec * 1000)
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

function pickJumpEvent(
  candidates: ReviewEvent[],
  preferKind?: ReviewEventKind,
): ReviewEvent | null {
  if (!candidates.length) return null
  const sorted = [...candidates].sort((a, b) => {
    const pref =
      (preferKind && a.kind === preferKind ? 0 : 1) -
      (preferKind && b.kind === preferKind ? 0 : 1)
    if (pref !== 0) return pref
    const rev =
      (a.status === 'unreviewed' ? 0 : 1) - (b.status === 'unreviewed' ? 0 : 1)
    if (rev !== 0) return rev
    const thumb = (b.thumbUrl ? 0 : 1) - (a.thumbUrl ? 0 : 1)
    if (thumb !== 0) return thumb
    return b.occurredAt.localeCompare(a.occurredAt)
  })
  return sorted[0] ?? null
}

function densityOf(total: number): 1 | 2 | 3 {
  if (total >= 4) return 3
  if (total >= 2) return 2
  return 1
}

/**
 * 生成自上而下「新→旧」的时间轴分段，结构贴近 Frigate EventSegment。
 */
export function buildTimelineSegments(opts: {
  events: ReviewEvent[]
  segmentDuration: number
  timestampSpread: number
  startMs: number
  endMs: number
  preferKind?: ReviewEventKind
}): TimelineSegment[] {
  const { events, segmentDuration, timestampSpread, preferKind } = opts
  const endSec = alignToSegment(Math.floor(opts.endMs / 1000), segmentDuration)
  const startSec = alignToSegment(Math.floor(opts.startMs / 1000), segmentDuration)
  const count = Math.max(1, Math.ceil((endSec - startSec) / segmentDuration))

  // 新→旧：从 end 往回
  const tops: number[] = []
  for (let i = 0; i < count; i++) {
    tops.push(endSec - (i + 1) * segmentDuration)
  }

  type Bucket = {
    counts: Record<ReviewEventKind, number>
    reviewed: boolean
    candidates: ReviewEvent[]
    byKind: Record<ReviewEventKind, ReviewEvent[]>
  }
  const buckets = new Map<number, Bucket>()

  for (const e of events) {
    const a = eventStartSec(e)
    const b = eventEndSec(e)
    const from = alignToSegment(a, segmentDuration)
    const to = alignToSegment(Math.max(a, b - 1), segmentDuration)
    for (let t = from; t <= to; t += segmentDuration) {
      if (t < startSec || t >= endSec) continue
      let bucket = buckets.get(t)
      if (!bucket) {
        bucket = {
          counts: { alert: 0, detection: 0, motion: 0 },
          reviewed: false,
          candidates: [],
          byKind: { alert: [], detection: [], motion: [] },
        }
        buckets.set(t, bucket)
      }
      bucket.counts[e.kind] += 1
      if (e.status === 'reviewed') bucket.reviewed = true
      bucket.candidates.push(e)
      bucket.byKind[e.kind].push(e)
    }
  }

  const hasKind = (segStart: number, kind: ReviewEventKind) => {
    const b = buckets.get(segStart)
    return !!b && b.counts[kind] > 0
  }

  return tops.map((segStart) => {
    const bucket = buckets.get(segStart)
    const total = bucket
      ? bucket.counts.alert + bucket.counts.detection + bucket.counts.motion
      : 0

    let primaryKind: ReviewEventKind | null = null
    if (bucket && total > 0) {
      if (preferKind && bucket.counts[preferKind] > 0) {
        primaryKind = preferKind
      } else {
        primaryKind = (['alert', 'detection', 'motion'] as ReviewEventKind[]).reduce(
          (best, k) => {
            if (!bucket.counts[k]) return best
            if (!best) return k
            return KIND_RANK[k] > KIND_RANK[best] ? k : best
          },
          null as ReviewEventKind | null,
        )
      }
    }

    const d = new Date(segStart * 1000)
    const showLabel =
      d.getSeconds() === 0 && d.getMinutes() % timestampSpread === 0

    // 邻段：视觉上方=更新(segStart+dur)，下方=更旧(segStart-dur)
    let roundTop = false
    let roundBottom = false
    if (primaryKind) {
      const above = segStart + segmentDuration
      const below = segStart - segmentDuration
      roundTop = !hasKind(above, primaryKind)
      roundBottom = !hasKind(below, primaryKind)
    }

    const jumpCandidates =
      primaryKind && bucket ? bucket.byKind[primaryKind] : bucket?.candidates ?? []
    const previewEvent = pickJumpEvent(jumpCandidates, preferKind)

    return {
      startSec: segStart,
      primaryKind,
      reviewed: !!bucket?.reviewed,
      counts: bucket?.counts ?? { alert: 0, detection: 0, motion: 0 },
      total,
      density: densityOf(total),
      jumpEventId: previewEvent?.id ?? null,
      previewEvent,
      showLabel,
      label: formatSegmentLabel(segStart),
      tickSize: tickSizeFor(segStart, timestampSpread),
      roundTop,
      roundBottom,
    }
  })
}

/** 根据网格当前可见事件，找最近的时间轴段起点 */
export function nearestSegmentStart(timeMs: number, segmentDuration: number) {
  return alignToSegment(Math.floor(timeMs / 1000), segmentDuration)
}

export function segmentIndexForTime(
  segments: TimelineSegment[],
  timeSec: number,
  segmentDuration: number,
) {
  const aligned = alignToSegment(timeSec, segmentDuration)
  return segments.findIndex((s) => s.startSec === aligned)
}
