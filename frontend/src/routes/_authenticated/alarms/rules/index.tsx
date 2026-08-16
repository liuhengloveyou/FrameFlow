import { createFileRoute } from '@tanstack/react-router'
import { LinkageRulesPage } from '@/features/alarms/linkage-rules-page'

export const Route = createFileRoute('/_authenticated/alarms/rules/')({
  component: LinkageRulesPage,
})
