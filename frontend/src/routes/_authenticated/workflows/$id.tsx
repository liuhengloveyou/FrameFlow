import { createFileRoute } from '@tanstack/react-router'
import { WorkflowCanvasPage } from '@/features/workflows/workflow-canvas-page'

export const Route = createFileRoute('/_authenticated/workflows/$id')({
  component: WorkflowCanvasPage,
})
