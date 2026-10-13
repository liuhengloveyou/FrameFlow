import { useEffect, useState, type ReactNode } from 'react'
import { useNavigate, useRouterState } from '@tanstack/react-router'
import { useIsFetching, useQueryClient } from '@tanstack/react-query'
import { TopNav } from '@astryxdesign/core/TopNav'
import { HStack } from '@astryxdesign/core/HStack'
import {
  ArrowPathIcon,
  BellIcon,
  ClockIcon,
  MagnifyingGlassIcon,
  Bars3BottomLeftIcon,
  Bars3Icon,
} from '@heroicons/react/24/outline'
import { useShellNav } from '@/app/shell-nav-context'
import { AccountMenu } from '@/features/account/account-menu'
import { listAlarms } from '@/apis/alarms-api'
import { useQuery } from '@tanstack/react-query'

function formatClock(d: Date) {
  const p = (n: number) => String(n).padStart(2, '0')
  return `${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}

/**
 * 全局顶栏（所有页面统一）：
 * 左 = 折叠开关 + 页面标题 + 场景徽标；
 * 右 = 页面动作 + 全局搜索 + 最后更新时间 + 刷新 + 通知铃铛 + 账号头像。
 */
export function PageHeader({
  title,
  actions,
  badge,
}: {
  title: string
  actions?: ReactNode
  /** 标题旁场景徽标，如「全部设备」 */
  badge?: string
}) {
  const shellNav = useShellNav()
  const ToggleIcon = shellNav?.isCollapsed ? Bars3Icon : Bars3BottomLeftIcon
  const navigate = useNavigate()
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  const queryClient = useQueryClient()
  const fetching = useIsFetching()

  const [q, setQ] = useState('')
  const [lastUpdated, setLastUpdated] = useState(() => new Date())

  // 路由切换即视为一次数据刷新，更新时间
  useEffect(() => {
    setLastUpdated(new Date())
  }, [pathname])

  // 铃铛红点：存在未处理报警时点亮
  const alarmsQuery = useQuery({
    queryKey: ['open-alarms-count'],
    queryFn: () => listAlarms({ status: 'open', pageSize: 1 }),
    refetchInterval: 30_000,
    staleTime: 10_000,
  })
  const hasOpenAlarms = (alarmsQuery.data?.total ?? 0) > 0

  async function refreshAll() {
    await queryClient.invalidateQueries()
    setLastUpdated(new Date())
  }

  function submitSearch(e: React.FormEvent) {
    e.preventDefault()
    const keyword = q.trim()
    if (!keyword) return
    void navigate({ to: '/search', search: { q: keyword } })
  }

  return (
    <TopNav
      label={title}
      className="ff-page-header-nav"
      heading={
        <HStack gap={2} vAlign="center" className="ff-page-header-heading">
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
          <span className="ff-page-title">{title}</span>
          {badge ? <span className="ff-topbar-badge">{badge}</span> : null}
        </HStack>
      }
      endContent={
        <div className="ff-page-header-actions">
          {actions}
          <form className="ff-topbar-search" onSubmit={submitSearch} role="search">
            <MagnifyingGlassIcon aria-hidden />
            <input
              type="search"
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder="搜索设备、通道、事件"
              aria-label="全局搜索"
            />
          </form>
          <span className="ff-topbar-time" title="数据最后更新时间">
            <ClockIcon aria-hidden />
            最后更新 {formatClock(lastUpdated)}
          </span>
          <button
            type="button"
            className="ff-topbar-iconbtn"
            aria-label="刷新数据"
            title="刷新数据"
            data-busy={fetching > 0 ? 'true' : undefined}
            onClick={() => void refreshAll()}
          >
            <ArrowPathIcon aria-hidden />
          </button>
          <button
            type="button"
            className="ff-topbar-iconbtn"
            aria-label="通知中心"
            title="通知中心"
            onClick={() => void navigate({ to: '/notifications' })}
          >
            <BellIcon aria-hidden />
            {hasOpenAlarms ? <span className="ff-topbar-dot" /> : null}
          </button>
          <AccountMenu />
        </div>
      }
    />
  )
}
