import { HStack } from '@astryxdesign/core/HStack'
import { StatusDot } from '@astryxdesign/core/StatusDot'
import { Text } from '@astryxdesign/core/Text'
import { ToneBadge, type Tone } from './tone-badge'
import type {
  AlarmLevel,
  AlarmStatus,
  CameraStatus,
  ModelLicenseStatus,
  NotifyDeliveryStatus,
  WorkflowStatus,
} from '@/types/api'

const cameraLabel: Record<CameraStatus, string> = {
  online: '在线',
  offline: '离线',
  error: '异常',
  running: '运行中',
  stopped: '已停止',
}

const cameraDot: Record<
  CameraStatus,
  'success' | 'warning' | 'error' | 'accent' | 'neutral'
> = {
  online: 'success',
  running: 'accent',
  offline: 'neutral',
  stopped: 'neutral',
  error: 'error',
}

/** 设备/通道状态：圆点 + 文字（圆点带呼吸动画） */
export function CameraStatusBadge({ status }: { status: CameraStatus }) {
  const label = cameraLabel[status]
  return (
    <HStack gap={2} vAlign="center">
      <StatusDot
        variant={cameraDot[status]}
        label={label}
        isPulsing={status === 'online' || status === 'running'}
      />
      <Text type="supporting">{label}</Text>
    </HStack>
  )
}

const licenseMeta: Record<ModelLicenseStatus, { text: string; tone: Tone }> = {
  authorized: { text: '已授权', tone: 'ok' },
  expired: { text: '已到期', tone: 'danger' },
  disabled: { text: '已关闭', tone: 'off' },
  unavailable: { text: '不可用', tone: 'warn' },
}

export function LicenseStatusBadge({ status }: { status: ModelLicenseStatus }) {
  const meta = licenseMeta[status] ?? { text: status, tone: 'off' as Tone }
  return <ToneBadge tone={meta.tone}>{meta.text}</ToneBadge>
}

const workflowMeta: Record<WorkflowStatus, { text: string; tone: Tone }> = {
  running: { text: '运行中', tone: 'ok' },
  idle: { text: '空闲', tone: 'info' },
  stopped: { text: '已停止', tone: 'off' },
  error: { text: '异常', tone: 'danger' },
}

export function WorkflowStatusBadge({ status }: { status: WorkflowStatus }) {
  const meta = workflowMeta[status] ?? { text: status, tone: 'off' as Tone }
  return <ToneBadge tone={meta.tone}>{meta.text}</ToneBadge>
}

const alarmLevelMeta: Record<AlarmLevel, { text: string; tone: Tone }> = {
  critical: { text: '严重', tone: 'danger' },
  warn: { text: '警告', tone: 'warn' },
  info: { text: '提示', tone: 'info' },
}

export function AlarmLevelBadge({ level }: { level: AlarmLevel | string }) {
  const meta =
    alarmLevelMeta[level as AlarmLevel] ?? { text: level as string, tone: 'off' as Tone }
  return <ToneBadge tone={meta.tone}>{meta.text}</ToneBadge>
}

const alarmStatusMeta: Record<AlarmStatus, { text: string; tone: Tone }> = {
  open: { text: '未处理', tone: 'danger' },
  ack: { text: '已确认', tone: 'warn' },
  closed: { text: '已关闭', tone: 'ok' },
}

export function AlarmStatusBadge({ status }: { status: AlarmStatus | string }) {
  const meta =
    alarmStatusMeta[status as AlarmStatus] ?? { text: status as string, tone: 'off' as Tone }
  return <ToneBadge tone={meta.tone}>{meta.text}</ToneBadge>
}

const deliveryMeta: Record<NotifyDeliveryStatus, { text: string; tone: Tone }> = {
  success: { text: '成功', tone: 'ok' },
  failed: { text: '失败', tone: 'danger' },
  pending: { text: '排队', tone: 'warn' },
}

export function DeliveryStatusBadge({ status }: { status: NotifyDeliveryStatus | string }) {
  const meta =
    deliveryMeta[status as NotifyDeliveryStatus] ??
    ({ text: status as string, tone: 'off' } as { text: string; tone: Tone })
  return <ToneBadge tone={meta.tone}>{meta.text}</ToneBadge>
}

/** 启用 / 停用（用户、渠道等开关型状态） */
export function EnabledBadge({ enabled }: { enabled: boolean }) {
  return <ToneBadge tone={enabled ? 'ok' : 'off'}>{enabled ? '已启用' : '已停用'}</ToneBadge>
}
