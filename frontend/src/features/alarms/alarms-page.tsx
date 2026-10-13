import { useMemo, useState } from 'react'
import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Button } from '@astryxdesign/core/Button'
import { TextInput } from '@astryxdesign/core/TextInput'
import { Selector } from '@astryxdesign/core/Selector'
import { HStack } from '@astryxdesign/core/HStack'
import { VStack } from '@astryxdesign/core/VStack'
import { Text } from '@astryxdesign/core/Text'
import { useToast } from '@astryxdesign/core/Toast'
import {
  ArchiveBoxIcon,
  BellAlertIcon,
  CheckBadgeIcon,
  ExclamationTriangleIcon,
} from '@heroicons/react/24/outline'
import { PageLayout } from '@/components/page-layout'
import { PageToolbar } from '@/components/page-toolbar'
import { PageTabsRow } from '@/components/page-tabs-row'
import { InlineStat, InlineStatGroup } from '@/components/inline-stat'
import { SidePanel } from '@/components/side-panel'
import { AlarmLevelBadge, AlarmStatusBadge } from '@/components/status-badge'
import { ToneBadge } from '@/components/tone-badge'
import { DataTable, proportional, pixel, type DataTableColumn } from '@/components/data-table'
import { linkAlarmWorkflow, listAlarms, listLinkageRules, updateAlarmStatus } from '@/apis/alarms-api'
import { listWorkflows } from '@/apis/workflows-api'
import { findMatchingRule } from '@/lib/match-linkage-rule'
import { getErrorMessage } from '@/lib/http'
import { seedPreviewSource } from '@/lib/preview-seed'
import type { AlarmLevel, AlarmRecord, AlarmStatus } from '@/types/api'
import { AlarmLinkageSubnav } from './alarm-linkage-subnav'

const PAGE_SIZE = 10

const LEVEL_LABEL: Record<AlarmLevel, string> = {
  info: '提示',
  warn: '警告',
  critical: '严重',
}

const STATUS_LABEL: Record<AlarmStatus, string> = {
  open: '未处理',
  ack: '已确认',
  closed: '已关闭',
}

