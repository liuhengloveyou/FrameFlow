import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Button } from '@astryxdesign/core/Button'
import { Card } from '@astryxdesign/core/Card'
import { Badge } from '@astryxdesign/core/Badge'
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
import { PageLayout } from '@/components/page-layout'
import { DataTable, proportional, type DataTableColumn } from '@/components/data-table'
import { getSystemSnapshot, type SystemAccelerator } from '@/apis/system-api'
import { listAuditLogs, type AuditLog } from '@/apis/users-api'

const ACCEL_STATUS: Record<
  SystemAccelerator['status'],
  { label: string; variant: 'success' | 'warning' | 'neutral' | 'error' }
> = {
  ready: { label: '空闲', variant: 'success' },
  busy: { label: '忙碌', variant: 'warning' },
  missing: { label: '未检测到', variant: 'neutral' },
  error: { label: '异常', variant: 'error' },
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
      title="运维"
      toolbar={
        <SegmentedControl label="系统视图" value={tab} onChange={(v) => setTab(v as Tab)}>
          <SegmentedControlItem value="overview" label="资源探针" />
          <SegmentedControlItem value="audit" label="审计日志" />
        </SegmentedControl>
      }
      actions={
        tab === 'overview' ? (
          <HStack gap={3} vAlign="center" wrap="wrap">
            <Switch
              label={`自动刷新 ${REFRESH_MS / 1000}s`}
              value={autoRefresh}
              onChange={setAutoRefresh}
            />
            {data?.sampledAt ? (
              <Text type="supporting" color="secondary">
                采样 {new Date(data.sampledAt).toLocaleTimeString()}
                {query.isFetching ? ' · 更新中' : ''}
              </Text>
            ) : null}
            <Button
              label="立即刷新"
              variant="secondary"
              size="sm"
              isLoading={query.isFetching}
              clickAction={async () => {
                await query.refetch()
              }}
            />
          </HStack>
        ) : (
          <HStack gap={2} vAlign="center" wrap="wrap">
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
          </HStack>
        )
      }
    >
      {tab === 'audit' ? (
        <div className="ff-table-wrap">
          <DataTable
            data={auditQuery.data?.items ?? []}
            idKey="id"
            emptyLabel={auditQuery.isLoading ? '加载中…' : '暂无审计日志'}
            columns={auditColumns}
          />
        </div>
      ) : data ? (
        <>
          <HStack gap={2} vAlign="center">
            <Badge label={data.version} variant="neutral" />
            <Text type="supporting" color="secondary">
              运行时长 {data.uptime}
            </Text>
          </HStack>

          <VStack gap={2}>
            <Text type="label">服务状态</Text>
            <div className="ff-model-grid">
              {data.services.map((s) => (
                <Card key={s.name} padding={4}>
                  <VStack gap={2}>
                    <strong>{s.name}</strong>
                    <Badge
                      label={
                        s.status === 'up' ? '正常' : s.status === 'degraded' ? '降级' : '异常'
                      }
                      variant={
                        s.status === 'up'
                          ? 'success'
                          : s.status === 'degraded'
                            ? 'warning'
                            : 'error'
                      }
                    />
                  </VStack>
                </Card>
              ))}
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
                    <strong style={{ fontSize: '1.35rem' }}>{m.value}</strong>
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
                const st = ACCEL_STATUS[a.status]
                return (
                  <Card key={a.id} padding={4}>
                    <VStack gap={2}>
                      <HStack justify="between" vAlign="center">
                        <strong>
                          {a.kind} · {a.name}
                        </strong>
                        <Badge label={st.label} variant={st.variant} />
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
              <strong>系统告警</strong>
              {data.alerts.map((a) => (
                <HStack key={a.id} gap={2} vAlign="center">
                  <Badge
                    label={a.level === 'warn' ? '警告' : '信息'}
                    variant={a.level === 'warn' ? 'warning' : 'neutral'}
                  />
                  <Text type="body">{a.body}</Text>
                </HStack>
              ))}
            </VStack>
          </Card>
        </>
      ) : (
        <div className="ff-placeholder">{query.isLoading ? '加载中…' : '暂无数据'}</div>
      )}
    </PageLayout>
  )
}
