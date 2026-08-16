import type { ReactNode } from 'react'
import { TopNav, TopNavHeading } from '@astryxdesign/core/TopNav'
import { HStack } from '@astryxdesign/core/HStack'
import { Bars3BottomLeftIcon, Bars3Icon } from '@heroicons/react/24/outline'
import { useShellNav } from '@/app/shell-nav-context'

export function PageHeader({
  title,
  actions,
  logo,
}: {
  title: string
  actions?: ReactNode
  logo?: ReactNode
}) {
  const shellNav = useShellNav()
  const ToggleIcon = shellNav?.isCollapsed ? Bars3Icon : Bars3BottomLeftIcon

  return (
    <TopNav
      label={title}
      className="ff-page-header-nav"
      heading={
        <HStack gap={1} vAlign="center" className="ff-page-header-heading">
          {shellNav ? (
            <button
              type="button"
              className="ff-sidenav-toggle"
              aria-label={shellNav.isCollapsed ? '展开侧栏' : '收起侧栏'}
              onClick={shellNav.toggle}
            >
              <ToggleIcon aria-hidden className="ff-sidenav-toggle-icon" />
            </button>
          ) : null}
          <TopNavHeading heading={title} logo={logo} />
        </HStack>
      }
      endContent={
        actions ? <div className="ff-page-header-actions">{actions}</div> : undefined
      }
    />
  )
}
