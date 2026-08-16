import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Button } from '@astryxdesign/core/Button'
import { TextInput } from '@astryxdesign/core/TextInput'
import { Switch } from '@astryxdesign/core/Switch'
import { VStack } from '@astryxdesign/core/VStack'
import { Text } from '@astryxdesign/core/Text'
import { useToast } from '@astryxdesign/core/Toast'
import {
  getOnvifSettings,
  updateOnvifSettings,
  type OnvifSettings,
} from '@/apis/onvif-api'
import { getErrorMessage } from '@/lib/http'

/** ONVIF 协议配置（对齐 RTSP/RTMP 协议配置页风格） */
export function OnvifSettingsPanel() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const query = useQuery({
    queryKey: ['onvif-settings'],
    queryFn: getOnvifSettings,
  })

  const [serverEnabled, setServerEnabled] = useState(true)
  const [defaultUsername, setDefaultUsername] = useState('')
  const [defaultPassword, setDefaultPassword] = useState('')
  const [passwordTouched, setPasswordTouched] = useState(false)
  const [discoverTimeoutSec, setDiscoverTimeoutSec] = useState('5')

  useEffect(() => {
    const s = query.data
    if (!s) return
    setServerEnabled(s.serverEnabled)
    setDefaultUsername(s.defaultUsername ?? '')
    setDefaultPassword('')
    setPasswordTouched(false)
    setDiscoverTimeoutSec(String(s.discoverTimeoutSec || 5))
  }, [query.data])

  const timeoutNumber = Number(discoverTimeoutSec)
  const timeoutError = !discoverTimeoutSec.trim()
    ? '超时不能为空'
    : Number.isNaN(timeoutNumber)
      ? '须为数字'
      : timeoutNumber < 1 || timeoutNumber > 60
        ? '范围 1–60 秒'
        : ''
  const isSaveDisabled = Boolean(timeoutError)

  const saveMut = useMutation({
    mutationFn: () => {
      const patch: Parameters<typeof updateOnvifSettings>[0] = {
        serverEnabled,
        defaultUsername: defaultUsername.trim(),
        discoverTimeoutSec: Number(discoverTimeoutSec) || 5,
      }
      if (passwordTouched) {
        patch.defaultPassword = defaultPassword
      }
      return updateOnvifSettings(patch)
    },
    onSuccess: async (_s: OnvifSettings) => {
      toast({ body: 'ONVIF 配置已保存', type: 'info' })
      setDefaultPassword('')
      setPasswordTouched(false)
      await queryClient.invalidateQueries({ queryKey: ['onvif-settings'] })
    },
    onError: (err) => toast({ body: getErrorMessage(err), type: 'error' }),
  })

  const s = query.data

  return (
    <div className="ff-rtmp-settings">
      <Text type="supporting" color="secondary">
        ONVIF 客户端：设备页添加摄像头时扫描局域网并 GetStreamUri 转 RTSP 拉流。
        ONVIF 服务端：把本机已有码流暴露给第三方 ONVIF 客户端（Hass / ODM 等）。
      </Text>

      <VStack gap={4} className="ff-rtmp-settings-form">
        <div className="ff-rtmp-field">
          <Text type="label">
            <span className="ff-req">*</span> ONVIF 服务端
          </Text>
          <Text type="supporting" color="secondary">
            开启后对外提供 {s?.serverPath || '/onvif/'}（Device / Media）。关闭则拒绝 SOAP
            请求。
          </Text>
          <Switch
            label="ONVIF 服务端"
            value={serverEnabled}
            onChange={setServerEnabled}
          />
        </div>

        <div className="ff-rtmp-field">
          <Text type="label">
            <span className="ff-req">*</span> 发现超时（秒）
          </Text>
          <Text type="supporting" color="secondary">
            设备页「扫描局域网」的 WS-Discovery 等待时间。
          </Text>
          <TextInput
            label="发现超时"
            isLabelHidden
            isRequired
            status={timeoutError ? { type: 'error', message: timeoutError } : undefined}
            value={discoverTimeoutSec}
            onChange={setDiscoverTimeoutSec}
          />
        </div>

        <div className="ff-rtmp-field">
          <Text type="label">发现默认账号（可选）</Text>
          <Text type="supporting" color="secondary">
            扫描选中设备时自动填入用户名；密码仅保存在服务端，选中后仍需在设备表单确认。
            {s?.hasDefaultPassword && !passwordTouched
              ? ' 当前已设置默认密码（留空保存则不改动）。'
              : ''}
          </Text>
          <TextInput
            label="默认用户名"
            isLabelHidden
            placeholder="例如 admin"
            value={defaultUsername}
            onChange={setDefaultUsername}
          />
          <TextInput
            label="默认密码"
            isLabelHidden
            type="password"
            placeholder={s?.hasDefaultPassword ? '已设置，留空不改' : '可选'}
            value={defaultPassword}
            onChange={(v) => {
              setDefaultPassword(v)
              setPasswordTouched(true)
            }}
          />
        </div>

        <div className="ff-rtmp-field">
          <Text type="label">客户端示例</Text>
          <Text type="supporting" style={{ wordBreak: 'break-all' }}>
            {s?.clientExample || 'onvif://user:pass@192.168.1.100?subtype=0'}
          </Text>
        </div>

        <Button
          label={saveMut.isPending ? '保存中…' : '保存'}
          variant="primary"
          isDisabled={isSaveDisabled || saveMut.isPending}
          onClick={() => saveMut.mutate()}
        />
      </VStack>
    </div>
  )
}
