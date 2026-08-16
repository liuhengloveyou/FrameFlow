import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { Button } from '@astryxdesign/core/Button'
import { Badge } from '@astryxdesign/core/Badge'
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
import { searchEvents } from '@/apis/search-api'
import { listCameras } from '@/apis/devices-api'
import type { ReviewEventStatus, SearchHit } from '@/types/api'

type ViewMode = 'cards' | 'table'

export function SearchPage() {
  const navigate = useNavigate()
  const [view, setView] = useState<ViewMode>('cards')
  const [q, setQ] = useState('')
  const [cameraId, setCameraId] = useState('')
  const [status, setStatus] = useState<ReviewEventStatus | ''>('')
  const [objectClass, setObjectClass] = useState('')
  const [objectColor, setObjectColor] = useState('')
  const [ocr, setOcr] = useState('')
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [applied, setApplied] = useState({
    q: '',
    cameraId: '',
    status: '' as ReviewEventStatus | '',
    objectClass: '',
    objectColor: '',
    ocr: '',
    from: '',
    to: '',
  })

  const camerasQuery = useQuery({
    queryKey: ['cameras', { forSearch: true }],
    queryFn: () => listCameras({ page: 1, pageSize: 200 }),
  })

  const query = useQuery({
    queryKey: ['smart-search', applied],
    queryFn: () =>
      searchEvents({
        q: applied.q || undefined,
        cameraIds: applied.cameraId ? [applied.cameraId] : undefined,
        status: applied.status || undefined,
        objectClass: applied.objectClass || undefined,
        objectColor: applied.objectColor || undefined,
        ocr: applied.ocr || undefined,
        from: applied.from || undefined,
        to: applied.to || undefined,
        pageSize: 100,
      }),
  })

  const items = query.data?.items ?? []

  const cameraOptions = useMemo(
    () => [
      { label: '全部摄像头', value: '' },
      ...(camerasQuery.data?.items.map((c) => ({
        label: c.name,
        value: String(c.id),
      })) ?? []),
    ],
    [camerasQuery.data],
  )

  const columns = [
    {
      key: 'title',
      header: '标题',
      width: proportional(2),
      render: (r: SearchHit) => <Text type="body">{r.title}</Text>,
    },
    {
      key: 'cameraName',
      header: '摄像头',
      width: proportional(1.4),
    },
    {
      key: 'objectClass',
      header: '类别',
      width: proportional(1),
      render: (r: SearchHit) => r.objectClass ?? '—',
    },
    {
      key: 'objectColor',
      header: '颜色',
      width: proportional(1),
      render: (r: SearchHit) => r.objectColor ?? '—',
    },
    {
      key: 'ocrText',
      header: 'OCR',
      width: proportional(1.2),
      render: (r: SearchHit) => r.ocrText || '—',
    },
    {
      key: 'status',
      header: '状态',
      width: proportional(1),
      render: (r: SearchHit) => (
        <Badge
          label={r.status === 'reviewed' ? '已核查' : '待核查'}
          variant={r.status === 'reviewed' ? 'success' : 'warning'}
        />
      ),
    },
    {
      key: 'occurredAt',
      header: '时间',
      width: proportional(1.6),
      render: (r: SearchHit) => new Date(r.occurredAt).toLocaleString(),
    },
  ] satisfies DataTableColumn<SearchHit>[]

  return (
    <PageLayout
      title="智能搜索"
      actions={
        <SegmentedControl
          label="结果视图"
          value={view}
          onChange={(v) => setView(v as ViewMode)}
        >
          <SegmentedControlItem value="cards" label="卡片" />
          <SegmentedControlItem value="table" label="表格" />
        </SegmentedControl>
      }
      toolbar={
        <div className="ff-search-filters">
          <TextInput
            label="关键词"
            isLabelHidden
            placeholder="标题 / 摄像头"
            value={q}
            onChange={setQ}
            hasClear
          />
          <Selector
            label="摄像头"
            isLabelHidden
            placeholder="全部摄像头"
            hasClear
            value={cameraId}
            onChange={(v) => setCameraId(v ?? '')}
            options={cameraOptions.filter((o) => o.value)}
          />
          <Selector
            label="状态"
            isLabelHidden
            placeholder="处理状态"
            hasClear
            value={status}
            onChange={(v) => setStatus((v ?? '') as ReviewEventStatus | '')}
            options={[
              { label: '待核查', value: 'unreviewed' },
              { label: '已核查', value: 'reviewed' },
            ]}
          />
          <Selector
            label="类别"
            isLabelHidden
            placeholder="对象类别"
            hasClear
            value={objectClass}
            onChange={(v) => setObjectClass(v ?? '')}
            options={[
              { label: '人', value: '人' },
              { label: '车', value: '车' },
              { label: '非机动车', value: '非机动车' },
              { label: '其他', value: '其他' },
            ]}
          />
          <TextInput
            label="颜色"
            isLabelHidden
            placeholder="对象颜色"
            value={objectColor}
            onChange={setObjectColor}
            hasClear
          />
          <TextInput
            label="OCR"
            isLabelHidden
            placeholder="车牌 / 工牌"
            value={ocr}
            onChange={setOcr}
            hasClear
          />
          <TextInput
            label="起"
            isLabelHidden
            placeholder="起始 ISO 时间"
            value={from}
            onChange={setFrom}
          />
          <TextInput
            label="止"
            isLabelHidden
            placeholder="结束 ISO 时间"
            value={to}
            onChange={setTo}
          />
          <Button
            label="检索"
            size="sm"
            clickAction={async () => {
              setApplied({
                q,
                cameraId,
                status,
                objectClass,
                objectColor,
                ocr,
                from,
                to,
              })
            }}
          />
        </div>
      }
    >
      <VStack gap={3}>
        <Text type="supporting" color="secondary">
          共 {query.data?.total ?? 0} 条结果（Mock 特征检索）
        </Text>

        {view === 'table' ? (
          <div className="ff-table-wrap">
            <DataTable
              data={items}
              idKey="id"
              emptyLabel={query.isLoading ? '加载中…' : '无匹配结果'}
              columns={columns}
              onRowClick={() => void navigate({ to: '/review' })}
            />
          </div>
        ) : query.isLoading ? (
          <div className="ff-placeholder">加载中…</div>
        ) : items.length === 0 ? (
          <div className="ff-placeholder">无匹配结果</div>
        ) : (
          <div className="ff-search-cards">
            {items.map((h) => (
              <button
                key={h.id}
                type="button"
                className="ff-search-card"
                onClick={() => {
                  void navigate({ to: '/review' })
                }}
              >
                <div className="ff-search-card-thumb">
                  {h.thumbUrl ? <img src={h.thumbUrl} alt="" /> : null}
                </div>
                <VStack gap={1}>
                  <HStack justify="between" vAlign="center" gap={1}>
                    <Text type="body">{h.title}</Text>
                    <Badge
                      label={h.status === 'reviewed' ? '已核查' : '待核查'}
                      variant={h.status === 'reviewed' ? 'success' : 'warning'}
                    />
                  </HStack>
                  <Text type="supporting" color="secondary">
                    {h.cameraName}
                    {h.objectClass ? ` · ${h.objectClass}` : ''}
                    {h.objectColor ? ` · ${h.objectColor}` : ''}
                  </Text>
                  {h.ocrText ? (
                    <Text type="supporting" color="secondary">
                      OCR {h.ocrText}
                    </Text>
                  ) : null}
                  <Text type="supporting" color="secondary">
                    {new Date(h.occurredAt).toLocaleString()}
                  </Text>
                </VStack>
              </button>
            ))}
          </div>
        )}
      </VStack>
    </PageLayout>
  )
}
