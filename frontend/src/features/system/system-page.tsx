import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Button } from '@astryxdesign/core/Button'
import { Card } from '@astryxdesign/core/Card'
import { Switch } from '@astryxdesign/core/Switch'
import { TextInput } from '@astryxdesign/core/TextInput'
import { Selector } from '@astryxdesign/core/Selector'
import {
  SegmentedControl,
  SegmentedControlItem,
} from '@astryxdesign/core/SegmentedControl'
import { HStack } from '@astryxdesign/core/HStack'
import { VStack } from '@astryxdesign/core/VStack'
import { Text } from '@astryxdesign/core/Text'
import {
  CheckCircleIcon,
  CpuChipIcon,
  ExclamationTriangleIcon,
  ServerStackIcon,
} from '@heroicons/react/24/outline'
import { PageLayout } from '@/components/page-layout'
import { PageToolbar } from '@/components/page-toolbar'
import { PageTabsRow } from '@/components/page-tabs-row'
import { InlineStat, InlineStatGroup } from '@/components/inline-stat'
import { ToneBadge, type Tone } from '@/components/tone-badge'
import { DataTable, proportional, type DataTableColumn } from '@/components/data-table'
import { getSystemSnapshot, type SystemAccelerator } from '@/apis/system-api'
import { listAuditLogs, type AuditLog } from '@/apis/users-api'

const ACCEL_STATUS: Record<
  SystemAccelerator['status'],
  { label: string; tone: Tone }
> = {
  ready: { label: '空闲', tone: 'ok' },
  busy: { label: '忙碌', tone: 'warn' },
  missing: { label: '未检测到', tone: 'off' },
  error: { label: '异常', tone: 'danger' },
}

const SERVICE_STATUS: Record<string, { label: string; tone: Tone }> = {
  up: { label: '正常', tone: 'ok' },
  degraded: { label: '降级', tone: 'warn' },
  down: { label: '异常', tone: 'danger' },
}

const REFRESH_MS = 5000

type Tab = 'overview' | 'audit'

