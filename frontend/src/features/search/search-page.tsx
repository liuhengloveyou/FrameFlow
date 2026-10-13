import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useNavigate, useSearch } from '@tanstack/react-router'
import { Selector } from '@astryxdesign/core/Selector'
import { MagnifyingGlassIcon } from '@heroicons/react/24/outline'
import { PageLayout } from '@/components/page-layout'
import { DataTable, proportional, type DataTableColumn } from '@/components/data-table'
import {
  ConfidenceValue,
  EventTypeBadge,
  ReviewStatusBadge,
} from '@/components/event-badges'
import { MediaThumb } from '@/components/media-thumb'
import { searchEvents } from '@/apis/search-api'
import { listReviewEvents } from '@/apis/review-api'
import type { ReviewEventStatus, SearchHit } from '@/types/api'

type ViewMode = 'cards' | 'table'
type SortMode = 'time' | 'score'

const EXAMPLES = ['未戴安全帽', '电动车', '夜间人员', '烟火']

function formatTime(iso: string) {
  const d = new Date(iso)
  const p = (n: number) => String(n).padStart(2, '0')
  return `${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

export function SearchPage() {
  const navigate = useNavigate()
  const routeSearch = useSearch({ from: '/_authenticated/search/' })
  const initialQ = (routeSearch as { q?: string }).q ?? ''

  const [view, setView] = useState<ViewMode>('cards')
  const [sort, setSort] = useState<SortMode>('time')
  const [q, setQ] = useState(initialQ)
  const [applied, setApplied] = useState(initialQ)
  const [elapsedMs, setElapsedMs] = useState(0)

  const query = useQuery({
    queryKey: ['smart-search', { q: applied }],
    queryFn: async () => {
      const t0 = performance.now()
      try {
        return await searchEvents({
          q: applied || undefined,
          pageSize: 100,
        })
      } finally {
        setElapsedMs(performance.now() - t0)
      }
    },
  })

  const hits = query.data?.items ?? []

  /** 置信度不在搜索结果里；按 eventId 关联核查事件取真实分数 */
  const scoresQuery = useQuery({
    queryKey: ['review-events', { forSearchScore: true }],
    queryFn: () => listReviewEvents({ pageSize: 500 }),
    enabled: hits.length > 0,
    staleTime: 60_000,
  })
  const scoreMap = useMemo(() => {
    const map = new Map<string, number>()
    for (const e of scoresQuery.data?.items ?? []) {
      if (e.score != null) map.set(e.id, e.score)
    }
    return map
  }, [scoresQuery.data])

  const items = useMemo(() => {
    const list = [...hits]
    if (sort === 'score') {
      list.sort(
        (a, b) => (scoreMap.get(b.eventId) ?? -1) - (scoreMap.get(a.eventId) ?? -1),
      )
    } else {
      list.sort(
        (a, b) =>
          new Date(b.occurredAt).getTime() - new Date(a.occurredAt).getTime(),
      )
    }
    return list
  }, [hits, sort, scoreMap])

  function submitSearch(keyword: string) {
    const k = keyword.trim()
    setQ(k)
    setApplied(k)
  }

  const columns = [
    {
      key: 'title',
      header: '标题',
      width: proportional(2),
      render: (r: SearchHit) => (
        <span className="ff-page-title">{r.title}</span>
      ),
    },
    {
      key: 'cameraName',
      header: '摄像头',
      width: proportional(1.4),
    },
    {
      key: 'type',
      header: '类型',
      width: proportional(1.2),
      render: (r: SearchHit) => (
        <EventTypeBadge label={r.title || r.objectClass} />
      ),
    },
    {
      key: 'score',
      header: 'AI 置信度',
      width: pixel96(),
      align: 'end',
      render: (r: SearchHit) => (
        <ConfidenceValue score={scoreMap.get(r.eventId)} />
      ),
    },
    {
      key: 'status',
      header: '状态',
      width: proportional(1),
      render: (r: SearchHit) => (
        <ReviewStatusBadge status={r.status as ReviewEventStatus} />
      ),
    },
    {
      key: 'occurredAt',
      header: '时间',
      width: proportional(1.6),
      render: (r: SearchHit) => formatTime(r.occurredAt),
    },
  ] satisfies DataTableColumn<SearchHit>[]

  return (
    <PageLayout title="智能搜索" badge="全量画面事件" contentPadding={4}>
      {/* 居中大搜索框 */}
      <div className="ff-search-hero">
        <form
          className="ff-search-hero-box"
          onSubmit={(e) => {
            e.preventDefault()
            submitSearch(q)
          }}
          role="search"
        >
          <MagnifyingGlassIcon aria-hidden />
          <input
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="搜画面内容：未戴安全帽、电动车、夜间人员…"
            aria-label="搜索画面内容"
          />
          <button type="submit">搜索</button>
        </form>
        <div className="ff-search-chips">
          <span>试试搜</span>
          {EXAMPLES.map((k) => (
            <button
              key={k}
              type="button"
              className="ff-search-chip"
              onClick={() => submitSearch(k)}
            >
              {k}
            </button>
          ))}
        </div>
      </div>

      {/* 结果统计 + 排序 + 视图 */}
      <div className="ff-search-result-bar">
        <span className="ff-toolbar-hint">
          找到 <strong>{query.data?.total ?? 0}</strong> 条结果 · 用时{' '}
          <strong>{(elapsedMs / 1000).toFixed(2)}s</strong>
        </span>
        <div style={{ marginLeft: 'auto', display: 'flex', gap: 10 }}>
          <Selector
            label="排序"
            isLabelHidden
            size="sm"
            value={sort}
            onChange={(v) => setSort((v as SortMode) || 'time')}
            options={[
              { label: '按时间', value: 'time' },
              { label: '按置信度', value: 'score' },
            ]}
          />
          <div className="ff-seg" role="radiogroup" aria-label="结果视图">
            <button
              type="button"
              role="radio"
              aria-checked={view === 'cards'}
              className="ff-seg-item"
              data-active={view === 'cards' ? 'true' : undefined}
              title="卡片视图"
              onClick={() => setView('cards')}
            >
              <svg viewBox="0 0 16 16" fill="none" aria-hidden>
                <rect x="1.5" y="1.5" width="6" height="6" rx="1.4" fill="currentColor" />
                <rect x="8.5" y="1.5" width="6" height="6" rx="1.4" fill="currentColor" />
                <rect x="1.5" y="8.5" width="6" height="6" rx="1.4" fill="currentColor" />
                <rect x="8.5" y="8.5" width="6" height="6" rx="1.4" fill="currentColor" />
              </svg>
            </button>
            <button
              type="button"
              role="radio"
              aria-checked={view === 'table'}
              className="ff-seg-item"
              data-active={view === 'table' ? 'true' : undefined}
              title="表格视图"
              onClick={() => setView('table')}
            >
              <svg viewBox="0 0 16 16" fill="none" aria-hidden>
                <rect x="1.5" y="1.5" width="13" height="3" rx="1" fill="currentColor" />
                <rect x="1.5" y="6" width="13" height="3" rx="1" fill="currentColor" opacity="0.55" />
                <rect x="1.5" y="10.5" width="13" height="3" rx="1" fill="currentColor" opacity="0.55" />
              </svg>
            </button>
          </div>
        </div>
      </div>

      {/* 结果区 */}
      {query.isLoading ? (
        <div className="ff-placeholder">搜索中…</div>
      ) : items.length === 0 ? (
        <div className="ff-placeholder">
          无匹配结果，换个关键词或缩小筛选范围试试
        </div>
      ) : view === 'table' ? (
        <div className="ff-table-wrap">
          <DataTable
            data={items}
            idKey="id"
            isStriped
            emptyLabel="无匹配结果"
            columns={columns}
            onRowClick={() => void navigate({ to: '/review' })}
          />
        </div>
      ) : (
        <div className="ff-result-grid">
          {items.map((h) => {
            const score = scoreMap.get(h.eventId)
            return (
              <button
                key={h.id}
                type="button"
                className="ff-result-card"
                onClick={() => void navigate({ to: '/review' })}
              >
                <MediaThumb src={h.thumbUrl} alt={h.title} seed={h.id} />
                <div className="ff-result-card-meta">
                  <span className="ff-result-card-time">
                    {formatTime(h.occurredAt)}
                  </span>
                  <EventTypeBadge label={h.title || h.objectClass} />
                </div>
                <div className="ff-result-card-foot">
                  <span>{h.cameraName}</span>
                  {h.ocrText ? <span>OCR {h.ocrText}</span> : null}
                  <ConfidenceValue score={score} />
                </div>
              </button>
            )
          })}
        </div>
      )}
    </PageLayout>
  )
}

/** 表格列宽小工具：96px 固定列 */
function pixel96() {
  return proportional(0.9, { minWidth: 96 })
}
