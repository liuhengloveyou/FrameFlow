import { Badge } from '@astryxdesign/core/Badge'
import { HStack } from '@astryxdesign/core/HStack'
import { StatusDot } from '@astryxdesign/core/StatusDot'
import { Text } from '@astryxdesign/core/Text'
import type { CameraStatus, ModelLicenseStatus, WorkflowStatus } from '@/types/api'

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

const licenseLabel: Record<ModelLicenseStatus, string> = {
  authorized: '已授权',
  expired: '已到期',
  disabled: '已关闭',
  unavailable: '不可用',
}

const workflowLabel: Record<WorkflowStatus, string> = {
  idle: '空闲',
  running: '运行中',
  stopped: '已停止',
  error: '异常',
}

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

export function LicenseStatusBadge({ status }: { status: ModelLicenseStatus }) {
  const variant =
    status === 'authorized'
      ? 'success'
      : status === 'expired' || status === 'disabled'
        ? 'error'
        : 'warning'
  return <Badge label={licenseLabel[status]} variant={variant} />
}

export function WorkflowStatusBadge({ status }: { status: WorkflowStatus }) {
  const variant =
    status === 'running' ? 'success' : status === 'error' ? 'error' : 'neutral'
  return (
    <span>
      <span className="ff-status-dot" data-status={status} />
      <Badge label={workflowLabel[status]} variant={variant} />
    </span>
  )
}
