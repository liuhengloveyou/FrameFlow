export type ReviewRangePreset = '24h' | 'today' | 'yesterday' | '7d' | 'day'

export type ReviewTimeRange = {
  preset: ReviewRangePreset
  /** 自定义日：本地 YYYY-MM-DD；仅 preset=day 时有效 */
  day?: string
}

export function localDateKey(d = new Date()) {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

export function resolveReviewRangeBounds(
  range: ReviewTimeRange,
  now = new Date(),
): { startMs: number; endMs: number } {
  const endNow = now.getTime()

  if (range.preset === '24h') {
    return { startMs: endNow - 24 * 3600 * 1000, endMs: endNow }
  }

  if (range.preset === '7d') {
    return { startMs: endNow - 7 * 24 * 3600 * 1000, endMs: endNow }
  }

  if (range.preset === 'today') {
    const start = new Date(now)
    start.setHours(0, 0, 0, 0)
    return { startMs: start.getTime(), endMs: endNow }
  }

  if (range.preset === 'yesterday') {
    const start = new Date(now)
    start.setHours(0, 0, 0, 0)
    start.setDate(start.getDate() - 1)
    const end = new Date(start)
    end.setDate(end.getDate() + 1)
    return { startMs: start.getTime(), endMs: end.getTime() }
  }

  // custom day
  const key = range.day || localDateKey(now)
  const [y, m, d] = key.split('-').map(Number)
  const start = new Date(y, (m || 1) - 1, d || 1, 0, 0, 0, 0)
  const end = new Date(start)
  end.setDate(end.getDate() + 1)
  const endMs = Math.min(end.getTime(), endNow)
  return { startMs: start.getTime(), endMs: Math.max(endMs, start.getTime() + 1000) }
}

export function reviewRangeLabel(range: ReviewTimeRange): string {
  switch (range.preset) {
    case '24h':
      return '过去 24 小时'
    case 'today':
      return '今天'
    case 'yesterday':
      return '昨天'
    case '7d':
      return '过去 7 天'
    case 'day': {
      if (!range.day) return '选择日期'
      const parts = range.day.split('-')
      const m = Number(parts[1])
      const d = Number(parts[2])
      return `${m}月${d}日`
    }
    default:
      return '时间段'
  }
}
