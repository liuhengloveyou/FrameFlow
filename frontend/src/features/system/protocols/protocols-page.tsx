import { useNavigate, useSearch } from '@tanstack/react-router'
import { Tab, TabList } from '@astryxdesign/core/TabList'
import { PageLayout } from '@/components/page-layout'
import { Gb28181SettingsPanel } from '@/features/system/protocols/gb28181-settings-panel'
import { OnvifSettingsPanel } from '@/features/system/protocols/onvif-settings-panel'
import { RtmpSettingsPanel } from '@/features/system/protocols/rtmp-settings-panel'
import { RtspSettingsPanel } from '@/features/system/protocols/rtsp-settings-panel'

export type ProtocolTab = 'rtmp' | 'rtsp' | 'onvif' | 'gb28181'

const TABS: { value: ProtocolTab; label: string }[] = [
  { value: 'rtmp', label: 'RTMP' },
  { value: 'rtsp', label: 'RTSP' },
  { value: 'onvif', label: 'ONVIF' },
  { value: 'gb28181', label: 'GB28181' },
]

function panel(tab: ProtocolTab) {
  if (tab === 'rtsp') return <RtspSettingsPanel />
  if (tab === 'onvif') return <OnvifSettingsPanel />
  if (tab === 'gb28181') return <Gb28181SettingsPanel />
  return <RtmpSettingsPanel />
}

/** 系统 → 协议配置：多协议共用一页，Tab 切换 */
export function ProtocolsPage() {
  const navigate = useNavigate()
  const { tab } = useSearch({ from: '/_authenticated/system/protocols/' })
  const active: ProtocolTab =
    tab === 'rtsp' || tab === 'onvif' || tab === 'gb28181' ? tab : 'rtmp'

  return (
    <PageLayout
      title="协议配置"
      badge="收流与发现"
      toolbar={
        <TabList
          value={active}
          onChange={(v) => {
            const next = (v as ProtocolTab) || 'rtmp'
            void navigate({
              to: '/system/protocols',
              search: { tab: next },
              replace: true,
            })
          }}
          size="sm"
          hasDivider
        >
          {TABS.map((t) => (
            <Tab key={t.value} value={t.value} label={t.label} />
          ))}
        </TabList>
      }
    >
      {panel(active)}
    </PageLayout>
  )
}
