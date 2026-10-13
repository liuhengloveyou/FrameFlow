import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Button } from '@astryxdesign/core/Button'
import { HStack } from '@astryxdesign/core/HStack'
import { Text } from '@astryxdesign/core/Text'
import { ToneBadge } from '@/components/tone-badge'
import { ReviewStatusBadge } from '@/components/event-badges'
import { listReviewContext } from '@/apis/review-api'
import type { ReviewEvent, ReviewEventKind } from '@/types/api'
import { formatAbsoluteShort, formatRelativeTime } from './review-time'

const KIND_LABEL: Record<ReviewEventKind, string> = {
  alert: '警报',
  detection: '检测',
  motion: '变动',
}

const CTX_PAGE_SIZE = 15

export function ReviewDetail({
  event,
  neighbors,
  onClose,
  onSelect,
  onMark,
  onPromote,
  onOpenLive,
  onOpenEventVideo,
  promoting,
  marking,
}: {
  event: ReviewEvent
  neighbors: { prev?: ReviewEvent; next?: ReviewEvent }
  onClose: () => void
  onSelect: (e: ReviewEvent) => void
  onMark: (reviewed: boolean) => void
  onPromote: () => void
  onOpenLive: () => void
  /** 跳到事件时刻观看录像 */
  onOpenEventVideo: () => void
  promoting?: boolean
  marking?: boolean
}) {
  const [ctxPage, setCtxPage] = useState(0)

  useEffect(() => {
    setCtxPage(0)
  }, [event.id])

  const ctxQuery = useQuery({
    queryKey: ['review-context', event.id, ctxPage],
    queryFn: () =>
      listReviewContext(event.id, {
        windowMinutes: 45,
        page: ctxPage,
        pageSize: CTX_PAGE_SIZE,
      }),
  })

  const context = ctxQuery.data?.items ?? []
  const ctxTotal = ctxQuery.data?.total ?? 0
  const ctxPageSize = ctxQuery.data?.pageSize ?? CTX_PAGE_SIZE
  const ctxTotalPages = Math.max(1, Math.ceil(ctxTotal / ctxPageSize))
  const resolvedCtxPage = ctxQuery.data?.page ?? (ctxPage > 0 ? ctxPage : 1)
  const reviewed = event.status === 'reviewed'

  const metaLine = useMemo(() => {
    const parts = [event.cameraName, formatAbsoluteShort(event.occurredAt)]
    if (event.objectClass) parts.push(event.objectClass)
    if (event.score != null) parts.push(`${Math.round(event.score * 100)}%`)
    return parts.filter(Boolean).join(' · ')
  }, [event])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault()
        onClose()
        return
      }
      if (e.key === 'ArrowLeft' && neighbors.prev) {
        e.preventDefault()
        onSelect(neighbors.prev)
      }
      if (e.key === 'ArrowRight' && neighbors.next) {
        e.preventDefault()
        onSelect(neighbors.next)
      }
      if (e.key === 'r' || e.key === 'R') {
        e.preventDefault()
        onMark(!reviewed)
      }
      if (e.key === 'p' || e.key === 'P') {
        e.preventDefault()
        onOpenEventVideo()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [reviewed, neighbors.next, neighbors.prev, onClose, onMark, onOpenEventVideo, onSelect])

  return (
    <div className="ff-review-detail" role="dialog" aria-modal="true" aria-label="事件详情">
      <div className="ff-review-detail-backdrop" onClick={onClose} />
      <div className="ff-review-detail-panel">
        <header className="ff-review-detail-header">
          <div className="ff-review-detail-header-main">
            <div className="ff-review-detail-title-row">
              <h2 className="ff-review-detail-title">{event.title}</h2>
              <ToneBadge tone="off">{KIND_LABEL[event.kind]}</ToneBadge>
              <ReviewStatusBadge status={reviewed ? 'reviewed' : 'unreviewed'} />
            </div>
            <p className="ff-review-detail-meta">{metaLine}</p>
          </div>
          <HStack gap={1} wrap="wrap">
            <Button
              label="上一条"
              size="sm"
              variant="ghost"
              isDisabled={!neighbors.prev}
              clickAction={() => neighbors.prev && onSelect(neighbors.prev)}
            />
            <Button
              label="下一条"
              size="sm"
              variant="ghost"
              isDisabled={!neighbors.next}
              clickAction={() => neighbors.next && onSelect(neighbors.next)}
            />
            <Button label="关闭" size="sm" variant="secondary" clickAction={onClose} />
          </HStack>
        </header>

        <div className="ff-review-detail-body">
          <div className="ff-review-detail-player">
            <button
              type="button"
              className="ff-review-detail-stage"
              onClick={onOpenEventVideo}
              title="观看事件视频"
            >
              {event.thumbUrl ? <img src={event.thumbUrl} alt="" /> : null}
              {event.boxes?.length ? (
                <svg className="ff-review-detail-boxes" viewBox="0 0 1 1" preserveAspectRatio="none">
                  {event.boxes.map((b, i) => (
                    <g key={i}>
                      <rect
                        x={b.x}
                        y={b.y}
                        width={b.w}
                        height={b.h}
                        className="ff-review-detail-box"
                      />
                      {b.label ? (
                        <text
                          x={b.x}
                          y={Math.max(0.03, b.y - 0.015)}
                          className="ff-review-detail-box-label"
                        >
                          {b.label}
                        </text>
                      ) : null}
                    </g>
                  ))}
                </svg>
              ) : null}
              <div className="ff-review-detail-stage-play" aria-hidden>
                <span className="ff-review-detail-stage-play-icon" />
                <span>观看事件视频</span>
              </div>
              <div className="ff-review-detail-stage-caption">
                {formatRelativeTime(event.occurredAt)} · 点击画面跳转回放
              </div>
            </button>

            <div className="ff-review-detail-actions">
              <Button
                label="观看事件视频"
                size="sm"
                clickAction={onOpenEventVideo}
              />
              <Button
                label={reviewed ? '标为待核查' : '标记已核查'}
                size="sm"
                variant="secondary"
                isLoading={marking}
                clickAction={() => onMark(!reviewed)}
              />
              {event.kind === 'detection' && !event.promotedAlertId ? (
                <Button
                  label="提升为警报"
                  size="sm"
                  variant="secondary"
                  isLoading={promoting}
                  clickAction={onPromote}
                />
              ) : null}
              <Button label="实时监控" size="sm" variant="ghost" clickAction={onOpenLive} />
            </div>

            {(event.message || event.objectClass || event.score != null) && (
              <div className="ff-review-detail-info">
                {event.message ? <p className="ff-review-detail-msg">{event.message}</p> : null}
                <p className="ff-review-detail-hint">
                  {event.objectClass ? `类别 ${event.objectClass}` : null}
                  {event.objectColor ? ` · 颜色 ${event.objectColor}` : null}
                  {event.ocrText ? ` · OCR ${event.ocrText}` : null}
                  {event.score != null ? ` · 置信度 ${event.score.toFixed(2)}` : null}
                </p>
              </div>
            )}

            <p className="ff-review-detail-keys">
              Esc 关闭 · ←/→ 切换 · R 核查 · P 观看视频
            </p>
          </div>

          <aside className="ff-review-detail-context">
            <div className="ff-review-detail-context-head">
              <Text type="label">同机上下文</Text>
              <span className="ff-review-detail-context-sub">
                ±45 分钟
                {ctxTotal > 0 ? ` · ${ctxTotal} 条` : ''}
              </span>
            </div>
            <div className="ff-review-detail-context-list">
              {ctxQuery.isLoading ? (
                <Text type="supporting" color="secondary">
                  加载中…
                </Text>
              ) : ctxQuery.isError ? (
                <Text type="supporting" color="secondary">
                  上下文加载失败
                </Text>
              ) : context.length === 0 ? (
                <Text type="supporting" color="secondary">
                  暂无邻近事件
                </Text>
              ) : (
                context.map((c) => (
                  <button
                    key={c.id}
                    type="button"
                    className="ff-review-context-item"
                    data-current={c.id === event.id ? 'true' : undefined}
                    data-kind={c.kind}
                    onClick={() => onSelect(c)}
                  >
                    <span className="ff-review-context-time">
                      {new Date(c.occurredAt).toLocaleTimeString([], {
                        hour: '2-digit',
                        minute: '2-digit',
                        second: '2-digit',
                      })}
                    </span>
                    <span className="ff-review-context-title">{c.title}</span>
                    <span className="ff-review-context-kind">{KIND_LABEL[c.kind]}</span>
                  </button>
                ))
              )}
            </div>
            {ctxTotalPages > 1 ? (
              <div className="ff-review-detail-context-pager">
                <Button
                  label="上一页"
                  size="sm"
                  variant="ghost"
                  isDisabled={resolvedCtxPage <= 1 || ctxQuery.isFetching}
                  clickAction={() => setCtxPage(Math.max(1, resolvedCtxPage - 1))}
                />
                <span className="ff-review-detail-context-page">
                  {resolvedCtxPage} / {ctxTotalPages}
                </span>
                <Button
                  label="下一页"
                  size="sm"
                  variant="ghost"
                  isDisabled={
                    resolvedCtxPage >= ctxTotalPages || ctxQuery.isFetching
                  }
                  clickAction={() =>
                    setCtxPage(Math.min(ctxTotalPages, resolvedCtxPage + 1))
                  }
                />
              </div>
            ) : null}
          </aside>
        </div>
      </div>
    </div>
  )
}
