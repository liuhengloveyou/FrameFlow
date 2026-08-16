/** 相对时间（Frigate 式：几分钟前 / 几小时前） */
export function formatRelativeTime(iso: string, now = Date.now()): string {
  const t = new Date(iso).getTime()
  if (Number.isNaN(t)) return '—'
  const sec = Math.round((now - t) / 1000)
  if (sec < 45) return '刚刚'
  const min = Math.round(sec / 60)
  if (min < 60) return `${min} 分钟前`
  const hr = Math.round(min / 60)
  if (hr < 24) return `${hr} 小时前`
  const day = Math.round(hr / 24)
  if (day === 1) return '昨天'
  if (day < 7) return `${day} 天前`
  return new Date(iso).toLocaleDateString()
}

export function formatAbsoluteShort(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return d.toLocaleString([], {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  })
}

export function hourOf(iso: string) {
  return new Date(iso).getHours()
}
