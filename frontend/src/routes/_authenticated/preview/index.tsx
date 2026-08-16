import { createFileRoute } from '@tanstack/react-router'
import { PreviewPage } from '@/features/preview/preview-page'

export const Route = createFileRoute('/_authenticated/preview/')({
  component: PreviewRoute,
})

function PreviewRoute() {
  return <PreviewPage />
}
