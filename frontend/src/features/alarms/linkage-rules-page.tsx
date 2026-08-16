import { Link } from '@tanstack/react-router'
import { Button } from '@astryxdesign/core/Button'
import { HStack } from '@astryxdesign/core/HStack'
import { PageLayout } from '@/components/page-layout'
import { AlarmLinkageSubnav } from './alarm-linkage-subnav'
import { LinkageRulesPanel } from './linkage-rules-panel'

export function LinkageRulesPage() {
  return (
    <PageLayout
      title="联动规则"
      actions={
        <HStack gap={2}>
          <Link to="/workflows">
            <Button label="管理工作流" size="sm" variant="secondary" />
          </Link>
        </HStack>
      }
      toolbar={<AlarmLinkageSubnav />}
    >
      <LinkageRulesPanel />
    </PageLayout>
  )
}
