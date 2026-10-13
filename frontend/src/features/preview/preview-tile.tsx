import { useEffect, useRef, useState, type MouseEvent } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useQueryClient } from '@tanstack/react-query'
import { Text } from '@astryxdesign/core/Text'
import { useToast } from '@astryxdesign/core/Toast'
import {
  ArrowsPointingOutIcon,
  VideoCameraSlashIcon,
} from '@heroicons/react/24/outline'
import { resolveAiChannelId } from '@/lib/ai-stream'
import { Go2rtcPlayer } from '@/lib/go2rtc/go2rtc-player'
import {
  resolveStreamSrc,
  type StreamProfile,
} from '@/lib/preview-source'
import {
  StreamErrors,
  gbDeviceNotReadyHint,
  isGbDeviceNotReady,
} from '@/lib/stream-error'
import { parseVisionPayload, type VisionBox } from '@/lib/vision-types'
import { startCamera } from '@/apis/devices-api'
import type { Camera, CameraStatus } from '@/types/api'
import { DetectionOverlay } from './detection-overlay'

type StreamMode = 'webrtc' | 'normal'
type PlayState = 'connecting' | 'playing' | 'paused' | 'offline' | 'error'

/** 最后在线时间：本地墙钟 HH:MM */
function formatLastSeen(iso?: string): string {
  if (!iso?.trim()) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  const p = (n: number) => String(n).padStart(2, '0')
  return `${p(d.getHours())}:${p(d.getMinutes())}`
}

/** 播放态 + 设备在线态合并为一个角标文案 */
function mergedStatusLabel(
  playState: PlayState,
  cameraStatus: CameraStatus,
): { label: string; tone: PlayState | 'online' | 'device-error' } {
  if (
    cameraStatus === 'offline' ||
    cameraStatus === 'stopped' ||
    cameraStatus === 'error'
  ) {
    if (cameraStatus === 'error') return { label: '异常', tone: 'device-error' }
    if (cameraStatus === 'stopped') return { label: '已停止', tone: 'offline' }
    return { label: '离线', tone: 'offline' }
  }
  // 设备在线：用播放态表达（在线隐含在「播放中/连接中」里）
  if (playState === 'playing') return { label: '播放中', tone: 'playing' }
  if (playState === 'connecting') return { label: '连接中', tone: 'connecting' }
  if (playState === 'paused') return { label: '已暂停', tone: 'paused' }
  if (playState === 'error') return { label: '拉流失败', tone: 'error' }
  return { label: '在线', tone: 'online' }
}

function formatBitrate(kbps: number | null | undefined): string {
  if (kbps == null || kbps <= 0) return '—'
  if (kbps >= 1000) return `${(kbps / 1000).toFixed(1)} Mbps`
  return `${Math.round(kbps)} kbps`
}

function formatResolution(
  width: number,
  height: number,
  fallback?: string,
): string {
  if (width > 0 && height > 0) return `${width}×${height}`
  return fallback?.trim() || ''
}

/** 单路预览：go2rtc WebRTC / MSE；回放页用 variant=playback 不拉实时流 */
export function PreviewTile({
  slotIndex,
  camera,
  channelId,
  mode,
  muted,
  selected,
  showAiOverlay = false,
  frameEpoch,
  suspended = false,
  variant = 'live',
  onSelect,
  onPickSource,
  onOpenDetail,
}: {
  slotIndex: number
  camera: Camera | null
  channelId?: string
  mode: StreamMode
  muted: boolean
  selected: boolean
  /** AI 检测框叠层（WebRTC DataChannel frameflow-vision 真实结果） */
  showAiOverlay?: boolean
  frameEpoch: number
  /** 详情层打开时暂停分屏拉流，避免与主码流抢连接 */
  suspended?: boolean
  /** live=实时监控；playback=视频回放宫格（占位，不拉直播） */
  variant?: 'live' | 'playback'
  onSelect: () => void
  onPickSource: () => void
  onOpenDetail: () => void
}) {
  if (!camera) {
    return (
      <button
        type="button"
        className="ff-preview-tile ff-preview-tile-empty"
        onClick={onSelect}
        onDoubleClick={onPickSource}
      >
        <span className="ff-preview-slot">路 {slotIndex + 1}</span>
        <span className="ff-preview-empty-icon" aria-hidden>
          +
        </span>
        <Text type="supporting">空闲</Text>
      </button>
    )
  }

  return (
    <PreviewTilePlayer
      key={`${camera.id}:${channelId ?? ''}:${frameEpoch}`}
      camera={camera}
      channelId={channelId}
      mode={mode}
      muted={muted}
      selected={selected}
      showAiOverlay={showAiOverlay}
      suspended={suspended || variant === 'playback'}
      variant={variant}
      onSelect={onSelect}
      onPickSource={onPickSource}
      onOpenDetail={onOpenDetail}
    />
  )
}