export function SystemPage() {
  const [tab, setTab] = useState<Tab>('overview')
  const [autoRefresh, setAutoRefresh] = useState(true)
  const [auditQ, setAuditQ] = useState('')
  const [auditModule, setAuditModule] = useState('')

  const query = useQuery({
    queryKey: ['system'],
    queryFn: getSystemSnapshot,
    refetchInterval: tab === 'overview' && autoRefresh ? REFRESH_MS : false,
    refetchIntervalInBackground: false,
  })
  const data = query.data

  const auditQuery = useQuery({
    queryKey: ['audit-logs', { auditQ, auditModule }],
    queryFn: () =>
      listAuditLogs({
        q: auditQ || undefined,
        module: auditModule || undefined,
        pageSize: 100,
      }),
    enabled: tab === 'audit',
  })

  const auditColumns = [
    {
      key: 'at',
      header: '时间',
      width: proportional(1.5),
      render: (r: AuditLog) => new Date(r.at).toLocaleString(),
    },
    {
      key: 'actor',
      header: '账号',
      width: proportional(1),
    },
    {
      key: 'module',
      header: '模块',
      width: proportional(1),
    },
    {
      key: 'action',
      header: '动作',
      width: proportional(1.4),
    },
    {
      key: 'target',
      header: '对象',
      width: proportional(1.2),
    },
    {
      key: 'ip',
      header: 'IP',
      width: proportional(1.2),
    },
  ] satisfies DataTableColumn<AuditLog>[]

  return (
    <PageLayout
      title="系统运维"
      badge={data?.version ? `v${data.version.replace(/^v/i, '')}` : '资源探针'}
      toolbar={
        <PageTabsRow
          stats={
            tab === 'overview' && data ? (
              <InlineStatGroup>
                <InlineStat
                  icon={<ServerStackIcon />}
                  value={data.services.length}
                  label="受监控服务"
                />
                <InlineStat
                  icon={<CheckCircleIcon />}
                  value={data.services.filter((s) => s.status === 'up').length}
                  label="服务正常"
                  tone="ok"
                />
                <InlineStat
                  icon={<CpuChipIcon />}
                  value={data.accelerators.length}
                  label="算力设备"
                  tone="teal"
                />
                <InlineStat
                  icon={<ExclamationTriangleIcon />}
                  value={data.alerts.length}
                  label="系统告警"
                  tone="warn"
                />
              </InlineStatGroup>
            ) : undefined
          }
        >
          <SegmentedControl label="系统视图" value={tab} onChange={(v) => setTab(v as Tab)}>
            <SegmentedControlItem value="overview" label="资源探针" />
            <SegmentedControlItem value="audit" label="审计日志" />
          </SegmentedControl>
        </PageTabsRow>
      }
    >
      {tab === 'audit' ? (
        <>
          <PageToolbar
            filters={
              <>
                <TextInput
                  label="搜索"
                  isLabelHidden
                  placeholder="动作 / 账号 / 对象"
                  value={auditQ}
                  onChange={setAuditQ}
                  hasClear
                />
                <Selector
                  label="模块"
                  isLabelHidden
                  placeholder="全部模块"
                  hasClear
                  value={auditModule}
                  onChange={(v) => setAuditModule(v ?? '')}
                  options={[
                    { label: '认证', value: '认证' },
                    { label: '设备', value: '设备' },
                    { label: '报警', value: '报警' },
                    { label: '工作流', value: '工作流' },
                    { label: '事件核查', value: '事件核查' },
                    { label: '录像', value: '录像' },
                    { label: '用户', value: '用户' },
                  ]}
                />
              </>
            }
            actions={<span className="ff-toolbar-hint">最近 100 条操作记录</span>}
          />
          <div className="ff-table-wrap">
            <DataTable
              data={auditQuery.data?.items ?? []}
              idKey="id"
              emptyLabel={auditQuery.isLoading ? '加载中…' : '暂无审计日志'}
              columns={auditColumns}
            />
          </div>
        </>
      ) : data ? (
        <>
          <PageToolbar
            filters={
              <>
                <ToneBadge tone="info">{data.version}</ToneBadge>
                <span className="ff-toolbar-hint">
                  运行时长 {data.uptime}
                  {data.sampledAt
                    ? ` · 采样 ${new Date(data.sampledAt).toLocaleTimeString()}`
                    : ''}
                </span>
              </>
            }
            actions={
              <>
                <Switch
                  label={`自动刷新 ${REFRESH_MS / 1000}s`}
                  value={autoRefresh}
                  onChange={setAutoRefresh}
                />
                <Button
                  label="立即刷新"
                  variant="secondary"
                  size="sm"
                  isLoading={query.isFetching}
                  clickAction={async () => {
                    await query.refetch()
                  }}
                />
              </>
            }
          />

          <VStack gap={2}>
            <Text type="label">服务状态</Text>
            <div className="ff-model-grid">
              {data.services.map((s) => {
                const st = SERVICE_STATUS[s.status] ?? { label: s.status, tone: 'off' as Tone }
                return (
                  <Card key={s.name} padding={4}>
                    <VStack gap={2}>
                      <strong className="ff-wf-card-name">{s.name}</strong>
                      <HStack gap={2} vAlign="center">
                        <ToneBadge tone={st.tone}>{st.label}</ToneBadge>
                      </HStack>
                    </VStack>
                  </Card>
                )
              })}
            </div>
          </VStack>

          <VStack gap={2}>
            <Text type="label">资源占用</Text>
            <div className="ff-model-grid">
              {data.metrics.map((m) => (
                <Card key={m.label} padding={4}>
                  <VStack gap={2}>
                    <HStack justify="between" vAlign="center">
                      <Text type="supporting" color="secondary">
                        {m.label}
                      </Text>
                      {m.hint ? (
                        <Text type="supporting" color="secondary">
                          {m.hint}
                        </Text>
                      ) : null}
                    </HStack>
                    <strong className="ff-metric-value">{m.value}</strong>
                    {m.percent != null ? (
                      <div className="ff-metric-bar" data-level={m.percent >= 80 ? 'high' : 'ok'}>
                        <span style={{ width: `${Math.min(100, m.percent)}%` }} />
                      </div>
                    ) : null}
                  </VStack>
                </Card>
              ))}
            </div>
          </VStack>

          <VStack gap={2}>
            <Text type="label">算力设备</Text>
            <div className="ff-model-grid">
              {data.accelerators.map((a) => {
                const st = ACCEL_STATUS[a.status] ?? { label: a.status, tone: 'off' as Tone }
                return (
                  <Card key={a.id} padding={4}>
                    <VStack gap={2}>
                      <HStack justify="between" vAlign="center">
                        <strong className="ff-wf-card-name">
                          {a.kind} · {a.name}
                        </strong>
                        <ToneBadge tone={st.tone}>{st.label}</ToneBadge>
                      </HStack>
                      <Text type="supporting" color="secondary">
                        利用率 {a.util}
                        {a.memory ? ` · 显存 ${a.memory}` : ''}
                      </Text>
                      <Text type="supporting" color="secondary">
                        {a.temperature ? `温度 ${a.temperature} · ` : ''}
                        驱动 {a.driver ?? '—'}
                      </Text>
                    </VStack>
                  </Card>
                )
              })}
            </div>
          </VStack>

          <Card padding={4}>
            <VStack gap={3}>
              <strong className="ff-wf-card-name">系统告警</strong>
              {data.alerts.length === 0 ? (
                <Text type="supporting" color="secondary">
                  暂无系统告警
                </Text>
              ) : (
                data.alerts.map((a) => (
                  <HStack key={a.id} gap={2} vAlign="center">
                    <ToneBadge tone={a.level === 'warn' ? 'warn' : 'info'}>
                      {a.level === 'warn' ? '警告' : '信息'}
                    </ToneBadge>
                    <Text type="body">{a.body}</Text>
                  </HStack>
                ))
              )}
            </VStack>
          </Card>
        </>
      ) : (
        <div className="ff-placeholder">
          {query.isLoading
            ? '加载中…'
            : query.isError
              ? '资源探针接口暂不可用；接入后这里会显示服务状态、资源占用与算力设备。'
              : '暂无数据'}
        </div>
      )}
    </PageLayout>
  )
}
