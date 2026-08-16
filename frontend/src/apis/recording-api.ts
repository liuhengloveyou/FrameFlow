import { http } from '@/lib/http'
import type { RecordingPolicy, RecordingSegment } from '@/types/api'

export async function listRecordingPolicies(): Promise<RecordingPolicy[]> {
  const { data } = await http.get('/recording/policies')
  return data.data
}

export async function upsertRecordingPolicy(
  input: Omit<RecordingPolicy, 'updatedAt'>,
): Promise<RecordingPolicy> {
  const { data } = await http.put(`/recording/policies/${input.cameraId}`, input)
  return data.data
}

export type SegmentQuery = {
  cameraId?: string
  date?: string
}

export async function listRecordingSegments(query: SegmentQuery = {}): Promise<RecordingSegment[]> {
  const { data } = await http.get('/recording/segments', { params: query })
  return data.data
}
