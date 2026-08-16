import { http } from '@/lib/http'
import type { NotifyChannel, NotifyDelivery, NotifyDeliveryStatus, PageResult } from '@/types/api'

export async function listNotifyChannels(): Promise<NotifyChannel[]> {
  const { data } = await http.get('/notifications/channels')
  return data.data
}

export type NotifyChannelInput = {
  name: string
  kind: NotifyChannel['kind']
  enabled: boolean
  endpoint: string
}

export async function createNotifyChannel(input: NotifyChannelInput): Promise<NotifyChannel> {
  const { data } = await http.post('/notifications/channels', input)
  return data.data
}

export async function updateNotifyChannel(
  id: string,
  input: Partial<NotifyChannelInput>,
): Promise<NotifyChannel> {
  const { data } = await http.patch(`/notifications/channels/${id}`, input)
  return data.data
}

export async function deleteNotifyChannel(id: string): Promise<void> {
  await http.delete(`/notifications/channels/${id}`)
}

export type DeliveryQuery = {
  page?: number
  pageSize?: number
  status?: NotifyDeliveryStatus | ''
  q?: string
}

export async function listNotifyDeliveries(
  query: DeliveryQuery = {},
): Promise<PageResult<NotifyDelivery>> {
  const { data } = await http.get('/notifications/deliveries', { params: query })
  return data.data
}
