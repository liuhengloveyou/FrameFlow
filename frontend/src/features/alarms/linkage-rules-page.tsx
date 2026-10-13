import { Link } from '@tanstack/react-router'
import { Button } from '@astryxdesign/core/Button'
import { PageLayout } from '@/components/page-layout'
import { AlarmLinkageSubnav } from './alarm-linkage-subnav'
import { LinkageRulesPanel } from './linkage-rules-panel'

export function LinkageRulesPage() {
  return (
    <PageLayout
      title="联动规则"
      badge="自动匹配"
      actions={
        <Link to="/workflows">
          <Button label="管理工作流" size="sm" variant="secondary" />
        </Link>
      }
      toolbar={<AlarmLinkageSubnav />}
    >
      <LinkageRulesPanel />
    </PageLayout>
  )
}
