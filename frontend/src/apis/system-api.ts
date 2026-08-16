import { http } from '@/lib/http'

export type SystemService = {
  name: string
  status: 'up' | 'degraded' | 'down'
}

export type SystemMetric = {
  label: string
  value: string
  hint?: string
  /** 0-100，用于进度条；无则不显示条 */
  percent?: number
}

export type SystemAlert = {
  id: string
  level: 'info' | 'warn'
  body: string
}

export type SystemAccelerator = {
  id: string
  name: string
  kind: 'GPU' | 'NPU' | 'CPU'
  status: 'ready' | 'busy' | 'missing' | 'error'
  util: string
  memory?: string
  temperature?: string
  driver?: string
}

export type SystemSnapshot = {
  version: string
  uptime: string
  services: SystemService[]
  metrics: SystemMetric[]
  accelerators: SystemAccelerator[]
  alerts: SystemAlert[]
  /** 快照生成时间（ISO） */
  sampledAt: string
}

export async function getSystemSnapshot(): Promise<SystemSnapshot> {
  const { data } = await http.get('/system/snapshot')
  return data.data
}
