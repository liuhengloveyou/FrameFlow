import { http } from '@/lib/http'
import type {
  PageResult,
  ReviewEvent,
  ReviewEventKind,
  ReviewEventStatus,
  ReviewKindCounts,
} from '@/types/api'

export type ReviewQuery = {
  page?: number
  pageSize?: number
  kind?: ReviewEventKind | ''
  status?: ReviewEventStatus | ''
  q?: string
  cameraId?: string
  /** 多选摄像头；未传 = 全部；传空数组 = 无匹配 */
  cameraIds?: string[]
  /** ISO 时间下界（含） */
  from?: string
  /** ISO 时间上界（含） */
  to?: string
}

export async function listReviewEvents(query: ReviewQuery = {}): Promise<PageResult<ReviewEvent>> {
  const { data } = await http.get('/review/events', { params: query })
  return data.data
}

export async function getReviewKindCounts(
  query: Omit<ReviewQuery, 'kind' | 'page' | 'pageSize'> = {},
): Promise<ReviewKindCounts> {
  const { data } = await http.get('/review/events/counts', { params: query })
  return data.data
}

/** 同摄像头、时间邻近的上下文事件（详情侧栏，分页；page=0 自动定位当前事件） */
export async function listReviewContext(
  id: string,
  opts: { windowMinutes?: number; page?: number; pageSize?: number } = {},
): Promise<PageResult<ReviewEvent>> {
  const { data } = await http.get(`/review/events/${id}/context`, {
    params: {
      windowMinutes: opts.windowMinutes ?? 45,
      page: opts.page ?? 0,
      pageSize: opts.pageSize ?? 15,
    },
  })
  const raw = data.data
  if (Array.isArray(raw)) {
    return {
      items: raw,
      total: raw.length,
      page: 1,
      pageSize: raw.length || 15,
    }
  }
  return raw
}

export async function markReviewed(
  ids: string[],
  status: ReviewEventStatus = 'reviewed',
): Promise<number> {
  const { data } = await http.post('/review/events/mark', { ids, status })
  return data.data
}

export async function getReviewEvent(id: string): Promise<ReviewEvent> {
  const { data } = await http.get(`/review/events/${id}`)
  return data.data
}

/** 将检测提升为警报（体现 Detection ≠ Alert） */
export async function promoteDetectionToAlert(id: string): Promise<ReviewEvent> {
  const { data } = await http.post(`/review/events/${id}/promote`)
  return data.data
}
