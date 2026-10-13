import type { ReactNode } from 'react'

/**
 * 页面级导航行：左侧 Tab / 分段控件，右侧挂行内指标。
 *
 * 用于「导航不是白卡」的页面（报警与联动、用户与权限、协议配置…），
 * 让指标与导航共用一行，避免再单起一条卡片指标条占掉整行高度。
 * 指标请传 <InlineStatGroup>。
 */
export function PageTabsRow({
  children,
  stats,
}: {
  children: ReactNode
  /** 右侧行内指标区 */
  stats?: ReactNode
}) {
  return (
    <div className="ff-page-tabs-row">
      <div className="ff-page-tabs-row-main">{children}</div>
      {stats ? <div className="ff-page-tabs-row-stats">{stats}</div> : null}
    </div>
  )
}
