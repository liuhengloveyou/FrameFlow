import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Button } from '@astryxdesign/core/Button'
import { NumberInput } from '@astryxdesign/core/NumberInput'
import { TextInput } from '@astryxdesign/core/TextInput'
import { Selector } from '@astryxdesign/core/Selector'
import { Switch } from '@astryxdesign/core/Switch'
import { HStack } from '@astryxdesign/core/HStack'
import { VStack } from '@astryxdesign/core/VStack'
import { Text } from '@astryxdesign/core/Text'
import { useToast } from '@astryxdesign/core/Toast'
import { SidePanel } from '@/components/side-panel'
import { PageToolbar } from '@/components/page-toolbar'
import { ToneBadge } from '@/components/tone-badge'
import { EnabledBadge } from '@/components/status-badge'
import { DataTable, proportional, pixel, type DataTableColumn } from '@/components/data-table'
import {
  applyLinkageRules,
  createLinkageRule,
  deleteLinkageRule,
  listLinkageRules,
  updateLinkageRule,
} from '@/apis/alarms-api'
import { listAlgorithms } from '@/apis/algorithms-api'
import { listCameras } from '@/apis/devices-api'
import { listWorkflows } from '@/apis/workflows-api'
import { getErrorMessage } from '@/lib/http'
import type { AlarmLevel, AlarmLinkageRule, AlarmLinkageRuleInput } from '@/types/api'

const LEVEL_LABEL: Record<AlarmLevel, string> = {
  info: '提示',
  warn: '警告',
  critical: '严重',
}

const emptyForm: AlarmLinkageRuleInput = {
  name: '',
  enabled: true,
  levels: [],
  algorithmName: '',
  cameraGroup: '',
  workflowId: '',
  workflowName: '',
  priority: 50,
}

