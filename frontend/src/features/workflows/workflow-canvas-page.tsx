import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  addEdge,
  useEdgesState,
  useNodesState,
  type Connection,
  type Edge,
  type NodeMouseHandler,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate, useParams } from '@tanstack/react-router'
import { Button } from '@astryxdesign/core/Button'
import { TextInput } from '@astryxdesign/core/TextInput'
import { TextArea } from '@astryxdesign/core/TextArea'
import { Text } from '@astryxdesign/core/Text'
import { Selector } from '@astryxdesign/core/Selector'
import { Dialog, DialogHeader } from '@astryxdesign/core/Dialog'
import { Layout } from '@astryxdesign/core/Layout'
import { HStack } from '@astryxdesign/core/HStack'
import { VStack } from '@astryxdesign/core/VStack'
import { useToast } from '@astryxdesign/core/Toast'
import { PageLayout } from '@/components/page-layout'
import { SidePanel } from '@/components/side-panel'
import { listCameras, getCamera } from '@/apis/devices-api'
import {
  applyAndRestartWorkflow,
  debugWorkflow,
  getWorkflow,
  listNodeSpecs,
  subscribeDebugStream,
  updateWorkflow,
  type ApplyPhase,
} from '@/apis/workflows-api'
import { getErrorMessage } from '@/lib/http'
import {
  firstWorkflowError,
  hasWorkflowErrors,
  validateNodeFields,
  validateWorkflowCanvas,
} from '@/lib/validate-workflow'
import type { Camera, DebugEvent, NodeSpec } from '@/types/api'
import { debugEventLabel } from './debug-event-label'
import { NodeConfigFields } from './node-config-fields'
import { workflowNodeTypes, type WorkflowFlowNode } from './workflow-node'

type WFNode = WorkflowFlowNode

