import { http } from '@/lib/http'
import type { PageResult, ReviewEventStatus, SearchHit } from '@/types/api'

export type SmartSearchQuery = {
  page?: number
  pageSize?: number
  q?: string
  cameraIds?: string[]
  status?: ReviewEventStatus | ''
  objectClass?: string
  objectColor?: string
  ocr?: string
  from?: string
  to?: string
}

export async function searchEvents(query: SmartSearchQuery = {}): Promise<PageResult<SearchHit>> {
  const { data } = await http.get('/search/events', { params: query })
  return data.data
}
