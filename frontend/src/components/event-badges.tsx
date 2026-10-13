/**
 * 事件类领域徽章 + 置信度。
 * 事件类型按关键词映射色调（未戴安全帽/区域入侵/人员聚集=橙，
 * 车牌识别=蓝，烟火/安全警报=红）。
 * 底层形态见 `tone-badge`，全站共用。
 */
import { ToneBadge, type Tone } from './tone-badge'

/** @deprecated 使用 `Tone`；保留别名以免旧引用报错 */
export type EventTone = Tone

function toneForLabel(label: string): Tone {
  if (/烟火|火|烟/.test(label)) return 'danger'
  if (/车牌/.test(label)) return 'info'
  if (/安全帽|入侵|聚集|徘徊|越界|未戴/.test(label)) return 'warn'
  return 'off'
}

export function EventTypeBadge({ label }: { label?: string }) {
  const text = label?.trim() || '未知事件'
  return <ToneBadge tone={toneForLabel(text)}>{text}</ToneBadge>
}

/** 处置状态：待核查(橙) / 已确认(绿) / 误报(灰) */
export function ReviewStatusBadge({ status }: { status: string }) {
  const map: Record<string, { text: string; tone: Tone }> = {
    unreviewed: { text: '待核查', tone: 'warn' },
    reviewed: { text: '已确认', tone: 'ok' },
    dismissed: { text: '误报', tone: 'off' },
  }
  const meta = map[status] ?? { text: status, tone: 'off' as Tone }
  return <ToneBadge tone={meta.tone}>{meta.text}</ToneBadge>
}

/** AI 置信度：等宽数字，0~1 归一化展示两位小数 */
export function ConfidenceValue({ score }: { score?: number }) {
  if (score == null || Number.isNaN(score)) {
    return <span className="ff-conf">—</span>
  }
  return <span className="ff-conf">{score.toFixed(2)}</span>
}
