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
  actions,
  logo,
  toolbar,
  children,
  contentPadding = 4,
  headerHeight = 48,
  end,
}: {
  title: string
  /** @deprecated 页头不再展示描述，保留以免调用方报错 */
  description?: string
  actions?: ReactNode
  logo?: ReactNode
  /** 页头下方的筛选/工具条，落在内容区顶部 */
  toolbar?: ReactNode
  children?: ReactNode
  contentPadding?: SpacingStep
  /** 顶栏高度（含右侧 sm 按钮，略高于默认以免裁切） */
  headerHeight?: number
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
          <PageHeader title={title} actions={actions} logo={logo} />
        </LayoutHeader>
      }
      end={end}
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
