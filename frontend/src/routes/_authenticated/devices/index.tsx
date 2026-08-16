import { createFileRoute } from '@tanstack/react-router'
import { DevicesPage } from '@/features/devices/devices-page'

export const Route = createFileRoute('/_authenticated/devices/')({
  component: DevicesPage,
})
