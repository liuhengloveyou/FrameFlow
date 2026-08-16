import { useNavigate, useRouterState } from '@tanstack/react-router'
import {
  SegmentedControl,
  SegmentedControlItem,
} from '@astryxdesign/core/SegmentedControl'

type LinkageTab = 'records' | 'rules' | 'workflows'

function tabFromPath(pathname: string): LinkageTab {
  if (pathname.startsWith('/alarms/rules')) return 'rules'
  if (pathname.startsWith('/workflows')) return 'workflows'
  return 'records'
}

/** 报警记录 / 联动规则 / 工作流 共用子导航 */
export function AlarmLinkageSubnav() {
  const navigate = useNavigate()
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  const tab = tabFromPath(pathname)

  return (
    <div className="ff-alarm-linkage-subnav">
      <SegmentedControl
        label="报警与联动"
        layout="hug"
        value={tab}
        onChange={(v) => {
          const next = (v as LinkageTab) || 'records'
          if (next === 'rules') {
            void navigate({ to: '/alarms/rules' })
            return
          }
          if (next === 'workflows') {
            void navigate({ to: '/workflows' })
            return
          }
          void navigate({ to: '/alarms' })
        }}
      >
        <SegmentedControlItem value="records" label="报警记录" />
        <SegmentedControlItem value="rules" label="联动规则" />
        <SegmentedControlItem value="workflows" label="工作流" />
      </SegmentedControl>
    </div>
  )
}
