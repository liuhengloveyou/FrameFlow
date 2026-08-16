import { createFileRoute } from '@tanstack/react-router'
import { RecordingPage } from '@/features/recording/recording-page'

export const Route = createFileRoute('/_authenticated/recording/')({
  component: RecordingPage,
})
