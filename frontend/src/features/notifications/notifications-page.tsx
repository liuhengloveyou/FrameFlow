import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Button } from '@astryxdesign/core/Button'
import { Badge } from '@astryxdesign/core/Badge'
import { TextInput } from '@astryxdesign/core/TextInput'
import { Selector } from '@astryxdesign/core/Selector'
import { Switch } from '@astryxdesign/core/Switch'
import {
  SegmentedControl,
  SegmentedControlItem,
} from '@astryxdesign/core/SegmentedControl'
import { HStack } from '@astryxdesign/core/HStack'
import { VStack } from '@astryxdesign/core/VStack'
import { Text } from '@astryxdesign/core/Text'
import { useToast } from '@astryxdesign/core/Toast'
import { PageLayout } from '@/components/page-layout'
import { SidePanel } from '@/components/side-panel'
import { DataTable, proportional, pixel, type DataTableColumn } from '@/components/data-table'
import {
  createNotifyChannel,
  deleteNotifyChannel,
  listNotifyChannels,
  listNotifyDeliveries,
  updateNotifyChannel,
  type NotifyChannelInput,
} from '@/apis/notifications-api'
import { getErrorMessage } from '@/lib/http'
import type {
  NotifyChannel,
  NotifyChannelKind,
  NotifyDelivery,
  NotifyDeliveryStatus,
} from '@/types/api'

type Tab = 'channels' | 'deliveries'

const KIND_LABEL: Record<NotifyChannelKind, string> = {
  email: '邮件',
  webhook: 'Webhook',
  im: '即时通讯',
}

const STATUS_LABEL: Record<NotifyDeliveryStatus, string> = {
  success: '成功',
  failed: '失败',
  pending: '排队',
}

const emptyForm: NotifyChannelInput = {
  name: '',
  kind: 'email',
  enabled: true,
  endpoint: '',
}

