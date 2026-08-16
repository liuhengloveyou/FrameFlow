import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Button } from '@astryxdesign/core/Button'
import { TextInput } from '@astryxdesign/core/TextInput'
import { Switch } from '@astryxdesign/core/Switch'
import { VStack } from '@astryxdesign/core/VStack'
import { Text } from '@astryxdesign/core/Text'
import { useToast } from '@astryxdesign/core/Toast'
import {
  getRtspSettings,
  updateRtspSettings,
  checkRtspPort,
  type PortCheckResult,
  type RtspSettings,
} from '@/apis/rtsp-api'
import { getErrorMessage } from '@/lib/http'

/** RTSP 协议配置表单（作为「协议配置」页的一个 Tab） */
export function RtspSettingsPanel() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const query = useQuery({
    queryKey: ['rtsp-settings'],
    queryFn: getRtspSettings,
  })

  const [enabled, setEnabled] = useState(true)
  const [listenPort, setListenPort] = useState('8554')
  const [advertiseHost, setAdvertiseHost] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [passwordTouched, setPasswordTouched] = useState(false)
  const [defaultQuery, setDefaultQuery] = useState('video&audio')
  const [portCheck, setPortCheck] = useState<PortCheckResult | null>(null)
  const [isCheckingPort, setIsCheckingPort] = useState(false)

  useEffect(() => {
    const s = query.data
    if (!s) return
    setEnabled(s.enabled)
    setListenPort(String(s.listenPort || 8554))
    setAdvertiseHost(s.advertiseHost ?? '')
    setUsername(s.username ?? '')
    setPassword('')
    setPasswordTouched(false)
    setDefaultQuery(s.defaultQuery || 'video&audio')
  }, [query.data])

  const listenPortNumber = Number(listenPort)
  const listenPortError = !listenPort.trim()
    ? '端口不能为空'
    : Number.isNaN(listenPortNumber)
    ? '端口必须是数字'
    : listenPortNumber < 1 || listenPortNumber > 65535
    ? '端口范围 1-65535'
    : ''
  const authError = passwordTouched && !username.trim()
    ? '设置密码时需填写用户名'
    : ''
  const defaultQueryError = !defaultQuery.trim()
    ? '默认媒体查询不能为空'
    : ''
  const isSaveDisabled = Boolean(listenPortError || authError || defaultQueryError)

  const saveMut = useMutation({
    mutationFn: () => {
      const patch: Parameters<typeof updateRtspSettings>[0] = {
        enabled,
        listenPort: Number(listenPort) || 8554,
        advertiseHost: advertiseHost.trim(),
        username: username.trim(),
        defaultQuery: defaultQuery.trim() || 'video&audio',
      }
      if (passwordTouched) {
        patch.password = password
      }
      return updateRtspSettings(patch)
    },
    onSuccess: async (s: RtspSettings) => {
      toast({
        body: s.runningListen
          ? `已保存，RTSP 正在监听 ${s.runningListen}`
          : s.enabled
            ? '已保存，但监听未启动（请检查端口）'
            : '已保存，RTSP 监听已关闭',
        type: 'info',
      })
      setPassword('')
      setPasswordTouched(false)
      await queryClient.invalidateQueries({ queryKey: ['rtsp-settings'] })
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
  const previewPort = Number(listenPort) || 8554
  const authPrefix = username.trim() ? `${username.trim()}@` : ''
  const previewUrl = `rtsp://${authPrefix}${previewHost}:${previewPort}/<stream>`

  return (
    <div className="ff-rtmp-settings">
      <Text type="supporting" color="secondary">
        配置本机 RTSP 服务（对外播放 / 拉流出口）。添加摄像头时的 RTSP
        拉流仍填写远端摄像头地址；本页只管本机监听。
      </Text>

      <VStack gap={4} className="ff-rtmp-settings-form">
        <div className="ff-rtmp-field">
          <Text type="label">
            <span className="ff-req">*</span> RTSP 服务
          </Text>
          <Switch
            label="RTSP 服务"
            value={enabled}
            onChange={setEnabled}
          />
        </div>

        <div className="ff-rtmp-field">
          <Text type="label">
            <span className="ff-req">*</span> 服务端口
          </Text>
          <Text type="supporting" color="secondary">
            保存后立即生效：开启则绑定端口，关闭则释放监听；进程启动时按已保存配置自动恢复监听。
          </Text>
          <div className="ff-port-check-row">
            <TextInput
              label="端口"
              isLabelHidden
              isRequired
              isDisabled={!enabled}
              disabledMessage={!enabled ? 'RTSP 服务已关闭' : undefined}
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
                  const result = await checkRtspPort(listenPortNumber)
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
          <Text type="label">对外播放主机</Text>
          <Text type="supporting" color="secondary">
            其它设备访问本机 RTSP 时用的 IP / 域名。留空则自动选用本机网卡。
          </Text>
          <TextInput
            label="对外播放主机"
            isLabelHidden
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
          <Text type="label">访问鉴权（可选）</Text>
          <Text type="supporting" color="secondary">
            设置后，非本机回环的客户端需带用户名密码。本机访问免鉴权。
            {s?.hasPassword && !passwordTouched
              ? ' 当前已设置密码（留空保存则不改动）。'
              : ''}
          </Text>
          <TextInput
            label="用户名"
            isLabelHidden
            placeholder="用户名"
            value={username}
            onChange={setUsername}
          />
          <TextInput
            label="密码"
            isLabelHidden
            type="password"
            placeholder={s?.hasPassword ? '••••••••（输入则更新）' : '密码'}
            status={authError ? { type: 'error', message: authError } : undefined}
            value={password}
            onChange={(v) => {
              setPassword(v)
              setPasswordTouched(true)
            }}
          />
        </div>

        <div className="ff-rtmp-field">
          <Text type="label">默认媒体查询</Text>
          <Text type="supporting" color="secondary">
            播放未指定 track 时的默认 query，一般保持 video&audio。
          </Text>
          <TextInput
            label="default_query"
            isLabelHidden
            status={defaultQueryError ? { type: 'error', message: defaultQueryError } : undefined}
            value={defaultQuery}
            onChange={setDefaultQuery}
          />
        </div>

        <div className="ff-rtmp-field">
          <Text type="label">播放地址预览</Text>
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
                ? "（配置为开启；若未监听请检查端口占用或重新保存）"
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
