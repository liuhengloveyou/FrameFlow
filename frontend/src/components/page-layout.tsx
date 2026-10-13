import type { ReactNode } from 'react'
import {
  Layout,
  LayoutContent,
  LayoutHeader,
  type SpacingStep,
} from '@astryxdesign/core/Layout'
import { PageHeader } from '@/components/page-header'

export function PageLayout({
  title,
  badge,
  actions,
  toolbar,
  children,
  contentPadding = 4,
  headerHeight = 52,
}: {
  title: string
  /** 标题旁场景徽标，如「全部设备」 */
  badge?: string
  /** 页面级动作（放在全局搜索左侧） */
  actions?: ReactNode
  /** 页头下方的筛选/工具条，落在内容区顶部 */
  toolbar?: ReactNode
  children?: ReactNode
  contentPadding?: SpacingStep
  /** 顶栏高度 */
  headerHeight?: number
  /** @deprecated 页头不再展示描述，保留以免调用方报错 */
  description?: string
  /** @deprecated 保留兼容 */
  logo?: ReactNode
  /** @deprecated 保留兼容 */
  end?: ReactNode
}) {
  return (
    <Layout
      height="fill"
      className="ff-page-layout"
      header={
        <LayoutHeader
          hasDivider
          height={headerHeight}
          padding={0}
          label={title}
          className="ff-page-header"
        >
          <PageHeader title={title} actions={actions} badge={badge} />
        </LayoutHeader>
      }
      content={
        <LayoutContent padding={contentPadding} className="ff-page-content">
          <div className="ff-page-body">
            {toolbar}
            {children}
          </div>
        </LayoutContent>
      }
    />
  )
}
