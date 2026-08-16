import type { ReactNode } from 'react'
import { Button } from '@astryxdesign/core/Button'

type SidePanelProps = {
  open: boolean
  title: ReactNode
  onClose: () => void
  children: ReactNode
  footer?: ReactNode
  width?: string | number
}

/** 统一右侧抽屉（backdrop + panel） */
export function SidePanel({ open, title, onClose, children, footer, width }: SidePanelProps) {
  if (!open) return null
  return (
    <>
      <div className="ff-side-panel-backdrop" onClick={onClose} />
      <aside className="ff-side-panel" style={width ? { width } : undefined}>
        <div className="ff-side-panel-header">
          <strong>{title}</strong>
          <Button label="关闭" size="sm" variant="ghost" onClick={onClose} />
        </div>
        <div className="ff-side-panel-body">{children}</div>
        {footer ? <div className="ff-side-panel-footer">{footer}</div> : null}
      </aside>
    </>
  )
}
