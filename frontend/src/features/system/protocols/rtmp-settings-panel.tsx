import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Button } from '@astryxdesign/core/Button'
import { TextInput } from '@astryxdesign/core/TextInput'
import { Switch } from '@astryxdesign/core/Switch'
import { VStack } from '@astryxdesign/core/VStack'
import { Text } from '@astryxdesign/core/Text'
import { useToast } from '@astryxdesign/core/Toast'
import {
  getRtmpSettings,
  updateRtmpSettings,
  checkRtmpPort,
  type PortCheckResult,
  type RtmpSettings,
} from '@/apis/rtmp-api'
import { getErrorMessage } from '@/lib/http'

/** RTMP 协议配置表单（作为「协议配置」页的一个 Tab） */
export function RtmpSettingsPanel() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const query = useQuery({
    queryKey: ['rtmp-settings'],
    queryFn: getRtmpSettings,
  })

  const [enabled, setEnabled] = useState(true)
  const [listenPort, setListenPort] = useState('1935')
  const [advertiseHost, setAdvertiseHost] = useState('')
  const [portCheck, setPortCheck] = useState<PortCheckResult | null>(null)
  const [isCheckingPort, setIsCheckingPort] = useState(false)

  useEffect(() => {
    const s = query.data
    if (!s) return
    setEnabled(s.enabled)
    setListenPort(String(s.listenPort || 1935))
    setAdvertiseHost(s.advertiseHost ?? '')
  }, [query.data])

  const listenPortNumber = Number(listenPort)
  const listenPortError = !listenPort.trim()
    ? '端口不能为空'
    : Number.isNaN(listenPortNumber)
    ? '端口必须是数字'
    : listenPortNumber < 1 || listenPortNumber > 65535
    ? '端口范围 1-65535'
    : ''
  const isSaveDisabled = Boolean(listenPortError)

  const saveMut = useMutation({
    mutationFn: () =>
      updateRtmpSettings({
        enabled,
        listenPort: listenPortNumber || 1935,
        advertiseHost: advertiseHost.trim(),
      }),
    onSuccess: async (s: RtmpSettings) => {
      toast({
        body: s.runningListen
          ? `已保存，RTMP 正在监听 ${s.runningListen}`
          : s.enabled
            ? '已保存，但监听未启动（请检查端口）'
            : '已保存，RTMP 监听已关闭',
        type: 'info',
      })
      await queryClient.invalidateQueries({ queryKey: ['rtmp-settings'] })
    },
    onError: (err) => toast({ body: getErrorMessage(err), type: 'error' }),
  })

  const s = query.data
  const previewHost =
    advertiseHost.trim() || s?.candidateHosts?.[0] || '127.0.0.1'

  const portCheckMessage = portCheck?.message ?? ''
  const portCheckStatus = portCheck
    ? portCheck.available
      ? { type: 'success' as const, message: portCheckMessage }
      : { type: 'error' as const, message: portCheckMessage }
    : undefined
  const previewPort = Number(listenPort) || 1935
  const previewUrl = `rtmp://${previewHost}:${previewPort}/<stream_key>`

  return (
    <div className="ff-rtmp-settings">
      <Text type="supporting" color="secondary">
        在此配置本机 RTMP 收流参数。添加摄像头选择「推流」时会直接使用这里的主机与端口，无需再填。
      </Text>

      <VStack gap={4} className="ff-rtmp-settings-form">
        <div className="ff-rtmp-field">
          <Text type="label">
            <span className="ff-req">*</span> RTMP 服务
          </Text>
          <Switch
            label="RTMP 服务"
            value={enabled}
            onChange={setEnabled}
          />
        </div>

        <div className="ff-rtmp-field">
          <Text type="label">
            <span className="ff-req">*</span> 服务端口
          </Text>
          <Text type="supporting" color="secondary">
            保存后立即生效：开启则绑定端口，关闭则释放监听；进程启动时也会按已保存配置自动监听。
          </Text>
          <div className="ff-port-check-row">
            <TextInput
              label="端口"
              isLabelHidden
              isRequired
              isDisabled={!enabled}
              disabledMessage={!enabled ? 'RTMP 服务已关闭' : undefined}
              status={listenPortError ? { type: 'error', message: listenPortError } : portCheckStatus}
              value={listenPort}
              onChange={(value) => {
                setListenPort(value)
                setPortCheck(null)
              }}
            />
            <Button
              label={isCheckingPort ? '检测中…' : '检测端口'}
              variant="secondary"
              isDisabled={!enabled || Boolean(listenPortError) || isCheckingPort}
              clickAction={async () => {
                setPortCheck(null)
                setIsCheckingPort(true)
                try {
                  const result = await checkRtmpPort(listenPortNumber)
                  setPortCheck(result)
                } catch (error) {
                  setPortCheck({
                    port: listenPortNumber,
                    available: false,
                    message: getErrorMessage(error),
                  })
                } finally {
                  setIsCheckingPort(false)
                }
              }}
            />
          </div>
        </div>

        <div className="ff-rtmp-field">
          <Text type="label">
            <span className="ff-req">*</span> 对外推流主机
          </Text>
          <Text type="supporting" color="secondary">
            摄像头 / OBS 要能访问到的 NAS IP 或域名。留空则自动选用本机网卡地址。
          </Text>
          <TextInput
            label="对外推流主机"
            isLabelHidden
            isRequired
            placeholder={s?.candidateHosts?.[0] || '例如 192.168.1.10'}
            value={advertiseHost}
            onChange={setAdvertiseHost}
          />
          {s?.candidateHosts?.length ? (
            <div className="ff-rtmp-host-chips">
              <Text type="supporting" color="secondary">
                本机网卡：
              </Text>
              {s.candidateHosts.map((h) => (
                <button
                  key={h}
                  type="button"
                  className="ff-rtmp-host-chip"
                  onClick={() => setAdvertiseHost(h)}
                >
                  {h}
                </button>
              ))}
            </div>
          ) : null}
        </div>

        <div className="ff-rtmp-field">
          <Text type="label">推流地址预览</Text>
          <Text type="supporting" style={{ wordBreak: 'break-all' }}>
            {previewUrl}
          </Text>
          {s?.runningListen ? (
            <Text type="supporting" color="secondary">
              当前进程监听：{s.runningListen}
            </Text>
          ) : (
            <Text type="supporting" color="secondary">
              当前未监听
              {s?.enabled
                ? "（配置为开启；进程重启后请再点保存以启动）"
                : "（打开上方开关并保存即可启动）"}
            </Text>
          )}
          {s?.restartHint ? (
            <Text type="supporting" color="secondary">
              {s.restartHint}
            </Text>
          ) : null}
        </div>

        <Button
          label="保存配置"
          variant="primary"
          isLoading={saveMut.isPending}
          isDisabled={saveMut.isPending || isSaveDisabled}
          clickAction={async () => {
            await saveMut.mutateAsync()
          }}
        />
      </VStack>
    </div>
  )
}
