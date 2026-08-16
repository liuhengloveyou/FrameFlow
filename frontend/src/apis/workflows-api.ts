import { http } from '@/lib/http'
import type {
  DebugEvent,
  NodeSpec,
  WorkflowDetail,
  WorkflowSummary,
} from '@/types/api'

/** 工作流走真实 /api/v1 */

export type ApplyPhase = 'saving' | 'stopping' | 'starting' | null

export async function listWorkflows(): Promise<WorkflowSummary[]> {
  const { data } = await http.get('/workflows')
  return data.data
}

export async function getWorkflow(id: string): Promise<WorkflowDetail> {
  const { data } = await http.get(`/workflows/${id}`)
  return data.data
}

export async function createWorkflow(input: {
  name: string
  description: string
}): Promise<WorkflowSummary> {
  const { data } = await http.post('/workflows', input)
  return data.data
}

export async function updateWorkflow(
  id: string,
  patch: Partial<Pick<WorkflowDetail, 'name' | 'description' | 'nodes' | 'edges'>>,
): Promise<WorkflowDetail> {
  const { data } = await http.put(`/workflows/${id}`, patch)
  return data.data
}

export async function deleteWorkflow(id: string): Promise<void> {
  await http.delete(`/workflows/${id}`)
}

export async function startWorkflow(id: string): Promise<WorkflowSummary> {
  const { data } = await http.post(`/workflows/${id}/start`)
  return data.data
}

export async function stopWorkflow(id: string): Promise<WorkflowSummary> {
  const { data } = await http.post(`/workflows/${id}/stop`)
  return data.data
}

export async function listNodeSpecs(): Promise<NodeSpec[]> {
  const { data } = await http.get('/workflows/node-specs')
  return data.data
}

/** 保存图 → 停止 → 启动（应用语义） */
export async function applyAndRestartWorkflow(
  id: string,
  detail: Pick<WorkflowDetail, 'name' | 'description' | 'nodes' | 'edges'>,
  onPhase?: (phase: ApplyPhase) => void,
): Promise<void> {
  onPhase?.('saving')
  await updateWorkflow(id, {
    name: detail.name,
    description: detail.description,
    nodes: detail.nodes,
    edges: detail.edges,
  })
  onPhase?.('stopping')
  try {
    await stopWorkflow(id)
  } catch {
    /* 已停止则忽略 */
  }
  onPhase?.('starting')
  await startWorkflow(id)
}

export async function debugWorkflow(
  id: string,
  body: {
    inputMode: 'latest_alarm' | 'custom'
    customInput?: Record<string, string>
    returnMode: 'sync' | 'stream'
  },
): Promise<{ run_id: string; events?: DebugEvent[] }> {
  const { data } = await http.post(`/workflows/${id}/debug`, body)
  return data.data
}

/** 调试流：回放服务端缓冲的事件 */
export function subscribeDebugStream(
  runId: string,
  onEvent: (event: DebugEvent) => void,
  onDone?: () => void,
): () => void {
  const es = new EventSource(
    `/api/v1/workflows/debug/stream?run_id=${encodeURIComponent(runId)}`,
    { withCredentials: true },
  )
  es.onmessage = (msg) => {
    try {
      onEvent(JSON.parse(msg.data) as DebugEvent)
    } catch {
      /* ignore */
    }
  }
  es.addEventListener('done', () => {
    es.close()
    onDone?.()
  })
  es.onerror = () => {
    es.close()
    onDone?.()
  }
  return () => es.close()
}

export type WorkflowToastPayload = {
  seq?: number
  message?: string
  channel?: string
  at?: string
  workflow?: string
  alarm?: Record<string, unknown>
}

export async function fetchLatestWorkflowToast(): Promise<{
  seq: number
  item: WorkflowToastPayload | null
}> {
  const { data } = await http.get('/workflows/notifications/latest')
  return data.data
}
