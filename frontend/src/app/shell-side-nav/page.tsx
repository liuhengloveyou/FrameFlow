import {
  useCallback,
  useMemo,
  useState,
  type MouseEvent,
  type ReactNode,
} from 'react'
import { useNavigate, useRouterState } from '@tanstack/react-router'
import { AppShell } from '@astryxdesign/core/AppShell'
import { Divider } from '@astryxdesign/core/Divider'
import {
  SideNav,
  SideNavItem,
  SideNavSection,
} from '@astryxdesign/core/SideNav'
import { ShellNavProvider } from '@/app/shell-nav-context'
import {
  MapIcon,
  VideoCameraIcon,
  CpuChipIcon,
  CubeIcon,
  SquaresPlusIcon,
  QueueListIcon,
  BellAlertIcon,
  UsersIcon,
  FilmIcon,
  MagnifyingGlassIcon,
  MegaphoneIcon,
  ClipboardDocumentCheckIcon,
  AdjustmentsHorizontalIcon,
  Cog6ToothIcon,
  SignalIcon,
} from '@heroicons/react/24/outline'
import type { IconType } from '@astryxdesign/core/Icon'
import { useAuthStore } from '@/stores/auth-store'
import { AccountMenu } from '@/features/account/account-menu'

type NavItem = {
  label: string;
  href: string;
  icon: IconType;
};

const monitorNav: NavItem[] = [
  { label: "地图空间", href: "/map", icon: MapIcon },
  { label: "实时监控", href: "/preview", icon: VideoCameraIcon },
  { label: "视频回放", href: "/recording", icon: FilmIcon },
];

const deviceNav: NavItem[] = [
  { label: "设备管理", href: "/devices", icon: CpuChipIcon },
];

const aiNav: NavItem[] = [
  { label: "模型仓库", href: "/models", icon: CubeIcon },
  { label: "算法接入", href: "/algorithms", icon: SquaresPlusIcon },
];

/** 报警与联动：记录 / 规则 / 工作流 */
const linkageNav: NavItem[] = [
  { label: "报警记录", href: "/alarms", icon: BellAlertIcon },
  { label: "联动规则", href: "/alarms/rules", icon: AdjustmentsHorizontalIcon },
  { label: "工作流", href: "/workflows", icon: QueueListIcon },
];

/** 核查：事件核查 / 智能搜索 */
const eventNav: NavItem[] = [
  { label: "事件核查", href: "/review", icon: ClipboardDocumentCheckIcon },
  { label: "智能搜索", href: "/search", icon: MagnifyingGlassIcon },
];

/** 系统：运维 + 协议配置 + 通知 + 用户 */
const opsNav: NavItem[] = [
  { label: "协议配置", href: "/system/protocols", icon: SignalIcon },
  { label: "用户&权限", href: "/users", icon: UsersIcon },
  { label: "通知中心", href: "/notifications", icon: MegaphoneIcon },
  { label: "系统运维", href: "/system", icon: Cog6ToothIcon },
];

function pathSelected(pathname: string, href: string) {
  const path =
    pathname.endsWith("/") && pathname.length > 1
      ? pathname.slice(0, -1)
      : pathname;
  const base = href.endsWith("/") && href.length > 1 ? href.slice(0, -1) : href;
  // 父路径勿吞掉子路径：/system 不选中 /system/protocols；/alarms 不选中 /alarms/rules
  if (base === "/alarms" || base === "/devices" || base === "/system") {
    return path === base;
  }
  return path === base || path.startsWith(`${base}/`);
}

function NavItems({
  items,
  pathname,
  canAccess,
}: {
  items: NavItem[];
  pathname: string;
  canAccess: (href: string) => boolean;
}) {
  const visible = items.filter((item) => canAccess(item.href));
  if (!visible.length) return null;
  return (
    <>
      {visible.map((item) => (
        <SideNavItem
          key={item.href}
          label={item.label}
          href={item.href}
          icon={item.icon}
          isSelected={pathSelected(pathname, item.href)}
        />
      ))}
    </>
  );
}

/**
 * Astryx shell-side-nav：
 * SideNav 主导航 · collapsible 内置折叠 · 展开固定宽度
 */
export function ShellSideNav({ children }: { children: ReactNode }) {
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  const navigate = useNavigate()
  const hasMenuAccess = useAuthStore((s) => s.hasMenuAccess)
  const [sideCollapsed, setSideCollapsed] = useState(false)
  const toggleSideNav = useCallback(() => setSideCollapsed((v) => !v), [])
  const shellNav = useMemo(
    () => ({ isCollapsed: sideCollapsed, toggle: toggleSideNav }),
    [sideCollapsed, toggleSideNav],
  )

  const goHome = (e: MouseEvent) => {
    e.preventDefault()
    void navigate({ to: '/preview' })
  }

  return (
    <ShellNavProvider value={shellNav}>
      <AppShell
        contentPadding={0}
        height="fill"
        variant="elevated"
        // 避免中等宽度窗口把侧栏收进移动抽屉且无顶栏入口
        mobileNav={{ breakpoint: "sm" }}
        sideNav={
          <SideNav
            className={
              sideCollapsed ? "ff-sidenav ff-sidenav--collapsed" : "ff-sidenav"
            }
            collapsible={{
              // 折叠按钮放在内容区 PageHeader 标题左侧
              hasButton: false,
              isCollapsed: sideCollapsed,
              onCollapsedChange: setSideCollapsed,
            }}
            header={
              sideCollapsed ? (
                <a
                  className="ff-sidenav-brand ff-sidenav-brand--collapsed"
                  href="/preview"
                  onClick={goHome}
                >
                  <img
                    className="ff-sidenav-mark"
                    src="/favicon_mark.png"
                    alt="巡帧 FrameFlow"
                    width={44}
                    height={41}
                  />
                </a>
              ) : (
                <a
                  className="ff-sidenav-brand"
                  href="/preview"
                  onClick={goHome}
                >
                  <img
                    className="ff-sidenav-logo"
                    src="/frameflow_logo_sidenav.png"
                    alt="巡帧 FrameFlow"
                    width={248}
                    height={59}
                  />
                </a>
              )
            }
            footer={<AccountMenu />}
          >
            <SideNavSection title="监控">
              <NavItems
                items={monitorNav}
                pathname={pathname}
                canAccess={hasMenuAccess}
              />
            </SideNavSection>
            <Divider />
            <SideNavSection title="核查">
              <NavItems
                items={eventNav}
                pathname={pathname}
                canAccess={hasMenuAccess}
              />
            </SideNavSection>
            <Divider />
            <SideNavSection title="报警与联动">
              <NavItems
                items={linkageNav}
                pathname={pathname}
                canAccess={hasMenuAccess}
              />
            </SideNavSection>
            <Divider />
            <SideNavSection title="设备">
              <NavItems
                items={deviceNav}
                pathname={pathname}
                canAccess={hasMenuAccess}
              />
            </SideNavSection>
            <Divider />
            <SideNavSection title="AI">
              <NavItems
                items={aiNav}
                pathname={pathname}
                canAccess={hasMenuAccess}
              />
            </SideNavSection>
            <Divider />
            <SideNavSection title="系统">
              <NavItems
                items={opsNav}
                pathname={pathname}
                canAccess={hasMenuAccess}
              />
            </SideNavSection>
          </SideNav>
        }
      >
        {children}
      </AppShell>
    </ShellNavProvider>
  );
}

export default ShellSideNav;
