import { forwardRef } from 'react'
import { Link } from '@tanstack/react-router'

type RouterLinkProps = {
  href: string
  className?: string
  style?: React.CSSProperties
  children?: React.ReactNode
} & Omit<React.AnchorHTMLAttributes<HTMLAnchorElement>, 'href'>

/** Astryx LinkProvider：把 href 映射为 TanStack Router 的 to */
export const RouterLink = forwardRef<HTMLAnchorElement, RouterLinkProps>(
  function RouterLink({ href, children, ...rest }, ref) {
    return (
      <Link
        ref={ref}
        to={href}
        // 侧栏选中由 SideNavItem.isSelected 控制；禁止 /system 在 /system/protocols 下被模糊高亮
        activeOptions={{ exact: true, includeSearch: false }}
        {...rest}
      >
        {children}
      </Link>
    )
  },
)
