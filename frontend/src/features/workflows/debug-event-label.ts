import type { DebugEventKind } from '@/types/api'

export const debugEventLabel: Record<DebugEventKind, string> = {
  workflow_start: '工作流开始',
  workflow_complete: '工作流完成',
  node_start: '节点开始',
  node_complete: '节点完成',
  node_error: '节点异常',
  node_output: '节点输出',
}
