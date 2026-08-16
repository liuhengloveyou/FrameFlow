import { createFileRoute } from '@tanstack/react-router'
import { AlgorithmsPage } from '@/features/algorithms/algorithms-page'

export const Route = createFileRoute('/_authenticated/algorithms/')({
  component: AlgorithmsPage,
})