export function WorkflowCanvasPage() {
  const { id } = useParams({ from: '/_authenticated/workflows/$id' })
  const navigate = useNavigate()
  const toast = useToast()
  const queryClient = useQueryClient()
  const streamRef = useRef<HTMLDivElement>(null)

  const workflowQuery = useQuery({
    queryKey: ['workflow', id],
    queryFn: () => getWorkflow(id),
  })
  const specsQuery = useQuery({ queryKey: ['node-specs'], queryFn: listNodeSpecs })
  const camerasQuery = useQuery({
    queryKey: ['cameras', { all: true }],
    queryFn: () => listCameras({ pageSize: 200 }),
  })

  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [nodes, setNodes, onNodesChange] = useNodesState<WFNode>([])
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([])
  const [dirty, setDirty] = useState(false)
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [debugOpen, setDebugOpen] = useState(false)
  const [exitOpen, setExitOpen] = useState(false)
  const [inputMode, setInputMode] = useState<'latest_alarm' | 'custom'>('latest_alarm')
  const [returnMode, setReturnMode] = useState<'sync' | 'stream'>('stream')
  const [customPairs, setCustomPairs] = useState([{ key: 'alarm_id', value: 'a-1' }])
  const [events, setEvents] = useState<DebugEvent[]>([])
  const [busy, setBusy] = useState(false)
  const [applyPhase, setApplyPhase] = useState<ApplyPhase | null>(null)
  const [validationTouched, setValidationTouched] = useState(false)
  const unsubRef = useRef<(() => void) | null>(null)

  const applyPhaseLabel: Record<Exclude<ApplyPhase, null>, string> = {
    saving: '保存中…',
    stopping: '停止中…',
    starting: '启动中…',
  }

  const specsByType = useMemo(() => {
    const map = new Map<string, NodeSpec>()
    for (const s of specsQuery.data ?? []) map.set(s.type, s)
    return map
  }, [specsQuery.data])

  const specsByCategory = useMemo(() => {
    const groups = new Map<string, NodeSpec[]>()
    for (const s of specsQuery.data ?? []) {
      const list = groups.get(s.category) ?? []
      list.push(s)
      groups.set(s.category, list)
    }
    return Array.from(groups.entries())
  }, [specsQuery.data])

  useEffect(() => {
    const wf = workflowQuery.data
    if (!wf) return
    setName(wf.name)
    setDescription(wf.description)
    setNodes(
      wf.nodes.map((n) => ({
        id: n.id,
        type: 'workflow' as const,
        position: n.position,
        data: { ...n.data, runState: 'idle' as const },
      })),
    )
    setEdges(wf.edges.map((e) => ({ ...e })))
    setDirty(false)
    setValidationTouched(false)
  }, [workflowQuery.data, setNodes, setEdges])

  useEffect(() => {
    return () => {
      unsubRef.current?.()
    }
  }, [])

  useEffect(() => {
    if (streamRef.current) {
      streamRef.current.scrollTop = streamRef.current.scrollHeight
    }
  }, [events])

  const selected = nodes.find((n) => n.id === selectedId) ?? null
  const selectedSpec = selected ? specsByType.get(selected.data.type) : undefined

  const flowNodes = useMemo(() => {
    if (!validationTouched) return nodes
    const result = validateWorkflowCanvas({ name, nodes, specsByType })
    const invalidIds = new Set(result.nodes.map((n) => n.nodeId))
    return nodes.map((n) => ({
      ...n,
      data: { ...n.data, configInvalid: invalidIds.has(n.id) },
    }))
  }, [nodes, validationTouched, name, specsByType])

  const selectedFieldErrors = useMemo(() => {
    if (!validationTouched || !selected || !selectedSpec) return undefined
    const fields = validateNodeFields(selectedSpec.inputs, selected.data.config)
    if (!selected.data.label?.trim()) fields.__label = '节点名称不能为空'
    return fields
  }, [validationTouched, selected, selectedSpec])

  const nameError =
    validationTouched && !name.trim() ? '工作流名称不能为空' : undefined

  function assertCanvasValid(requireTrigger: boolean): boolean {
    const result = validateWorkflowCanvas({
      name,
      nodes,
      specsByType,
      requireTrigger,
    })
    setValidationTouched(true)
    if (!hasWorkflowErrors(result)) return true
    toast({ body: firstWorkflowError(result), type: 'error' })
    if (result.nodes[0]) {
      setDebugOpen(false)
      setSelectedId(result.nodes[0].nodeId)
    }
    return false
  }

  const onConnect = useCallback(
    (connection: Connection) => {
      setEdges((eds) => addEdge(connection, eds))
      setDirty(true)
    },
    [setEdges],
  )

  const onNodeClick: NodeMouseHandler<WFNode> = useCallback((_, node) => {
    setDebugOpen(false)
    setSelectedId(node.id)
  }, [])

  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      if (!selectedId) return
      if (e.key === 'Delete' || e.key === 'Backspace') {
        const tag = (e.target as HTMLElement)?.tagName
        if (tag === 'INPUT' || tag === 'TEXTAREA') return
        setNodes((ns) => ns.filter((n) => n.id !== selectedId))
        setEdges((es) => es.filter((ed) => ed.source !== selectedId && ed.target !== selectedId))
        setSelectedId(null)
        setDirty(true)
        return
      }
      const step = e.shiftKey ? 20 : 8
      const delta =
        e.key === 'ArrowUp'
          ? { x: 0, y: -step }
          : e.key === 'ArrowDown'
            ? { x: 0, y: step }
            : e.key === 'ArrowLeft'
              ? { x: -step, y: 0 }
              : e.key === 'ArrowRight'
                ? { x: step, y: 0 }
                : null
      if (!delta) return
      e.preventDefault()
      setNodes((ns) =>
        ns.map((n) =>
          n.id === selectedId
            ? { ...n, position: { x: n.position.x + delta.x, y: n.position.y + delta.y } }
            : n,
        ),
      )
      setDirty(true)
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [selectedId, setNodes, setEdges])

  function addNodeFromSpec(spec: NodeSpec) {
    if (spec.isTrigger && nodes.some((n) => specsByType.get(n.data.type)?.isTrigger)) {
      toast({ body: '触发节点唯一，不能添加多个入口节点', type: 'error' })
      return
    }
    const nodeId = `n-${Date.now()}`
    const node: WFNode = {
      id: nodeId,
      type: 'workflow',
      position: { x: 180 + nodes.length * 40, y: 120 + nodes.length * 24 },
      data: {
        type: spec.type,
        label: spec.label,
        description: '',
        config: {},
        runState: 'idle',
      },
    }
    setNodes((ns) => [...ns, node])
    setSelectedId(nodeId)
    setDebugOpen(false)
    setDirty(true)
  }

  function onDragStart(e: React.DragEvent, spec: NodeSpec) {
    e.dataTransfer.setData('application/frameflow-node', spec.type)
    e.dataTransfer.effectAllowed = 'move'
  }

  function onDrop(e: React.DragEvent) {
    e.preventDefault()
    const type = e.dataTransfer.getData('application/frameflow-node')
    const spec = specsByType.get(type)
    if (spec) addNodeFromSpec(spec)
  }

  function buildDetailPayload() {
    return {
      ...(workflowQuery.data!),
      name,
      description,
      nodes: nodes.map((n) => ({
        id: n.id,
        type: n.data.type,
        position: n.position,
        data: {
          type: n.data.type,
          label: n.data.label,
          description: n.data.description,
          config: n.data.config,
        },
      })),
      edges: edges.map((e) => ({ id: e.id, source: e.source, target: e.target })),
    }
  }

  /** 仅落库，不重启（草稿） */
  async function persistDraft() {
    setBusy(true)
    setApplyPhase('saving')
    try {
      const detail = buildDetailPayload()
      await updateWorkflow(id, {
        name: detail.name,
        description: detail.description,
        nodes: detail.nodes,
        edges: detail.edges,
      })
      setDirty(false)
      setValidationTouched(false)
      toast({ body: '已保存草稿（未重启）', type: 'info' })
      await queryClient.invalidateQueries({ queryKey: ['workflow', id] })
      await queryClient.invalidateQueries({ queryKey: ['workflows'] })
    } catch (error) {
      toast({ body: getErrorMessage(error), type: 'error' })
      throw error
    } finally {
      setBusy(false)
      setApplyPhase(null)
    }
  }

  /** 保存 → 停止 → 启动（与 README 应用语义一致） */
  async function persistAndApply() {
    setBusy(true)
    try {
      await applyAndRestartWorkflow(id, buildDetailPayload(), setApplyPhase)
      setDirty(false)
      setValidationTouched(false)
      toast({ body: '已保存并应用（停止 → 启动）', type: 'info' })
      await queryClient.invalidateQueries({ queryKey: ['workflow', id] })
      await queryClient.invalidateQueries({ queryKey: ['workflows'] })
    } catch (error) {
      toast({ body: getErrorMessage(error), type: 'error' })
      throw error
    } finally {
      setBusy(false)
      setApplyPhase(null)
    }
  }

  function closeDebug() {
    unsubRef.current?.()
    unsubRef.current = null
    setDebugOpen(false)
    setEvents([])
    setNodes((ns) =>
      ns.map((n) => ({
        ...n,
        data: { ...n.data, runState: 'idle' },
      })),
    )
  }

  function openDebug() {
    setSelectedId(null)
    setDebugOpen(true)
  }

  function applyRunState(nodeId: string | undefined, state: 'idle' | 'running' | 'done' | 'error') {
    if (!nodeId) return
    setNodes((ns) =>
      ns.map((n) =>
        n.id === nodeId ? { ...n, data: { ...n.data, runState: state } } : n,
      ),
    )
  }

  async function runDebug() {
    if (!assertCanvasValid(true)) return
    setValidationTouched(false)
    setBusy(true)
    try {
      const customInput = Object.fromEntries(
        customPairs.filter((p) => p.key).map((p) => [p.key, p.value]),
      )
      const res = await debugWorkflow(id, { inputMode, customInput, returnMode })
      setEvents([])
      if (returnMode === 'sync') {
        for (const ev of res.events ?? []) {
          setEvents((prev) => [...prev, ev])
          if (ev.kind === 'node_start') applyRunState(ev.nodeId, 'running')
          if (ev.kind === 'node_complete' || ev.kind === 'node_output')
            applyRunState(ev.nodeId, 'done')
          if (ev.kind === 'node_error') applyRunState(ev.nodeId, 'error')
        }
      } else {
        unsubRef.current?.()
        unsubRef.current = subscribeDebugStream(
          res.run_id,
          (ev) => {
            setEvents((prev) => [...prev, ev])
            if (ev.kind === 'node_start') applyRunState(ev.nodeId, 'running')
            if (ev.kind === 'node_complete' || ev.kind === 'node_output')
              applyRunState(ev.nodeId, 'done')
            if (ev.kind === 'node_error') applyRunState(ev.nodeId, 'error')
          },
          () => setBusy(false),
        )
        return
      }
    } catch (error) {
      toast({ body: getErrorMessage(error), type: 'error' })
    } finally {
      if (returnMode === 'sync') setBusy(false)
    }
  }

  function requestExit() {
    if (!dirty) {
      void navigate({ to: '/workflows' })
      return
    }
    setExitOpen(true)
  }

  const status = workflowQuery.data?.status
  const cameras = camerasQuery.data?.items ?? []
  const showNodeDrawer = Boolean(selected && selectedSpec && !debugOpen)

  return (
    <PageLayout
        title={name || '工作流画布'}
        description="顶栏拖入节点 · 连线编排 · 调试"
        contentPadding={3}
        actions={
          <HStack gap={2} vAlign="center" className="ff-workflow-header-actions">
            {applyPhase ? (
              <Text type="supporting" color="secondary">
                {applyPhaseLabel[applyPhase]}
              </Text>
            ) : null}
            <Button
              label={debugOpen ? '关闭调试' : '调试'}
              size="sm"
              variant="primary"
              onClick={() => (debugOpen ? closeDebug() : openDebug())}
            />
            <Button
              label="退出"
              size="sm"
              variant="ghost"
              isDisabled={busy}
              clickAction={async () => {
                requestExit()
              }}
            />
            <Button
              label="仅保存草稿"
              size="sm"
              variant="ghost"
              isDisabled={busy}
              clickAction={async () => {
                if (!assertCanvasValid(false)) return
                await persistDraft()
              }}
            />
            <Button
              label="保存并应用"
              size="sm"
              variant="secondary"
              isLoading={busy}
              clickAction={async () => {
                if (!assertCanvasValid(true)) return
                await persistAndApply()
              }}
            />
            <Button
              label="保存并退出"
              size="sm"
              variant="primary"
              isLoading={busy}
              clickAction={async () => {
                if (!assertCanvasValid(true)) return
                await persistAndApply()
                void navigate({ to: '/workflows' })
              }}
            />
          </HStack>
        }
      >

      <div className="ff-workflow-meta">
        <HStack gap={3} vAlign="center" className="ff-workflow-meta-row">
          <Text type="label" className="ff-workflow-meta-label">
            名称
          </Text>
          <div className="ff-workflow-meta-name">
            <TextInput
              label="名称"
              isLabelHidden
              placeholder="工作流名称"
              value={name}
              status={nameError ? { type: 'error', message: nameError } : undefined}
              onChange={(v) => {
                setName(v)
                setDirty(true)
              }}
            />
          </div>
          <Text type="label" className="ff-workflow-meta-label">
            描述
          </Text>
          <div className="ff-workflow-meta-desc">
            <TextInput
              label="描述"
              isLabelHidden
              placeholder="可选说明"
              value={description}
              onChange={(v) => {
                setDescription(v)
                setDirty(true)
              }}
            />
          </div>
        </HStack>
      </div>

      <div className="ff-workflow-layout">
        <aside className="ff-node-palette">
          {specsByCategory.map(([category, specs]) => (
            <div key={category} className="ff-palette-group">
              <span className="ff-palette-group-title">{category}</span>
              {specs.map((spec) => (
                <button
                  key={spec.type}
                  type="button"
                  className="ff-node-palette-item"
                  draggable
                  onDragStart={(e) => onDragStart(e, spec)}
                  onClick={() => addNodeFromSpec(spec)}
                  title={spec.description}
                >
                  {spec.label}
                </button>
              ))}
            </div>
          ))}
        </aside>

        <div
          className="ff-canvas-wrap"
          onDragOver={(e) => e.preventDefault()}
          onDrop={onDrop}
        >
          <ReactFlow
            nodes={flowNodes}
            edges={edges}
            nodeTypes={workflowNodeTypes}
            onNodesChange={(changes) => {
              onNodesChange(changes)
              setDirty(true)
            }}
            onEdgesChange={(changes) => {
              onEdgesChange(changes)
              setDirty(true)
            }}
            onConnect={onConnect}
            onNodeClick={onNodeClick}
            fitView
            fitViewOptions={{ padding: 0.2 }}
            minZoom={0.25}
            maxZoom={1.5}
            proOptions={{ hideAttribution: true }}
            style={{ width: '100%', height: '100%' }}
          >
            <Background />
            <Controls position="bottom-left" />
            <MiniMap position="bottom-right" pannable zoomable />
          </ReactFlow>
        </div>
      </div>

      <SidePanel
        open={showNodeDrawer}
        title={`节点编辑 · ${selectedSpec?.label ?? ''}`}
        onClose={() => setSelectedId(null)}
        footer={
          <Button
            label={applyPhase ? applyPhaseLabel[applyPhase] : '保存并应用'}
            variant="primary"
            isLoading={busy}
            clickAction={async () => {
              if (!assertCanvasValid(true)) return
              await persistAndApply()
            }}
          />
        }
      >
        {selected && selectedSpec ? (
          <VStack gap={3}>
            <TextInput
              label="节点名称"
              value={selected.data.label}
              status={
                selectedFieldErrors?.__label
                  ? { type: 'error', message: selectedFieldErrors.__label }
                  : undefined
              }
              onChange={(v) => {
                setNodes((ns) =>
                  ns.map((n) =>
                    n.id === selected.id ? { ...n, data: { ...n.data, label: v } } : n,
                  ),
                )
                setDirty(true)
              }}
            />
            <TextArea
              label="节点描述"
              value={selected.data.description}
              onChange={(v) => {
                setNodes((ns) =>
                  ns.map((n) =>
                    n.id === selected.id
                      ? { ...n, data: { ...n.data, description: v } }
                      : n,
                  ),
                )
                setDirty(true)
              }}
            />
            <div>
              <p className="ff-page-desc">输入参数</p>
              <NodeConfigFields
                fields={selectedSpec.inputs}
                values={selected.data.config}
                cameras={cameras}
                errors={selectedFieldErrors}
                onChange={(key, value) => {
                  setNodes((ns) =>
                    ns.map((n) =>
                      n.id === selected.id
                        ? {
                            ...n,
                            data: {
                              ...n.data,
                              config: { ...n.data.config, [key]: value },
                            },
                          }
                        : n,
                    ),
                  )
                  setDirty(true)
                }}
                onCameraSelect={async (camera: Camera) => {
                  const full = await getCamera(camera.id)
                  setNodes((ns) =>
                    ns.map((n) =>
                      n.id === selected.id
                        ? {
                            ...n,
                            data: {
                              ...n.data,
                              config: { ...n.data.config, camera_info: full },
                            },
                          }
                        : n,
                    ),
                  )
                  setDirty(true)
                }}
              />
            </div>
            <div>
              <p className="ff-page-desc">输出说明</p>
              <ul className="ff-page-desc">
                {selectedSpec.outputs.map((o) => (
                  <li key={o.key}>
                    {o.label} ({o.type})
                  </li>
                ))}
                {selectedSpec.outputs.length === 0 ? <li>无</li> : null}
              </ul>
            </div>
          </VStack>
        ) : null}
      </SidePanel>

      <SidePanel
        open={debugOpen}
        title="工作流调试"
        onClose={closeDebug}
        width="min(460px, 100vw)"
      >
        <VStack gap={3}>
          <Selector
            label="输入来源"
            value={inputMode}
            onChange={(v) => setInputMode((v as typeof inputMode) || 'latest_alarm')}
            options={[
              { label: '自动读取最新报警记录', value: 'latest_alarm' },
              { label: '自定义输入', value: 'custom' },
            ]}
          />
          {inputMode === 'custom'
            ? customPairs.map((pair, idx) => (
                <HStack key={idx} gap={2}>
                  <TextInput
                    label="键"
                    value={pair.key}
                    onChange={(v) =>
                      setCustomPairs((rows) =>
                        rows.map((r, i) => (i === idx ? { ...r, key: v } : r)),
                      )
                    }
                  />
                  <TextInput
                    label="值"
                    value={pair.value}
                    onChange={(v) =>
                      setCustomPairs((rows) =>
                        rows.map((r, i) => (i === idx ? { ...r, value: v } : r)),
                      )
                    }
                  />
                </HStack>
              ))
            : null}
          {inputMode === 'custom' ? (
            <Button
              label="添加键值"
              size="sm"
              variant="ghost"
              onClick={() => setCustomPairs((rows) => [...rows, { key: '', value: '' }])}
            />
          ) : null}
          <Selector
            label="返回模式"
            value={returnMode}
            onChange={(v) => setReturnMode((v as typeof returnMode) || 'stream')}
            options={[
              { label: '一次性返回（同步）', value: 'sync' },
              { label: '实时流返回（SSE）', value: 'stream' },
            ]}
          />
          <Button label="开始调试" variant="primary" isLoading={busy} clickAction={runDebug} />
          {status !== 'running' ? (
            <Text type="supporting" color="secondary">
              提示：调试不要求启动；报警联动仍需工作流为「运行中」。
            </Text>
          ) : null}
          <div className="ff-debug-stream" ref={streamRef}>
            {events.map((ev) => (
              <div key={ev.id} className="ff-debug-event" data-kind={ev.kind}>
                <strong>{debugEventLabel[ev.kind] ?? ev.kind}</strong>
                <div>{ev.message}</div>
                {ev.payload != null && ev.kind === 'node_output' ? (
                  <pre className="ff-debug-payload">
                    {JSON.stringify(ev.payload, null, 2)}
                  </pre>
                ) : null}
                <div className="ff-page-desc">{new Date(ev.at).toLocaleTimeString()}</div>
              </div>
            ))}
          </div>
        </VStack>
      </SidePanel>

      <Dialog isOpen={exitOpen} onOpenChange={setExitOpen} purpose="required" width={440}>
        <Layout
          header={<DialogHeader title="未保存的变更" />}
          content={<p className="ff-page-desc">当前画布有未保存修改，请选择如何离开。</p>}
          footer={
            <HStack justify="end" gap={2}>
              <Button label="取消" variant="ghost" onClick={() => setExitOpen(false)} />
              <Button
                label="直接退出"
                variant="secondary"
                onClick={() => {
                  setExitOpen(false)
                  void navigate({ to: '/workflows' })
                }}
              />
              <Button
                label="保存并退出"
                variant="primary"
                isLoading={busy}
                clickAction={async () => {
                  if (!assertCanvasValid(true)) return
                  await persistAndApply()
                  setExitOpen(false)
                  void navigate({ to: '/workflows' })
                }}
              />
            </HStack>
          }
        />
      </Dialog>
    </PageLayout>
  )
}
