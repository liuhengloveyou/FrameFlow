import type { ReactNode } from 'react'

/**
 * 页面工具条（白卡）：左侧筛选组 + 中部行内指标 + 右侧动作组。
 * 设备管理 / 地图 / 回放 / 核查 / 搜索等页统一复用。
 *
 * stats 适合放 <InlineStatGroup>——「内容优先」的页面（地图/回放）
 * 用行内指标，不额外占一整行高度。
 */
export function PageToolbar({
  filters,
  stats,
  actions,
}: {
  filters?: ReactNode
  /** 中部行内指标区（靠右对齐，位于筛选与动作之间） */
  stats?: ReactNode
  actions?: ReactNode
}) {
  return (
    <div className="ff-toolbar-card">
      {filters ? <div className="ff-toolbar-filters">{filters}</div> : null}
      {stats || actions ? (
        <div className="ff-toolbar-end">
          {stats ? <div className="ff-toolbar-stats">{stats}</div> : null}
          {actions ? <div className="ff-toolbar-actions">{actions}</div> : null}
        </div>
      ) : null}
    </div>
  )
}
