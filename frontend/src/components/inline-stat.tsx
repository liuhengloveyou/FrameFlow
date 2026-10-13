import type { ReactNode } from 'react'

export type StatTone = 'brand' | 'ok' | 'warn' | 'teal'

/**
 * 行内指标：图标 + 数字 + 标签，单行排布。
 *
 * 全站统一的指标形态——挂在页面工具条或导航行的右侧，
 * 与筛选/动作共用一行，不额外占一整行高度。
 * （旧的卡片式指标条已废弃：每个页面顶部压 4 张卡会吃掉约 86px 纵向空间。）
 */
export function InlineStat({
  icon,
  value,
  unit,
  label,
  tone = 'brand',
  onClick,
  title,
}: {
  icon: ReactNode
  value: ReactNode
  unit?: string
  label: string
  tone?: StatTone
  onClick?: () => void
  title?: string
}) {
  const Tag = onClick ? 'button' : 'span'
  return (
    <Tag
      className="ff-live-stat"
      data-tone={tone}
      onClick={onClick}
      title={title}
      type={onClick ? 'button' : undefined}
    >
      <span className="ff-live-stat-icon" aria-hidden>
        {icon}
      </span>
      <span className="ff-live-stat-value">
        {value}
        {unit ? <span className="ff-live-stat-unit">{unit}</span> : null}
      </span>
      <span className="ff-live-stat-label">{label}</span>
    </Tag>
  )
}

/** 行内指标组：左侧细分割线，相邻指标之间自动补一条竖分隔线 */
export function InlineStatGroup({ children }: { children: ReactNode }) {
  return <div className="ff-live-stats">{children}</div>
}