export function LinkageRulesPanel() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [editing, setEditing] = useState<AlarmLinkageRule | null>(null)
  const [form, setForm] = useState<AlarmLinkageRuleInput>(emptyForm)
  /** 多选级别：用逗号拼接存入 Selector 多值不便，改用三个开关 */
  const [levelFlags, setLevelFlags] = useState<Record<AlarmLevel, boolean>>({
    info: false,
    warn: false,
    critical: false,
  })

  const rulesQuery = useQuery({
    queryKey: ['linkage-rules'],
    queryFn: listLinkageRules,
  })
  const workflowsQuery = useQuery({ queryKey: ['workflows'], queryFn: listWorkflows })
  const algorithmsQuery = useQuery({ queryKey: ['algorithms'], queryFn: () => listAlgorithms() })
  const camerasQuery = useQuery({
    queryKey: ['cameras', { forRules: true }],
    queryFn: () => listCameras({ pageSize: 200 }),
  })

  const algorithmOptions = useMemo(() => {
    const names = new Set((algorithmsQuery.data?.items ?? []).map((a) => a.name))
    return [...names].map((n) => ({ label: n, value: n }))
  }, [algorithmsQuery.data])

  const groupOptions = useMemo(() => {
    const groups = new Set((camerasQuery.data?.items ?? []).map((c) => c.group))
    return [...groups].map((g) => ({ label: g, value: g }))
  }, [camerasQuery.data])

  const saveMutation = useMutation({
    mutationFn: async () => {
      const levels = (Object.keys(levelFlags) as AlarmLevel[]).filter((k) => levelFlags[k])
      const wf = (workflowsQuery.data ?? []).find((w) => w.id === form.workflowId)
      if (!wf) throw new Error('请选择联动工作流')
      const input: AlarmLinkageRuleInput = {
        ...form,
        levels,
        algorithmName: form.algorithmName || undefined,
        cameraGroup: form.cameraGroup || undefined,
        workflowName: wf.name,
      }
      if (editing) return updateLinkageRule(editing.id, input)
      return createLinkageRule(input)
    },
    onSuccess: async () => {
      toast({ body: editing ? '规则已更新' : '规则已创建', type: 'info' })
      setDrawerOpen(false)
      setEditing(null)
      await queryClient.invalidateQueries({ queryKey: ['linkage-rules'] })
    },
    onError: (error) => toast({ body: getErrorMessage(error), type: 'error' }),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteLinkageRule(id),
    onSuccess: async () => {
      toast({ body: '规则已删除', type: 'info' })
      setDrawerOpen(false)
      setEditing(null)
      await queryClient.invalidateQueries({ queryKey: ['linkage-rules'] })
    },
    onError: (error) => toast({ body: getErrorMessage(error), type: 'error' }),
  })

  const applyMutation = useMutation({
    mutationFn: () => applyLinkageRules({ onlyUnlinked: true }),
    onSuccess: async (res) => {
      toast({ body: `已按规则更新 ${res.updated} 条报警`, type: 'info' })
      await queryClient.invalidateQueries({ queryKey: ['alarms'] })
    },
    onError: (error) => toast({ body: getErrorMessage(error), type: 'error' }),
  })

  const toggleMutation = useMutation({
    mutationFn: (rule: AlarmLinkageRule) =>
      updateLinkageRule(rule.id, {
        name: rule.name,
        enabled: !rule.enabled,
        levels: rule.levels,
        algorithmName: rule.algorithmName,
        cameraGroup: rule.cameraGroup,
        workflowId: rule.workflowId,
        workflowName: rule.workflowName,
        priority: rule.priority,
      }),
    onSuccess: async (row) => {
      toast({ body: row.enabled ? '规则已启用' : '规则已停用', type: 'info' })
      await queryClient.invalidateQueries({ queryKey: ['linkage-rules'] })
    },
    onError: (error) => toast({ body: getErrorMessage(error), type: 'error' }),
  })

  function openCreate() {
    setEditing(null)
    setForm(emptyForm)
    setLevelFlags({ info: false, warn: false, critical: false })
    setDrawerOpen(true)
  }

  function openEdit(rule: AlarmLinkageRule) {
    setEditing(rule)
    setForm({
      name: rule.name,
      enabled: rule.enabled,
      levels: rule.levels,
      algorithmName: rule.algorithmName ?? '',
      cameraGroup: rule.cameraGroup ?? '',
      workflowId: rule.workflowId,
      workflowName: rule.workflowName,
      priority: rule.priority,
    })
    setLevelFlags({
      info: rule.levels.includes('info'),
      warn: rule.levels.includes('warn'),
      critical: rule.levels.includes('critical'),
    })
    setDrawerOpen(true)
  }

  const enabledCount = (rulesQuery.data ?? []).filter((r) => r.enabled).length

  return (
    <>
      <PageToolbar
        filters={
          <>
            <ToneBadge tone={enabledCount ? 'ok' : 'off'}>
              启用 {enabledCount}/{rulesQuery.data?.length ?? 0}
            </ToneBadge>
            <span className="ff-toolbar-hint">
              按优先级匹配：级别 + 算法 + 分组 → 自动绑定工作流；手工联动优先
            </span>
          </>
        }
        actions={
          <>
            <Button
              label="应用到未联动报警"
              size="sm"
              variant="secondary"
              isLoading={applyMutation.isPending}
              clickAction={async () => {
                await applyMutation.mutateAsync()
              }}
            />
            <Button label="新建规则" size="sm" variant="primary" onClick={openCreate} />
          </>
        }
      />

      <div className="ff-table-wrap">
        <DataTable
          data={rulesQuery.data ?? []}
          idKey="id"
          emptyLabel={rulesQuery.isLoading ? '加载中…' : '暂无联动规则'}
          onRowClick={openEdit}
          columns={
            [
              {
                key: 'priority',
                header: '优先级',
                width: pixel(72),
                render: (r) => (
                  <Text type="supporting" color="secondary">
                    {r.priority}
                  </Text>
                ),
              },
              {
                key: 'name',
                header: '规则名称',
                width: proportional(1.2),
                render: (r) => <Text type="body">{r.name}</Text>,
              },
              {
                key: 'enabled',
                header: '状态',
                width: pixel(80),
                  render: (r) => <EnabledBadge enabled={r.enabled} />,
              },
              {
                key: 'levels',
                header: '级别',
                width: pixel(140),
                render: (r) =>
                  r.levels.length ? (
                    <Text type="supporting">
                      {r.levels.map((l) => LEVEL_LABEL[l]).join(' / ')}
                    </Text>
                  ) : (
                    <Text type="supporting" color="secondary">
                      全部
                    </Text>
                  ),
              },
              {
                key: 'algorithmName',
                header: '算法',
                width: pixel(130),
                render: (r) => r.algorithmName || '全部',
              },
              {
                key: 'cameraGroup',
                header: '分组',
                width: pixel(100),
                render: (r) => r.cameraGroup || '全部',
              },
              {
                key: 'workflowName',
                header: '联动工作流',
                width: proportional(1),
                render: (r) => r.workflowName,
              },
            ] satisfies DataTableColumn<AlarmLinkageRule>[]
          }
        />
      </div>

      <SidePanel
        open={drawerOpen}
        title={editing ? `编辑规则 · ${editing.name}` : '新建联动规则'}
        onClose={() => {
          setDrawerOpen(false)
          setEditing(null)
        }}
        footer={
          <HStack gap={2} justify="end">
            {editing ? (
              <Button
                label="删除"
                size="sm"
                variant="destructive"
                isLoading={deleteMutation.isPending}
                clickAction={async () => {
                  await deleteMutation.mutateAsync(editing.id)
                }}
              />
            ) : null}
            <Button
              label="保存规则"
              size="sm"
              variant="primary"
              isLoading={saveMutation.isPending}
              clickAction={async () => {
                await saveMutation.mutateAsync()
              }}
            />
          </HStack>
        }
      >
        <VStack gap={3}>
          <TextInput
            label="规则名称"
            value={form.name}
            onChange={(v) => setForm((f) => ({ ...f, name: v }))}
          />
          <Switch
            label="启用规则"
            value={form.enabled}
            onChange={(v) => setForm((f) => ({ ...f, enabled: v }))}
          />
          <NumberInput
            label="优先级（越小越先匹配）"
            value={form.priority}
            min={1}
            max={999}
            step={1}
            onChange={(v) => setForm((f) => ({ ...f, priority: v ?? 50 }))}
          />
          <VStack gap={2}>
            <Text type="label">匹配级别（不选=全部）</Text>
            <HStack gap={3} wrap="wrap">
              {(Object.keys(LEVEL_LABEL) as AlarmLevel[]).map((lv) => (
                <Switch
                  key={lv}
                  label={LEVEL_LABEL[lv]}
                  value={levelFlags[lv]}
                  onChange={(v) => setLevelFlags((f) => ({ ...f, [lv]: v }))}
                />
              ))}
            </HStack>
          </VStack>
          <Selector
            label="算法（空=全部）"
            placeholder="全部算法"
            hasClear
            value={form.algorithmName ?? ''}
            onChange={(v) => setForm((f) => ({ ...f, algorithmName: v ?? '' }))}
            options={algorithmOptions}
          />
          <Selector
            label="摄像头分组（空=全部）"
            placeholder="全部分组"
            hasClear
            value={form.cameraGroup ?? ''}
            onChange={(v) => setForm((f) => ({ ...f, cameraGroup: v ?? '' }))}
            options={groupOptions}
          />
          <Selector
            label="联动工作流"
            placeholder="选择工作流"
            value={form.workflowId}
            onChange={(v) => setForm((f) => ({ ...f, workflowId: v ?? '' }))}
            options={(workflowsQuery.data ?? []).map((w) => ({
              label: `${w.name}${w.status === 'running' ? ' · 运行中' : ''}`,
              value: w.id,
            }))}
          />
          {editing ? (
            <HStack gap={2}>
              <Button
                label={editing.enabled ? '停用' : '启用'}
                size="sm"
                variant="secondary"
                isLoading={toggleMutation.isPending}
                clickAction={async () => {
                  await toggleMutation.mutateAsync(editing)
                  setDrawerOpen(false)
                }}
              />
              <Link to="/workflows/$id" params={{ id: editing.workflowId }}>
                <Button label="打开工作流" size="sm" variant="ghost" />
              </Link>
            </HStack>
          ) : null}
          <Text type="supporting" color="secondary">
            保存后可点击「应用到未联动报警」，将规则写回匹配的报警记录。
          </Text>
        </VStack>
      </SidePanel>
    </>
  )
}
