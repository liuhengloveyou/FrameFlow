import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Button } from '@astryxdesign/core/Button'
import { TextInput } from '@astryxdesign/core/TextInput'
import { Switch } from '@astryxdesign/core/Switch'
import { VStack } from '@astryxdesign/core/VStack'
import { Text } from '@astryxdesign/core/Text'
import { useToast } from '@astryxdesign/core/Toast'
import {
  checkGb28181Port,
  getGb28181Settings,
  updateGb28181Settings,
  type Gb28181Settings,
  type PortCheckResult,
} from '@/apis/gb28181-api'
import { getErrorMessage } from '@/lib/http'

/** 国标平台 SIP/媒体配置（「协议配置」页 Tab） */
export function Gb28181SettingsPanel() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const query = useQuery({
    queryKey: ['gb28181-settings'],
    queryFn: getGb28181Settings,
  })

  const [enabled, setEnabled] = useState(false)
  const [listenPort, setListenPort] = useState('5060')
  const [serial, setSerial] = useState('')
  const [realm, setRealm] = useState('')
  const [password, setPassword] = useState('')
  const [sipIp, setSipIp] = useState('')
  const [mediaIp, setMediaIp] = useState('')
  const [mediaPort, setMediaPort] = useState('10000-20000')
  const [transport, setTransport] = useState('udp')
  const [allowUnknown, setAllowUnknown] = useState(false)
  const [portCheck, setPortCheck] = useState<PortCheckResult | null>(null)
  const [isCheckingPort, setIsCheckingPort] = useState(false)

  useEffect(() => {
    const s = query.data
    if (!s) return
    setEnabled(s.enabled)
    setListenPort(String(s.listenPort || 5060))
    setSerial(s.serial ?? '')
    setRealm(s.realm ?? '')
    setPassword(s.password ?? '')
    setSipIp(s.sipIp ?? '')
    setMediaIp(s.mediaIp ?? '')
    setMediaPort(s.mediaPort || '10000-20000')
    setTransport(s.transport || 'udp')
    setAllowUnknown(Boolean(s.allowUnknown))
  }, [query.data])

  const listenPortNumber = Number(listenPort)
  const listenPortError = !listenPort.trim()
    ? '端口不能为空'
    : Number.isNaN(listenPortNumber)
      ? '端口必须是数字'
      : listenPortNumber < 1 || listenPortNumber > 65535
        ? '端口范围 1-65535'
        : ''
  const serialError = !serial.trim() ? '平台国标编码不能为空' : ''
  const realmError = !realm.trim() ? 'Realm 不能为空' : ''
  const isSaveDisabled = Boolean(listenPortError || serialError || realmError)

  const saveMut = useMutation({
    mutationFn: () =>
      updateGb28181Settings({
        enabled,
        listenPort: listenPortNumber || 5060,
        serial: serial.trim(),
        realm: realm.trim(),
        password,
        sipIp: sipIp.trim(),
        mediaIp: mediaIp.trim(),
        mediaPort: mediaPort.trim() || '10000-20000',
        transport: transport.trim() || 'udp',
        allowUnknown,
      }),
    onSuccess: async (s: Gb28181Settings) => {
      toast({
        body: s.runningListen
          ? `已保存，国标 SIP 正在监听 ${s.runningListen}`
          : s.enabled
            ? '已保存，但 SIP 未启动（请检查端口与日志）'
            : '已保存，国标 SIP 已关闭',
        type: 'info',
      })
      await queryClient.invalidateQueries({ queryKey: ['gb28181-settings'] })
    },
    onError: (err) => toast({ body: getErrorMessage(err), type: 'error' }),
  })

  const s = query.data
  const portCheckMessage = portCheck?.message ?? ''
  const portCheckStatus = portCheck
    ? portCheck.available
      ? { type: 'success' as const, message: portCheckMessage }
      : { type: 'error' as const, message: portCheckMessage }
    : undefined

  return (
    <div className="ff-rtmp-settings">
      <Text type="supporting" color="secondary">
        配置本机作为 GB/T 28181 平台的 SIP/媒体参数。设备仍在「设备管理」中登记；此处只管平台监听与鉴权。
      </Text>

      <VStack gap={4} className="ff-rtmp-settings-form">
        <div className="ff-rtmp-field">
          <Text type="label">
            <span className="ff-req">*</span> 国标 SIP 服务
          </Text>
          <Switch label="国标 SIP 服务" value={enabled} onChange={setEnabled} />
        </div>

        <div className="ff-rtmp-field">
          <Text type="label">
            <span className="ff-req">*</span> SIP 端口
          </Text>
          <Text type="supporting" color="secondary">
            保存后立即生效：开启则绑定端口，关闭则停止 SIP；进程启动时按已保存配置自动监听。
          </Text>
          <div className="ff-port-check-row">
            <TextInput
              label="端口"
              isLabelHidden
              isRequired
              isDisabled={!enabled}
              disabledMessage={!enabled ? '国标 SIP 已关闭' : undefined}
              status={
                listenPortError
                  ? { type: 'error', message: listenPortError }
                  : portCheckStatus
              }
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
                  setPortCheck(await checkGb28181Port(listenPortNumber))
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
            <span className="ff-req">*</span> 平台国标编码
          </Text>
          <TextInput
            label="平台国标编码"
            isLabelHidden
            isRequired
            status={serialError ? { type: 'error', message: serialError } : undefined}
            value={serial}
            onChange={setSerial}
          />
        </div>

        <div className="ff-rtmp-field">
          <Text type="label">
            <span className="ff-req">*</span> Realm
          </Text>
          <TextInput
            label="Realm"
            isLabelHidden
            isRequired
            status={realmError ? { type: 'error', message: realmError } : undefined}
            value={realm}
            onChange={setRealm}
          />
        </div>

        <div className="ff-rtmp-field">
          <Text type="label">平台 Digest 密码</Text>
          <Text type="supporting" color="secondary">
            留空表示不校验设备注册 Digest（仅建议联调使用）。
            {s?.hasPassword ? ' 当前已保存密码。' : ''}
          </Text>
          <TextInput
            label="密码"
            isLabelHidden
            type="password"
            value={password}
            onChange={setPassword}
            placeholder={s?.hasPassword ? '••••••••' : '可选'}
          />
        </div>

        <div className="ff-rtmp-field">
          <Text type="label">SIP 对外 IP</Text>
          <Text type="supporting" color="secondary">
            摄像头能访问到的本机地址；留空则自动探测。
            {s?.resolvedSipIp ? ` 当前解析：${s.resolvedSipIp}` : ''}
          </Text>
          <TextInput
            label="SIP IP"
            isLabelHidden
            value={sipIp}
            onChange={setSipIp}
            placeholder={s?.resolvedSipIp || '自动探测'}
          />
        </div>

        <div className="ff-rtmp-field">
          <Text type="label">媒体 IP</Text>
          <Text type="supporting" color="secondary">
            RTP 媒体面地址；留空则与 SIP IP 相同。
            {s?.resolvedMediaIp ? ` 当前解析：${s.resolvedMediaIp}` : ''}
          </Text>
          <TextInput
            label="媒体 IP"
            isLabelHidden
            value={mediaIp}
            onChange={setMediaIp}
            placeholder={s?.resolvedMediaIp || '同 SIP IP'}
          />
        </div>

        <div className="ff-rtmp-field">
          <Text type="label">媒体端口池</Text>
          <TextInput
            label="媒体端口池"
            isLabelHidden
            value={mediaPort}
            onChange={setMediaPort}
            placeholder="10000-20000"
          />
        </div>

        <div className="ff-rtmp-field">
          <Text type="label">传输方式</Text>
          <Text type="supporting" color="secondary">
            udp 或 tcp-passive
          </Text>
          <TextInput
            label="传输方式"
            isLabelHidden
            value={transport}
            onChange={setTransport}
            placeholder="udp"
          />
        </div>

        <div className="ff-rtmp-field">
          <Text type="label">未知设备策略</Text>
          <Switch
            label="允许未知设备直接 active"
            value={allowUnknown}
            onChange={setAllowUnknown}
          />
          <Text type="supporting" color="secondary">
            关闭时未知设备入库为 pending，需人工确认。
          </Text>
        </div>

        <div className="ff-rtmp-field">
          <Text type="label">运行状态</Text>
          {s?.runningListen ? (
            <Text type="supporting" color="secondary">
              当前进程监听：{s.runningListen}
            </Text>
          ) : (
            <Text type="supporting" color="secondary">
              当前未监听
              {s?.enabled
                ? '（配置为开启；请检查端口或重新保存）'
                : '（打开上方开关并保存即可启动）'}
            </Text>
          )}
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
