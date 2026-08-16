import { createFileRoute } from '@tanstack/react-router'
import { ProtocolsPage, type ProtocolTab } from '@/features/system/protocols/protocols-page'

function parseTab(raw: unknown): ProtocolTab {
  if (raw === 'rtsp' || raw === 'onvif' || raw === 'gb28181') return raw
  return 'rtmp'
}

export const Route = createFileRoute('/_authenticated/system/protocols/')({
  validateSearch: (search: Record<string, unknown>): { tab: ProtocolTab } => ({
    tab: parseTab(search.tab),
  }),
  component: ProtocolsPage,
})
