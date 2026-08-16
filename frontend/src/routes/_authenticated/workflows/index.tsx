import { createFileRoute } from '@tanstack/react-router'
import { WorkflowsPage } from '@/features/workflows/workflows-page'

export const Route = createFileRoute('/_authenticated/workflows/')({
  component: WorkflowsPage,
})
