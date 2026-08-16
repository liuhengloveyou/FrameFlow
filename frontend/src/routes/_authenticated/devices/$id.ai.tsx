import { useEffect } from 'react'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { getCamera } from '@/apis/devices-api'
import { resolveAiChannelId } from '@/lib/ai-stream'

/** 旧入口 /devices/$id/ai → 跳转到默认视频 AI 页 */
export const Route = createFileRoute('/_authenticated/devices/$id/ai')({
  component: DeviceAiLegacyRedirect,
})

function DeviceAiLegacyRedirect() {
  const { id } = Route.useParams()
  const navigate = useNavigate()
  const cameraQuery = useQuery({
    queryKey: ['camera', id],
    queryFn: () => getCamera(id),
  })

  useEffect(() => {
    if (!cameraQuery.data) return
    const channelId = resolveAiChannelId(cameraQuery.data)
    void navigate({
      to: '/devices/$id/channels/$channelId/ai',
      params: { id, channelId },
      replace: true,
    })
  }, [cameraQuery.data, id, navigate])

  return null
}
