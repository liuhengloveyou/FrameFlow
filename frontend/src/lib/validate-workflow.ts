import type { Camera, NodeFieldSpec, NodeSpec, WorkflowNodeData } from '@/types/api'

export type NodeFieldErrors = Record<string, string>

export type WorkflowNodeIssue = {
  nodeId: string
  label: string
  fields: NodeFieldErrors
  message: string
}

export type WorkflowValidationResult = {
  name?: string
  graph?: string
  nodes: WorkflowNodeIssue[]
}

function isBlank(value: unknown): boolean {
  return value === undefined || value === null || value === ''
}

export function validateNodeFields(
  fields: NodeFieldSpec[],
  config: Record<string, unknown>,
): NodeFieldErrors {
  const errors: NodeFieldErrors = {}

  for (const field of fields) {
    const raw = config[field.key]

    if (field.type === 'boolean') continue

    if (field.type === 'camera_info') {
      const cam = raw as Camera | undefined
      if (field.required && (!cam || !cam.id)) {
        errors[field.key] = `${field.label}不能为空`
      }
      continue
    }

    if (field.type === 'json') {
      if (typeof raw === 'string') {
        try {
          JSON.parse(raw)
        } catch {
          errors[field.key] = `${field.label}不是合法 JSON`
          continue
        }
      }
      if (field.required && (isBlank(raw) || (typeof raw === 'string' && !raw.trim()))) {
        errors[field.key] = `${field.label}不能为空`
      }
      continue
    }

    if (field.required && isBlank(raw)) {
      errors[field.key] = `${field.label}不能为空`
      continue
    }

    if (isBlank(raw)) continue

    if (field.type === 'number') {
      const n = typeof raw === 'number' ? raw : Number(raw)
      if (Number.isNaN(n)) {
        errors[field.key] = `${field.label}须为有效数字`
      } else if (field.min != null && n < field.min) {
        errors[field.key] = `${field.label}不能小于 ${field.min}`
      } else if (field.max != null && n > field.max) {
        errors[field.key] = `${field.label}不能大于 ${field.max}`
      }
      continue
    }

    if (field.type === 'select' && field.options?.length) {
      const ok = field.options.some((o) => o.value === String(raw))
      if (!ok) errors[field.key] = `${field.label}选项无效`
    }
  }

  return errors
}

export function validateWorkflowNodes(
  nodes: Array<{ id: string; data: WorkflowNodeData }>,
  specsByType: Map<string, NodeSpec>,
): WorkflowNodeIssue[] {
  const issues: WorkflowNodeIssue[] = []

  for (const node of nodes) {
    const fieldErrors: NodeFieldErrors = {}
    if (!node.data.label?.trim()) {
      fieldErrors.__label = '节点名称不能为空'
    }

    const spec = specsByType.get(node.data.type)
    if (!spec) {
      issues.push({
        nodeId: node.id,
        label: node.data.label || node.data.type,
        fields: { __type: '未知节点类型' },
        message: `节点「${node.data.label || node.data.type}」类型无效`,
      })
      continue
    }

    Object.assign(fieldErrors, validateNodeFields(spec.inputs, node.data.config))
    if (Object.keys(fieldErrors).length) {
      const first = Object.values(fieldErrors)[0]
      issues.push({
        nodeId: node.id,
        label: node.data.label || spec.label,
        fields: fieldErrors,
        message: `节点「${node.data.label || spec.label}」：${first}`,
      })
    }
  }

  return issues
}

export function validateWorkflowCanvas(input: {
  name: string
  nodes: Array<{ id: string; data: WorkflowNodeData }>
  specsByType: Map<string, NodeSpec>
  /** 保存并重启 / 调试时要求存在触发节点 */
  requireTrigger?: boolean
}): WorkflowValidationResult {
  const result: WorkflowValidationResult = { nodes: [] }

  if (!input.name.trim()) {
    result.name = '工作流名称不能为空'
  }

  result.nodes = validateWorkflowNodes(input.nodes, input.specsByType)

  if (input.requireTrigger) {
    const hasTrigger = input.nodes.some((n) => input.specsByType.get(n.data.type)?.isTrigger)
    if (!hasTrigger) {
      result.graph = '请添加报警触发节点作为入口'
    }
  }

  return result
}

export function hasWorkflowErrors(result: WorkflowValidationResult): boolean {
  return Boolean(result.name || result.graph || result.nodes.length)
}

export function firstWorkflowError(result: WorkflowValidationResult): string {
  return result.name ?? result.graph ?? result.nodes[0]?.message ?? '请修正配置后再试'
}
