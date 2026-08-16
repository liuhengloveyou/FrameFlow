import { createFileRoute } from '@tanstack/react-router'
import { DeviceAiPage } from '@/features/ai-config/device-ai-page'

export const Route = createFileRoute(
  '/_authenticated/devices/$id/channels/$channelId/ai',
)({
  component: StreamAiRoute,
})

function StreamAiRoute() {
  const { id, channelId } = Route.useParams()
  return <DeviceAiPage cameraId={id} channelId={channelId} />
}