export function NotificationsPage() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const [tab, setTab] = useState<Tab>('channels')
  const [panelOpen, setPanelOpen] = useState(false)
  const [editing, setEditing] = useState<NotifyChannel | null>(null)
  const [form, setForm] = useState<NotifyChannelInput>(emptyForm)
  const [status, setStatus] = useState<NotifyDeliveryStatus | ''>('')
  const [q, setQ] = useState('')

  const channelsQuery = useQuery({
    queryKey: ['notify-channels'],
    queryFn: listNotifyChannels,
  })
  const deliveriesQuery = useQuery({
    queryKey: ['notify-deliveries', { status, q }],
    queryFn: () => listNotifyDeliveries({ status: status || undefined, q: q || undefined }),
    enabled: tab === 'deliveries',
  })

  const saveMut = useMutation({
    mutationFn: async () => {
      if (!form.name.trim() || !form.endpoint.trim()) throw new Error('请填写名称与端点')
      if (editing) return updateNotifyChannel(editing.id, form)
      return createNotifyChannel(form)
    },
    onSuccess: async () => {
      toast({ body: editing ? '渠道已更新' : '渠道已创建', type: 'info' })
      setPanelOpen(false)
      setEditing(null)
      setForm(emptyForm)
      await queryClient.invalidateQueries({ queryKey: ['notify-channels'] })
    },
    onError: (err) => toast({ body: getErrorMessage(err), type: 'error' }),
  })

  const deleteMut = useMutation({
    mutationFn: deleteNotifyChannel,
    onSuccess: async () => {
      toast({ body: '渠道已删除', type: 'info' })
      await queryClient.invalidateQueries({ queryKey: ['notify-channels'] })
    },
    onError: (err) => toast({ body: getErrorMessage(err), type: 'error' }),
  })

  const channelColumns = useMemo(
    () =>
      [
        {
          key: 'name',
          header: '名称',
          width: proportional(1.4),
          render: (r: NotifyChannel) => <Text type="body">{r.name}</Text>,
        },
        {
          key: 'kind',
          header: '类型',
          width: proportional(1),
          render: (r: NotifyChannel) => <Badge label={KIND_LABEL[r.kind]} variant="neutral" />,
        },
        {
          key: 'endpoint',
          header: '端点',
          width: proportional(2.4),
          render: (r: NotifyChannel) => (
            <Text type="supporting" color="secondary">
              {r.endpoint}
            </Text>
          ),
        },
        {
          key: 'enabled',
          header: '启用',
          width: pixel(80),
          render: (r: NotifyChannel) => (r.enabled ? '是' : '否'),
        },
        {
          key: 'updatedAt',
          header: '更新',
          width: proportional(1.4),
          render: (r: NotifyChannel) => new Date(r.updatedAt).toLocaleString(),
        },
        {
          key: 'actions',
          header: '操作',
          width: pixel(200),
          render: (r: NotifyChannel) => (
            <HStack gap={1}>
              <Button
                label="编辑"
                size="sm"
                variant="ghost"
                clickAction={() => {
                  setEditing(r)
                  setForm({
                    name: r.name,
                    kind: r.kind,
                    enabled: r.enabled,
                    endpoint: r.endpoint,
                  })
                  setPanelOpen(true)
                }}
              />
              <Button
                label="删除"
                size="sm"
                variant="ghost"
                clickAction={async () => {
                  await deleteMut.mutateAsync(r.id)
                }}
              />
            </HStack>
          ),
        },
      ] satisfies DataTableColumn<NotifyChannel>[],
    [deleteMut],
  )

  const deliveryColumns = [
    {
      key: 'title',
      header: '标题',
      width: proportional(1.6),
      render: (r: NotifyDelivery) => <Text type="body">{r.title}</Text>,
    },
    {
      key: 'channelName',
      header: '渠道',
      width: proportional(1.2),
    },
    {
      key: 'kind',
      header: '类型',
      width: proportional(1),
      render: (r: NotifyDelivery) => KIND_LABEL[r.kind],
    },
    {
      key: 'status',
      header: '状态',
      width: proportional(1),
      render: (r: NotifyDelivery) => (
        <Badge
          label={STATUS_LABEL[r.status]}
          variant={
            r.status === 'success' ? 'success' : r.status === 'failed' ? 'error' : 'warning'
          }
        />
      ),
    },
    {
      key: 'detail',
      header: '详情',
      width: proportional(1.6),
    },
    {
      key: 'sentAt',
      header: '时间',
      width: proportional(1.4),
      render: (r: NotifyDelivery) => new Date(r.sentAt).toLocaleString(),
    },
  ] satisfies DataTableColumn<NotifyDelivery>[]

  return (
    <PageLayout
      title="通知中心"
      actions={
        tab === 'channels' ? (
          <Button
            label="新建渠道"
            size="sm"
            clickAction={() => {
              setEditing(null)
              setForm(emptyForm)
              setPanelOpen(true)
            }}
          />
        ) : null
      }
      toolbar={
        <HStack gap={2} vAlign="center" wrap="wrap">
          <SegmentedControl label="通知视图" value={tab} onChange={(v) => setTab(v as Tab)}>
            <SegmentedControlItem value="channels" label="渠道配置" />
            <SegmentedControlItem value="deliveries" label="投递记录" />
          </SegmentedControl>
          {tab === 'deliveries' ? (
            <>
              <TextInput
                label="搜索"
                isLabelHidden
                placeholder="标题 / 渠道"
                value={q}
                onChange={setQ}
                hasClear
              />
              <Selector
                label="状态"
                isLabelHidden
                placeholder="全部状态"
                hasClear
                value={status}
                onChange={(v) => setStatus((v ?? '') as NotifyDeliveryStatus | '')}
                options={[
                  { label: '成功', value: 'success' },
                  { label: '失败', value: 'failed' },
                  { label: '排队', value: 'pending' },
                ]}
              />
            </>
          ) : null}
        </HStack>
      }
    >
      {tab === 'channels' ? (
        <div className="ff-table-wrap">
          <DataTable
            data={channelsQuery.data ?? []}
            idKey="id"
            emptyLabel={channelsQuery.isLoading ? '加载中…' : '暂无渠道'}
            columns={channelColumns}
          />
        </div>
      ) : (
        <div className="ff-table-wrap">
          <DataTable
            data={deliveriesQuery.data?.items ?? []}
            idKey="id"
            emptyLabel={deliveriesQuery.isLoading ? '加载中…' : '暂无投递记录'}
            columns={deliveryColumns}
          />
        </div>
      )}

      <SidePanel
        open={panelOpen}
        title={editing ? '编辑渠道' : '新建渠道'}
        onClose={() => setPanelOpen(false)}
        footer={
          <HStack gap={2} justify="end">
            <Button label="取消" variant="secondary" size="sm" onClick={() => setPanelOpen(false)} />
            <Button
              label="保存"
              size="sm"
              isLoading={saveMut.isPending}
              clickAction={async () => {
                await saveMut.mutateAsync()
              }}
            />
          </HStack>
        }
      >
        <VStack gap={3}>
          <TextInput
            label="名称"
            value={form.name}
            onChange={(v) => setForm((f) => ({ ...f, name: v }))}
          />
          <Selector
            label="类型"
            value={form.kind}
            onChange={(v) => setForm((f) => ({ ...f, kind: (v ?? 'email') as NotifyChannelKind }))}
            options={[
              { label: '邮件', value: 'email' },
              { label: 'Webhook', value: 'webhook' },
              { label: '即时通讯', value: 'im' },
            ]}
          />
          <TextInput
            label="端点"
            value={form.endpoint}
            onChange={(v) => setForm((f) => ({ ...f, endpoint: v }))}
            placeholder="邮箱 / URL / IM Webhook"
          />
          <Switch
            label="启用"
            value={form.enabled}
            onChange={(v) => setForm((f) => ({ ...f, enabled: v }))}
          />
        </VStack>
      </SidePanel>
    </PageLayout>
  )
}