export function AlarmsPage() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const [q, setQ] = useState('')
  const [level, setLevel] = useState('')
  const [status, setStatus] = useState('')
  const [selected, setSelected] = useState<AlarmRecord | null>(null)
  const [linkWfId, setLinkWfId] = useState('')

  const query = useInfiniteQuery({
    queryKey: ['alarms', { q, level, status, infinite: true }],
    queryFn: ({ pageParam }) =>
      listAlarms({
        q,
        level: level || undefined,
        status: status || undefined,
        page: pageParam,
        pageSize: PAGE_SIZE,
      }),
    initialPageParam: 1,
    getNextPageParam: (last) => {
      const loaded = last.page * last.pageSize
      return loaded < last.total ? last.page + 1 : undefined
    },
  })
  const workflowsQuery = useQuery({ queryKey: ['workflows'], queryFn: listWorkflows })
  const rulesQuery = useQuery({
    queryKey: ['linkage-rules'],
    queryFn: listLinkageRules,
  })

  // 各处置状态的计数（与列表筛选解耦，pageSize=1 只取 total）
  const countsQuery = useQuery({
    queryKey: ['alarms-counts'],
    queryFn: async () => {
      const [open, ack, closed, all] = await Promise.all([
        listAlarms({ status: 'open', page: 1, pageSize: 1 }),
        listAlarms({ status: 'ack', page: 1, pageSize: 1 }),
        listAlarms({ status: 'closed', page: 1, pageSize: 1 }),
        listAlarms({ page: 1, pageSize: 1 }),
      ])
      return { open: open.total, ack: ack.total, closed: closed.total, all: all.total }
    },
  })
  const counts = countsQuery.data

  const items = useMemo(
    () => query.data?.pages.flatMap((p) => p.items) ?? [],
    [query.data],
  )
  const total = query.data?.pages[0]?.total ?? 0

  const invalidateAll = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['alarms'] }),
      queryClient.invalidateQueries({ queryKey: ['alarms-counts'] }),
      queryClient.invalidateQueries({ queryKey: ['open-alarms-count'] }),
    ])
  }

  const statusMutation = useMutation({
    mutationFn: ({ id, next }: { id: number; next: AlarmStatus }) => updateAlarmStatus(id, next),
    onSuccess: async (row) => {
      toast({ body: `已更新为「${STATUS_LABEL[row.status]}」`, type: 'info' })
      setSelected(row)
      await invalidateAll()
    },
    onError: (error) => toast({ body: getErrorMessage(error), type: 'error' }),
  })

  /** 批量确认：按当前筛选取一批待处理报警并全部置为已确认（不逐条弹提示） */
  const ackAllMutation = useMutation({
    mutationFn: async () => {
      const opens = await listAlarms({
        q,
        level: level || undefined,
        status: 'open',
        page: 1,
        pageSize: 200,
      })
      await Promise.all(opens.items.map((a) => updateAlarmStatus(a.id, 'ack')))
      return opens.items.length
    },
    onSuccess: async (n) => {
      toast({ body: n ? `已确认 ${n} 条报警` : '没有待处理报警', type: 'info' })
      await invalidateAll()
    },
    onError: (error) => toast({ body: getErrorMessage(error), type: 'error' }),
  })

  const linkMutation = useMutation({
    mutationFn: () => {
      if (!selected) throw new Error('未选择报警')
      if (!linkWfId) return linkAlarmWorkflow(selected.id, null)
      const wf = (workflowsQuery.data ?? []).find((w) => w.id === linkWfId)
      if (!wf) throw new Error('工作流不存在')
      return linkAlarmWorkflow(selected.id, { id: wf.id, name: wf.name })
    },
    onSuccess: async (row) => {
      toast({
        body: row.workflowId
          ? '已保存手工联动（若工作流运行中将立即执行）'
          : '已清除联动',
        type: 'info',
      })
      setSelected(row)
      await invalidateAll()
    },
    onError: (error) => toast({ body: getErrorMessage(error), type: 'error' }),
  })

  const openCount = counts?.open ?? 0

  const suggestedRule = useMemo(() => {
    if (!selected || !rulesQuery.data) return null
    return findMatchingRule(rulesQuery.data, selected)
  }, [selected, rulesQuery.data])

  return (
    <PageLayout
      title="报警记录"
      badge={openCount ? `待处理 ${openCount}` : '全部已处理'}
      toolbar={
        <PageTabsRow
          stats={
            <InlineStatGroup>
              <InlineStat
                icon={<ExclamationTriangleIcon />}
                value={counts?.open ?? '—'}
                label="未处理"
                tone="warn"
              />
              <InlineStat
                icon={<CheckBadgeIcon />}
                value={counts?.ack ?? '—'}
                label="已确认"
              />
              <InlineStat
                icon={<ArchiveBoxIcon />}
                value={counts?.closed ?? '—'}
                label="已关闭"
                tone="ok"
              />
              <InlineStat
                icon={<BellAlertIcon />}
                value={counts?.all ?? '—'}
                label="报警总量"
                tone="teal"
              />
            </InlineStatGroup>
          }
        >
          <AlarmLinkageSubnav />
        </PageTabsRow>
      }
    >
      <>
        <PageToolbar
          filters={
            <>
              <TextInput
                label="搜索"
                isLabelHidden
                placeholder="标题 / 摄像头 / 算法 / 信息"
                value={q}
                onChange={setQ}
                hasClear
              />
              <Selector
                label="级别"
                isLabelHidden
                placeholder="全部级别"
                hasClear
                value={level}
                onChange={(v) => setLevel(v ?? '')}
                options={[
                  { label: '提示', value: 'info' },
                  { label: '警告', value: 'warn' },
                  { label: '严重', value: 'critical' },
                ]}
              />
              <Selector
                label="状态"
                isLabelHidden
                placeholder="全部状态"
                hasClear
                value={status}
                onChange={(v) => setStatus(v ?? '')}
                options={[
                  { label: '未处理', value: 'open' },
                  { label: '已确认', value: 'ack' },
                  { label: '已关闭', value: 'closed' },
                ]}
              />
            </>
          }
          actions={
            <Button
              label="全部确认"
              size="sm"
              variant="secondary"
              isDisabled={!openCount}
              isLoading={ackAllMutation.isPending}
              clickAction={async () => {
                await ackAllMutation.mutateAsync()
              }}
            />
          }
        />

        <div className="ff-table-wrap">
          <DataTable
            data={items}
            idKey="id"
            groupBy="level"
            groupLabel={(key) => LEVEL_LABEL[key as AlarmLevel] ?? key}
            emptyLabel={query.isLoading ? '加载中…' : '暂无报警'}
            onRowClick={(row) => {
              setSelected(row)
              setLinkWfId(row.workflowId ?? '')
            }}
            columns={
              [
                {
                  key: 'title',
                  header: '标题',
                  width: proportional(1.1),
                  render: (a) => <Text type="body">{a.title}</Text>,
                },
                {
                  key: 'level',
                  header: '级别',
                  width: pixel(88),
                  render: (a) => <AlarmLevelBadge level={a.level} />,
                },
                {
                  key: 'status',
                  header: '状态',
                  width: pixel(96),
                  render: (a) => <AlarmStatusBadge status={a.status} />,
                },
                {
                  key: 'cameraName',
                  header: '摄像头',
                  width: pixel(130),
                },
                {
                  key: 'algorithmName',
                  header: '算法',
                  width: pixel(120),
                },
                {
                  key: 'workflowName',
                  header: '联动工作流',
                  width: pixel(170),
                  render: (a) =>
                    a.workflowName ? (
                      <VStack gap={0}>
                        <Text type="body">{a.workflowName}</Text>
                        {a.linkageRuleName ? (
                          <Text type="supporting" color="secondary">
                            规则 · {a.linkageRuleName}
                          </Text>
                        ) : (
                          <Text type="supporting" color="secondary">
                            手工联动
                          </Text>
                        )}
                      </VStack>
                    ) : (
                      <Text type="supporting" color="secondary">
                        —
                      </Text>
                    ),
                },
                {
                  key: 'occurredAt',
                  header: '发生时间',
                  width: pixel(170),
                  render: (a) => (
                    <Text type="supporting" color="secondary">
                      {new Date(a.occurredAt).toLocaleString()}
                    </Text>
                  ),
                },
              ] satisfies DataTableColumn<AlarmRecord>[]
            }
          />
        </div>

        {query.hasNextPage ? (
          <div className="ff-load-more">
            <Button
              label="加载更多"
              variant="secondary"
              size="sm"
              isLoading={query.isFetchingNextPage}
              clickAction={async () => {
                await query.fetchNextPage()
              }}
            />
            <span className="ff-toolbar-hint">
              已加载 {items.length}/{total}
            </span>
          </div>
        ) : items.length > 0 ? (
          <Text type="supporting" color="secondary" style={{ textAlign: 'center' }}>
            已加载全部 {total} 条
          </Text>
        ) : null}

        <SidePanel
          open={Boolean(selected)}
          title={selected?.title ?? '报警详情'}
          onClose={() => setSelected(null)}
          footer={
            selected ? (
              <HStack gap={2}>
                {selected.status === 'open' ? (
                  <Button
                    label="确认"
                    variant="secondary"
                    isLoading={statusMutation.isPending}
                    clickAction={async () => {
                      await statusMutation.mutateAsync({ id: selected.id, next: 'ack' })
                    }}
                  />
                ) : null}
                {selected.status !== 'closed' ? (
                  <Button
                    label="关闭报警"
                    variant="primary"
                    isLoading={statusMutation.isPending}
                    clickAction={async () => {
                      await statusMutation.mutateAsync({ id: selected.id, next: 'closed' })
                    }}
                  />
                ) : null}
              </HStack>
            ) : null
          }
        >
          {selected ? (
            <VStack gap={3}>
              <HStack gap={2}>
                <AlarmLevelBadge level={selected.level} />
                <AlarmStatusBadge status={selected.status} />
              </HStack>
              <Text type="body">{selected.message}</Text>
              <VStack gap={1}>
                <Text type="supporting" color="secondary">
                  摄像头：{selected.cameraName}
                  {selected.cameraGroup ? ` · ${selected.cameraGroup}` : ''}
                </Text>
                <Text type="supporting" color="secondary">
                  算法：{selected.algorithmName}
                </Text>
                <Text type="supporting" color="secondary">
                  时间：{new Date(selected.occurredAt).toLocaleString()}
                </Text>
              </VStack>
              <VStack gap={2}>
                <Text type="label">报警证据</Text>
                {selected.evidenceNote ? (
                  <Text type="supporting" color="secondary">
                    {selected.evidenceNote}
                  </Text>
                ) : (
                  <Text type="supporting" color="secondary">
                    暂无证据说明
                  </Text>
                )}
                {selected.evidenceFileIds?.length ? (
                  <div className="ff-file-preview ff-file-preview-media">
                    <img
                      className="ff-file-preview-img"
                      src={`/api/v1/alarms/${selected.id}/evidence`}
                      alt="报警抓拍"
                    />
                    <div className="ff-file-preview-caption">检出抓拍</div>
                  </div>
                ) : (
                  <div className="ff-file-preview" style={{ minHeight: 100 }}>
                    无关联文件
                  </div>
                )}
                {selected.evidenceFileIds?.length ? (
                  <a href={`/api/v1/alarms/${selected.id}/evidence`} target="_blank" rel="noreferrer">
                    <Button label="查看原图" size="sm" variant="secondary" />
                  </a>
                ) : null}
              </VStack>
              <VStack gap={2}>
                <HStack gap={2} vAlign="center">
                  <Text type="label">联动配置</Text>
                  {selected.workflowName ? (
                    <ToneBadge tone="info">{selected.workflowName}</ToneBadge>
                  ) : (
                    <ToneBadge tone="off">未联动</ToneBadge>
                  )}
                </HStack>
                {selected.linkageRuleName ? (
                  <Text type="supporting" color="secondary">
                    来源规则：{selected.linkageRuleName}
                  </Text>
                ) : selected.workflowId ? (
                  <Text type="supporting" color="secondary">
                    来源：手工指定（应用规则时不会覆盖）
                  </Text>
                ) : suggestedRule ? (
                  <Text type="supporting" color="secondary">
                    建议规则「{suggestedRule.name}」→ {suggestedRule.workflowName}
                    （可到{' '}
                    <Link to="/alarms/rules">联动规则</Link>
                    {' '}页应用）
                  </Text>
                ) : (
                  <Text type="supporting" color="secondary">
                    无匹配联动规则，可手工指定工作流
                  </Text>
                )}
                <Selector
                  label="联动工作流"
                  isLabelHidden
                  placeholder="不联动"
                  hasClear
                  value={linkWfId}
                  onChange={(v) => setLinkWfId(v ?? '')}
                  options={(workflowsQuery.data ?? []).map((w) => ({
                    label: `${w.name}${w.status === 'running' ? ' · 运行中' : ''}`,
                    value: w.id,
                  }))}
                />
                <HStack gap={2}>
                  <Button
                    label="保存手工联动"
                    size="sm"
                    variant="primary"
                    isLoading={linkMutation.isPending}
                    clickAction={async () => {
                      await linkMutation.mutateAsync()
                    }}
                  />
                  {selected.workflowId ? (
                    <Link to="/workflows/$id" params={{ id: selected.workflowId }}>
                      <Button label="打开工作流" size="sm" variant="secondary" />
                    </Link>
                  ) : null}
                </HStack>
              </VStack>
              <HStack gap={2}>
                <Link
                  to="/preview"
                  onClick={() =>
                    seedPreviewSource({ cameraId: selected.cameraId })
                  }
                >
                  <Button label="预览画面" size="sm" variant="secondary" />
                </Link>
                <Link to="/devices/$id/ai" params={{ id: selected.cameraId }}>
                  <Button label="视频 AI" size="sm" variant="ghost" />
                </Link>
              </HStack>
            </VStack>
          ) : null}
        </SidePanel>
      </>
    </PageLayout>
  )
}
