import type { AlgorithmRuleField, Point2D } from '@/types/api'

export type AiConfigErrors = {
  fields: Record<string, string>
  regions?: string
}

export function validateAiConfig(
  schema: AlgorithmRuleField[],
  rules: Record<string, unknown>,
  regions: Point2D[][],
  opts?: { requireRegions?: boolean },
): AiConfigErrors {
  const fields: Record<string, string> = {}

  for (const field of schema) {
    const raw = rules[field.key]

    if (field.type === 'boolean') continue

    if (field.required && (raw === undefined || raw === null || raw === '')) {
      fields[field.key] = `${field.label}不能为空`
      continue
    }

    if (field.type === 'number') {
      const n = typeof raw === 'number' ? raw : Number(raw)
      if (raw === '' || raw === undefined || raw === null || Number.isNaN(n)) {
        if (field.required) fields[field.key] = `${field.label}须为有效数字`
        continue
      }
      if (field.min != null && n < field.min) {
        fields[field.key] = `${field.label}不能小于 ${field.min}`
      } else if (field.max != null && n > field.max) {
        fields[field.key] = `${field.label}不能大于 ${field.max}`
      }
    }

    if (field.type === 'select' && field.options?.length && raw != null && raw !== '') {
      const ok = field.options.some((o) => o.value === String(raw))
      if (!ok) fields[field.key] = `${field.label}选项无效`
    }
  }

  let regionsErr: string | undefined
  const validRegions = regions.filter((r) => r.length >= 3)
  // 空区域 = 整帧检测；仅当已画区域时校验顶点
  if (regions.length > 0) {
    if (validRegions.length === 0) {
      regionsErr = '区域至少需要 3 个顶点'
    } else if (validRegions.length < regions.length) {
      regionsErr = '存在无效区域（顶点数不足），请删除或补全'
    }
  } else if (opts?.requireRegions) {
    regionsErr = '请至少绘制一个检测区域'
  }

  return { fields, regions: regionsErr }
}

export function hasAiConfigErrors(errors: AiConfigErrors): boolean {
  return Object.keys(errors.fields).length > 0 || Boolean(errors.regions)
}
