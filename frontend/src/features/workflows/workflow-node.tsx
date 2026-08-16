import { memo } from 'react'
import { Handle, Position, type Node, type NodeProps } from '@xyflow/react'
import type { WorkflowNodeData } from '@/types/api'

export type WorkflowFlowNode = Node<WorkflowNodeData, 'workflow'>

function WorkflowNodeView({ data }: NodeProps<WorkflowFlowNode>) {
  const state = data.runState ?? 'idle'
  return (
    <div
      className="ff-wf-node"
      data-state={state}
      data-invalid={data.configInvalid ? 'true' : undefined}
    >
      <Handle type="target" position={Position.Left} />
      <div className="ff-wf-node-bar" />
      <div className="ff-wf-node-title">{data.label}</div>
      <div className="ff-wf-node-type">{data.type}</div>
      {data.configInvalid ? <div className="ff-wf-node-hint">配置待完善</div> : null}
      <Handle type="source" position={Position.Right} />
    </div>
  )
}

export const workflowNodeTypes = {
  workflow: memo(WorkflowNodeView),
}
