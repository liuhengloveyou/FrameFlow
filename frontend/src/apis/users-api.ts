import { http } from '@/lib/http'
import type { PageResult } from '@/types/api'

export type UserRole = 'admin' | 'operator' | 'viewer'
export type UserStatus = 'active' | 'disabled'

export type UserRow = {
  id: string
  username: string
  displayName: string
  role: UserRole
  status: UserStatus
  lastLoginAt: string
}

export type AuditLog = {
  id: string
  actor: string
  action: string
  target: string
  module: string
  ip: string
  at: string
}

export type UserInput = {
  username: string
  displayName: string
  role: UserRole
  status: UserStatus
}

export async function listUsers(q?: string): Promise<UserRow[]> {
  const { data } = await http.get('/users', { params: { q } })
  return data.data
}

export async function createUser(input: UserInput): Promise<UserRow> {
  const { data } = await http.post('/users', input)
  return data.data
}

export async function updateUser(id: string, input: Partial<UserInput>): Promise<UserRow> {
  const { data } = await http.patch(`/users/${id}`, input)
  return data.data
}

export async function deleteUser(id: string): Promise<void> {
  await http.delete(`/users/${id}`)
}

export type AuditLogQuery = {
  page?: number
  pageSize?: number
  q?: string
  module?: string
  actor?: string
}

export async function listAuditLogs(query: AuditLogQuery = {}): Promise<PageResult<AuditLog>> {
  const { data } = await http.get('/audit-logs', { params: query })
  return data.data
}