function PreviewTilePlayer({
  camera,
  channelId,
  mode,
  muted,
  selected,
  showAiOverlay,
  suspended,
  variant,
  onSelect,
  onPickSource,
  onOpenDetail,
}: {
  camera: Camera
  channelId?: string
  mode: StreamMode
  muted: boolean
  selected: boolean
  showAiOverlay: boolean
  suspended: boolean
  variant: 'live' | 'playback'
  onSelect: () => void
  onPickSource: () => void
  onOpenDetail: () => void
}) {
  const navigate = useNavigate()
  const toast = useToast()
  const queryClient = useQueryClient()
  const [reconnecting, setReconnecting] = useState(false)
  const deviceOffline =
    camera.status === 'offline' ||
    camera.status === 'stopped' ||
    camera.status === 'error'

  const reconnect = async (e: MouseEvent) => {
    e.stopPropagation()
    if (reconnecting) return
    setReconnecting(true)
    try {
      await startCamera(camera.id)
      toast({ body: `已发起重连：${camera.name}`, type: 'info' })
      await queryClient.invalidateQueries({ queryKey: ['cameras'] })
    } catch (err) {
      toast({
        body: err instanceof Error ? err.message : '重连失败',
        type: 'error',
      })
    } finally {
      setReconnecting(false)
    }
  }

  // 国标：仅作提示，不拦截拉流（在线态以后端 INVITE 为准，避免列表态滞后误拦）
  const notReadyHint =
    isGbDeviceNotReady(camera) && !deviceOffline
      ? gbDeviceNotReadyHint(camera)
      : null

  const channel = camera.channels?.find(
    (c) => String(c.id) === channelId || c.channelId === channelId,
  )
  const title =
    channel?.name?.trim() ||
    (channelId ? `通道 ${channelId}` : camera.name)
  const subtitle = channelId
    ? `${camera.name} · ${channelId}`
    : camera.group || camera.deviceId || String(camera.id)
  const nameLabel = subtitle ? `${title} · ${subtitle}` : title
  const aiChannelId = resolveAiChannelId(camera, channelId)

  const [streamProfile, setStreamProfile] = useState<StreamProfile>('sub')
  const supportsStreamSwitch = camera.protocol === 'gb28181'
  const streamSrc = resolveStreamSrc(camera, channelId, streamProfile)

  const [playState, setPlayState] = useState<PlayState>(
    deviceOffline || !streamSrc ? 'offline' : 'connecting',
  )
  const [errorHint, setErrorHint] = useState('')
  const [paused, setPaused] = useState(false)
  const [bitrateKbps, setBitrateKbps] = useState(0)
  const [videoSize, setVideoSize] = useState({ w: 0, h: 0 })
  const [visionBoxes, setVisionBoxes] = useState<VisionBox[]>([])
  const gotVisionRef = useRef(false)
  const [visionHint, setVisionHint] = useState('')

  useEffect(() => {
    if (deviceOffline || !streamSrc) setPlayState('offline')
    else setPlayState('connecting')
    setErrorHint('')
    setBitrateKbps(0)
    setVideoSize({ w: 0, h: 0 })
    setVisionBoxes([])
    setVisionHint('')
    gotVisionRef.current = false
  }, [deviceOffline, streamSrc, mode])

  // 不再注入 mockVisionBoxes：仅展示 WebRTC DataChannel 真实检测结果
  const isPlaybackSlot = variant === 'playback'
  const overlayText = isPlaybackSlot
    ? deviceOffline
      ? camera.status === 'error'
        ? '信号异常'
        : StreamErrors.DEVICE_OFFLINE.message
      : '双击打开录像回放'
    : playState === 'offline'
      ? !streamSrc
        ? '无流地址'
        : camera.status === 'error'
          ? '信号异常'
          : isGbDeviceNotReady(camera)
            ? gbDeviceNotReadyHint(camera)
            : StreamErrors.DEVICE_OFFLINE.message
      : playState === 'connecting'
        ? notReadyHint || '正在拉流…'
        : playState === 'error'
          ? errorHint || '拉流失败'
          : playState === 'paused'
            ? '已暂停'
            : null

  const resText = formatResolution(
    videoSize.w,
    videoSize.h,
    camera.resolution,
  )
  const techRest = [
    resText || null,
    camera.fps ? `${camera.fps}fps` : null,
    playState === 'playing' ? formatBitrate(bitrateKbps) : null,
    muted ? '静音' : null,
    showAiOverlay && visionBoxes.length ? `AI ${visionBoxes.length}` : null,
  ].filter(Boolean)
  /** 后端 vision DataChannel 真实状态（无框时的原因提示） */
  const aiStatusLabel =
    showAiOverlay && !visionBoxes.length && visionHint ? visionHint : null

  const statusBadge = isPlaybackSlot
    ? { label: '回放', tone: 'paused' as const }
    : mergedStatusLabel(playState, camera.status)
  const streamLabel = streamProfile === 'main' ? '主码流' : '辅码流'
  const canSwitchStream =
    !isPlaybackSlot && supportsStreamSwitch && !deviceOffline && !!streamSrc

  const openAiConfig = (e: MouseEvent) => {
    e.stopPropagation()
    void navigate({
      to: '/devices/$id/channels/$channelId/ai',
      params: { id: String(camera.id), channelId: aiChannelId },
    })
  }

  return (
    <div
      role="button"
      tabIndex={0}
      className="ff-preview-tile"
      data-selected={selected ? 'true' : undefined}
      data-status={camera.status}
      data-play={isPlaybackSlot ? 'playback' : playState}
      data-variant={variant}
      onClick={() => {
        onSelect()
      }}
      onDoubleClick={(e) => {
        e.preventDefault()
        onSelect()
        onOpenDetail()
      }}
      onKeyDown={(e) => {
        if (e.key === 'Enter') {
          e.preventDefault()
          onSelect()
          onOpenDetail()
        } else if (e.key === ' ') {
          e.preventDefault()
          onSelect()
        }
      }}
    >
      {!deviceOffline && streamSrc && !paused && !suspended ? (
        <Go2rtcPlayer
          streamSrc={streamSrc}
          mode={mode}
          muted={muted}
          className="ff-go2rtc-host"
          onState={(s) => {
            if (s === 'playing') {
              setErrorHint('')
              setPlayState('playing')
            } else if (s === 'error') setPlayState('error')
            else if (s === 'connecting') setPlayState('connecting')
          }}
          onError={(msg) => {
            setErrorHint(msg)
            setPlayState('error')
          }}
          onStats={({ bitrateKbps: kbps, width, height }) => {
            setBitrateKbps(kbps)
            if (width > 0 && height > 0) setVideoSize({ w: width, h: height })
          }}
          onVisionFrame={(payload) => {
            if (!showAiOverlay) return
            const boxes = parseVisionPayload(payload)
            gotVisionRef.current = true
            setVisionBoxes(boxes)
            const text =
              payload && typeof payload === 'object' && 'text' in payload
                ? String((payload as { text?: string }).text ?? '')
                : ''
            if (boxes.length) setVisionHint('')
            else if (text.includes('未启动') || text.includes('未运行'))
              setVisionHint('未启动算法')
            else if (text.includes('未配置') || text.includes('模型'))
              setVisionHint('无模型')
            else setVisionHint('')
          }}
        />
      ) : null}

      <DetectionOverlay
        boxes={visionBoxes}
        visible={showAiOverlay}
        videoW={videoSize.w}
        videoH={videoSize.h}
      />

      {!isPlaybackSlot && !deviceOffline ? (
        <span className="ff-live-badge">直播</span>
      ) : null}

      {deviceOffline && !isPlaybackSlot ? (
        <div className="ff-tile-offline">
          <span className="ff-tile-offline-icon" aria-hidden>
            <VideoCameraSlashIcon />
          </span>
          <span className="ff-tile-offline-title">设备离线</span>
          <span className="ff-tile-offline-sub">
            最后在线 {formatLastSeen(camera.updatedAt)} · 检查网络后可重连
          </span>
          <button
            type="button"
            className="ff-tile-offline-btn"
            onClick={reconnect}
            disabled={reconnecting}
          >
            {reconnecting ? '重连中…' : '重连设备'}
          </button>
        </div>
      ) : overlayText ? (
        <div className="ff-preview-offline" data-kind={playState}>
          <span>{overlayText}</span>
          {playState === 'connecting' ? (
            <span className="ff-preview-offline-sub">{streamSrc}</span>
          ) : null}
        </div>
      ) : null}

      <span className="ff-preview-slot ff-preview-slot-name" title={nameLabel}>
        {nameLabel}
      </span>

      <div className="ff-preview-hud">
        <div className="ff-preview-hud-main">
          <div className="ff-preview-hud-row">
            <span
              className="ff-preview-state-pill"
              data-tone={statusBadge.tone}
            >
              {statusBadge.label}
            </span>
            <span className="ff-preview-hud-tech">
              {supportsStreamSwitch ? (
                canSwitchStream ? (
                  <button
                    type="button"
                    className="ff-preview-stream-chip"
                    data-profile={streamProfile}
                    title={
                      streamProfile === 'sub'
                        ? '当前辅码流，点击切换主码流'
                        : '当前主码流，点击切换辅码流'
                    }
                    onClick={(e) => {
                      e.stopPropagation()
                      setStreamProfile((p) => (p === 'sub' ? 'main' : 'sub'))
                    }}
                  >
                    {streamLabel}
                  </button>
                ) : (
                  streamLabel
                )
              ) : null}
              {techRest.length
                ? `${supportsStreamSwitch ? ' · ' : ''}${techRest.join(' · ')}`
                : ''}
              {aiStatusLabel ? (
                <>
                  {supportsStreamSwitch || techRest.length ? ' · ' : ''}
                  <button
                    type="button"
                    className="ff-preview-ai-chip"
                    title="前往配置设备 AI"
                    onClick={openAiConfig}
                  >
                    {aiStatusLabel}
                  </button>
                </>
              ) : null}
            </span>
          </div>
        </div>
      </div>

      <div className="ff-preview-actions">
        {!isPlaybackSlot && !deviceOffline && streamSrc ? (
          <button
            type="button"
            className="ff-preview-switch"
            onClick={(e) => {
              e.stopPropagation()
              setPaused((p) => !p)
              setPlayState((s) => (s === 'paused' || paused ? 'connecting' : 'paused'))
            }}
          >
            {paused ? '播放' : '暂停'}
          </button>
        ) : null}
        {isPlaybackSlot ? (
          <button
            type="button"
            className="ff-preview-switch"
            onClick={(e) => {
              e.stopPropagation()
              onOpenDetail()
            }}
          >
            打开回放
          </button>
        ) : (
          <button
            type="button"
            className="ff-preview-switch"
            title="配置本路视频 AI 算法"
            onClick={openAiConfig}
          >
            AI 配置
          </button>
        )}
        <button
          type="button"
          className="ff-preview-switch"
          onClick={(e) => {
            e.stopPropagation()
            onPickSource()
          }}
        >
          换源
        </button>
        <button
          type="button"
          className="ff-preview-switch ff-preview-switch--icon"
          title="全屏查看"
          aria-label="全屏查看"
          onClick={(e) => {
            e.stopPropagation()
            onOpenDetail()
          }}
        >
          <ArrowsPointingOutIcon aria-hidden />
        </button>
      </div>
    </div>
  )
}
