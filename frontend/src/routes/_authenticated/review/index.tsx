import { createFileRoute } from '@tanstack/react-router'
import { ReviewPage } from '@/features/review/review-page'

export const Route = createFileRoute('/_authenticated/review/')({
  component: ReviewPage,
})
