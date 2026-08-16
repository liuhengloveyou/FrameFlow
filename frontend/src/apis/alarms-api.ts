import { http } from '@/lib/http'
import type {
  AlarmLinkageRule,
  AlarmLinkageRuleInput,
  AlarmRecord,
  AlarmStatus,
  PageResult,
} from '@/types/api'

export type AlarmQuery = {
  page?: number
  pageSize?: number
  level?: string
  status?: string
  q?: string
}

export async function listAlarms(query: AlarmQuery = {}): Promise<PageResult<AlarmRecord>> {
  const { data } = await http.get('/alarms', { params: query })
  return data.data
}

export async function updateAlarmStatus(id: number, status: AlarmStatus): Promise<AlarmRecord> {
  const { data } = await http.patch(`/alarms/${id}`, { status })
  return data.data
}

/** 手动配置单条报警联动；传 null 清除；会清除规则来源标记 */
export async function linkAlarmWorkflow(
  id: number,
  workflow: { id: string; name: string } | null,
): Promise<AlarmRecord> {
  const { data } = await http.patch(`/alarms/${id}/workflow`, {
    workflowId: workflow?.id ?? null,
  })
  return data.data
}

export async function listLinkageRules(): Promise<AlarmLinkageRule[]> {
  const { data } = await http.get('/alarms/linkage-rules')
  return data.data
}

export async function createLinkageRule(
  input: AlarmLinkageRuleInput,
): Promise<AlarmLinkageRule> {
  const { data } = await http.post('/alarms/linkage-rules', input)
  return data.data
}

export async function updateLinkageRule(
  id: number,
  input: AlarmLinkageRuleInput,
): Promise<AlarmLinkageRule> {
  const { data } = await http.put(`/alarms/linkage-rules/${id}`, input)
  return data.data
}

export async function deleteLinkageRule(id: number): Promise<void> {
  await http.delete(`/alarms/linkage-rules/${id}`)
}

/**
 * 将启用规则应用到报警。
 * onlyUnlinked=true（默认）：跳过「手工联动」记录（有 workflow 且无 linkageRuleId）；
 * 规则来源记录会被刷新；无匹配时清除规则来源联动。
 */
export async function applyLinkageRules(opts?: {
  onlyUnlinked?: boolean
}): Promise<{ updated: number }> {
  const { data } = await http.post('/alarms/linkage-rules/apply', opts ?? {})
  return data.data
}
