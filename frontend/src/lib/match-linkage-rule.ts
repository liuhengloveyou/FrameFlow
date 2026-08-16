import type { AlarmLinkageRule, AlarmRecord } from '@/types/api'

export function ruleMatchesAlarm(rule: AlarmLinkageRule, alarm: AlarmRecord): boolean {
  if (!rule.enabled) return false
  if (rule.levels.length && !rule.levels.includes(alarm.level)) return false
  if (rule.algorithmName && rule.algorithmName !== alarm.algorithmName) return false
  if (rule.cameraGroup && rule.cameraGroup !== (alarm.cameraGroup ?? '')) return false
  return true
}

/** 按优先级取第一条匹配规则 */
export function findMatchingRule(
  rules: AlarmLinkageRule[],
  alarm: AlarmRecord,
): AlarmLinkageRule | null {
  const sorted = [...rules].sort((a, b) => a.priority - b.priority || a.name.localeCompare(b.name))
  return sorted.find((r) => ruleMatchesAlarm(r, alarm)) ?? null
}
