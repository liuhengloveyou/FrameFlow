import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { Button } from '@astryxdesign/core/Button'
import { Card } from '@astryxdesign/core/Card'
import { TextInput } from '@astryxdesign/core/TextInput'
import { Selector } from '@astryxdesign/core/Selector'
import { MoreMenu } from '@astryxdesign/core/MoreMenu'
import { Dialog, DialogHeader } from '@astryxdesign/core/Dialog'
import { Layout } from '@astryxdesign/core/Layout'
import { HStack } from '@astryxdesign/core/HStack'
import { VStack } from '@astryxdesign/core/VStack'
import { useToast } from '@astryxdesign/core/Toast'
import { PageLayout } from '@/components/page-layout'
import { WorkflowStatusBadge } from '@/components/status-badge'
import { AlarmLinkageSubnav } from '@/features/alarms/alarm-linkage-subnav'
import {
  createWorkflow,
  deleteWorkflow,
  listWorkflows,
  startWorkflow,
  stopWorkflow,
  updateWorkflow,
} from '@/apis/workflows-api'
import { getErrorMessage } from '@/lib/http'
import type { WorkflowStatus, WorkflowSummary } from '@/types/api'

export function WorkflowsPage() {
  const toast = useToast()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [createOpen, setCreateOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<WorkflowSummary | null>(null)
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [statusFilter, setStatusFilter] = useState('')

  const query = useQuery({ queryKey: ['workflows'], queryFn: listWorkflows })

  const filtered = useMemo(() => {
    const list = query.data ?? []
    if (!statusFilter) return list
    return list.filter((w) => w.status === statusFilter)
  }, [query.data, statusFilter])

  const createMutation = useMutation({
    mutationFn: () => createWorkflow({ name, description }),
    onSuccess: async (wf) => {
      toast({ body: '工作流已创建，进入编排', type: 'info' })
      setCreateOpen(false)
      setName('')
      setDescription('')
      await queryClient.invalidateQueries({ queryKey: ['workflows'] })
      void navigate({ to: '/workflows/$id', params: { id: wf.id } })
    },
    onError: (error) => toast({ body: getErrorMessage(error), type: 'error' }),
  })

  const editMutation = useMutation({
    mutationFn: () =>
      updateWorkflow(editTarget!.id, { name, description }),
    onSuccess: async () => {
      toast({ body: '工作流已更新', type: 'info' })
      setEditTarget(null)
      await queryClient.invalidateQueries({ queryKey: ['workflows'] })
    },
    onError: (error) => toast({ body: getErrorMessage(error), type: 'error' }),
  })

  const actionMutation = useMutation({
    mutationFn: async ({
      id,
      action,
    }: {
      id: string
      action: 'start' | 'stop' | 'delete'
    }) => {
      if (action === 'start') return startWorkflow(id)
      if (action === 'stop') return stopWorkflow(id)
      return deleteWorkflow(id)
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['workflows'] })
    },
    onError: (error) => toast({ body: getErrorMessage(error), type: 'error' }),
  })

  function openEdit(wf: WorkflowSummary) {
    setEditTarget(wf)
    setName(wf.name)
    setDescription(wf.description)
  }

  function openCreate() {
    setCreateOpen(true)
    setName('')
    setDescription('')
  }

  return (
    <PageLayout
        title="工作流"
        description="编排处置流程；由联动规则或手工绑定触发"
        actions={
          <HStack gap={2}>
            <Button
              label="刷新"
              variant="secondary"
              clickAction={async () => {
                await query.refetch()
              }}
            />
            <Button label="新建工作流" variant="primary" onClick={openCreate} />
          </HStack>
        }
        toolbar={<AlarmLinkageSubnav />}
      >

      <div className="ff-toolbar">
        <Selector
          label="运行状态"
          isLabelHidden
          placeholder="全部状态"
          hasClear
          value={statusFilter}
          onChange={(v) => setStatusFilter(v ?? '')}
          options={[
            { label: '运行中', value: 'running' },
            { label: '已停止', value: 'stopped' },
            { label: '空闲', value: 'idle' },
            { label: '异常', value: 'error' },
          ] satisfies Array<{ label: string; value: WorkflowStatus }>}
        />
      </div>

      <div className="ff-model-grid">
        {filtered.map((wf) => (
          <Card key={wf.id} padding={4}>
            <VStack gap={3}>
              <div>
                <strong>{wf.name}</strong>
                <p className="ff-page-desc">{wf.description}</p>
              </div>
              <WorkflowStatusBadge status={wf.status} />
              <div>
                <p className="ff-page-desc">
                  节点 {wf.nodeCount} · 连线 {wf.edgeCount} · 调用 {wf.callCount} · 成功{' '}
                  {wf.successCount} · 成功率 {(wf.successRate * 100).toFixed(1)}%
                </p>
                <div className="ff-success-bar">
                  <span style={{ width: `${Math.min(100, wf.successRate * 100)}%` }} />
                </div>
              </div>
              <p className="ff-page-desc">
                最近执行 {wf.lastRunAt ? new Date(wf.lastRunAt).toLocaleString() : '-'} · 更新{' '}
                {new Date(wf.updatedAt).toLocaleString()}
              </p>
              <HStack gap={1} vAlign="center">
                <Link to="/workflows/$id" params={{ id: wf.id }}>
                  <Button label="编排" size="sm" variant="primary" />
                </Link>
                <Button
                  label={wf.status === 'running' ? '停止' : '启动'}
                  size="sm"
                  variant="secondary"
                  isLoading={actionMutation.isPending}
                  clickAction={async () => {
                    await actionMutation.mutateAsync({
                      id: wf.id,
                      action: wf.status === 'running' ? 'stop' : 'start',
                    })
                  }}
                />
                <MoreMenu
                  size="sm"
                  label={`${wf.name} 更多操作`}
                  items={[
                    { label: '编辑信息', onClick: () => openEdit(wf) },
                    {
                      label: '进入编排',
                      onClick: () => {
                        void navigate({ to: '/workflows/$id', params: { id: wf.id } })
                      },
                    },
                    { type: 'divider' },
                    {
                      label: '删除',
                      onClick: () => {
                        void actionMutation.mutateAsync({ id: wf.id, action: 'delete' })
                      },
                    },
                  ]}
                />
              </HStack>
            </VStack>
          </Card>
        ))}
      </div>
      {filtered.length === 0 ? <div className="ff-placeholder">无匹配工作流</div> : null}

      <Dialog
        isOpen={createOpen}
        onOpenChange={setCreateOpen}
        purpose="form"
        width={440}
        padding={6}
      >
        <Layout
          header={<DialogHeader title="新建工作流" onOpenChange={setCreateOpen} />}
          content={
            <div className="ff-dialog-body">
              <VStack gap={4}>
                <p className="ff-page-desc" style={{ margin: 0 }}>
                  创建后进入画布编排节点；默认已带「报警触发 → 通知 → 保存」模板，启动后可接报警联动。
                </p>
                <TextInput
                  label="名称"
                  placeholder="例如：报警联动-通知"
                  value={name}
                  onChange={setName}
                />
                <TextInput
                  label="描述"
                  placeholder="可选，说明用途"
                  value={description}
                  onChange={setDescription}
                />
              </VStack>
            </div>
          }
          footer={
            <div className="ff-dialog-footer">
              <HStack justify="end" gap={2}>
                <Button label="取消" variant="secondary" onClick={() => setCreateOpen(false)} />
                <Button
                  label="创建"
                  variant="primary"
                  isLoading={createMutation.isPending}
                  clickAction={async () => {
                    await createMutation.mutateAsync()
                  }}
                />
              </HStack>
            </div>
          }
        />
      </Dialog>

      <Dialog
        isOpen={Boolean(editTarget)}
        onOpenChange={(open) => {
          if (!open) setEditTarget(null)
        }}
        purpose="form"
        width={440}
        padding={6}
      >
        <Layout
          header={
            <DialogHeader
              title="编辑工作流"
              onOpenChange={(open) => {
                if (!open) setEditTarget(null)
              }}
            />
          }
          content={
            <div className="ff-dialog-body">
              <VStack gap={4}>
                <TextInput
                  label="名称"
                  placeholder="工作流名称"
                  value={name}
                  onChange={setName}
                />
                <TextInput
                  label="描述"
                  placeholder="可选，说明用途"
                  value={description}
                  onChange={setDescription}
                />
              </VStack>
            </div>
          }
          footer={
            <div className="ff-dialog-footer">
              <HStack justify="end" gap={2}>
                <Button label="取消" variant="secondary" onClick={() => setEditTarget(null)} />
                <Button
                  label="保存"
                  variant="primary"
                  isLoading={editMutation.isPending}
                  clickAction={async () => {
                    await editMutation.mutateAsync()
                  }}
                />
              </HStack>
            </div>
          }
        />
      </Dialog>
    </PageLayout>
  )
}
