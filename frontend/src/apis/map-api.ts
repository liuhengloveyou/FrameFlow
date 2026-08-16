import { http } from '@/lib/http'
import type { MapElement } from '@/types/api'

export async function listMapElements(): Promise<MapElement[]> {
  const { data } = await http.get('/map/elements')
  return data.data
}

export async function saveMapElements(elements: MapElement[]): Promise<MapElement[]> {
  const { data } = await http.put('/map/elements', { elements })
  return data.data
}

export function nextMapElementId(prefix: string) {
  return `${prefix}-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 7)}`
}
