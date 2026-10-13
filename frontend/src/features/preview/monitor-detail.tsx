import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import type { ISODateString } from '@astryxdesign/core/Calendar'
import { DateInput } from '@astryxdesign/core/DateInput'
import { Selector } from '@astryxdesign/core/Selector'
import { useToast } from '@astryxdesign/core/Toast'
import {
  buildPlaybackStreamUrl,
  listGb28181Records,
  postPtz,
  type PtzCmd,
} from '@/apis/gb28181-api'
import {
  Go2rtcPlayer,
  type Go2rtcPlayerHandle,
  type StreamPlayStats,
} from '@/lib/go2rtc/go2rtc-player'
import { resolveAiChannelId } from '@/lib/ai-stream'
import { withAdvertiseHost } from '@/lib/camera-protocol'
import { resolveStreamSrc } from '@/lib/preview-source'
import { getErrorMessage } from '@/lib/http'
import {
  gbDeviceNotReadyHint,
  isGbDeviceNotReady,
} from '@/lib/stream-error'
import { parseVisionPayload, type VisionBox } from '@/lib/vision-types'
import type { Camera } from '@/types/api'
import { DetectionOverlay } from './detection-overlay'
import { PlaybackTimeline } from './playback-timeline'
import {
  dayBounds,
  findSegmentForTime,
  formatClock,
  formatDay,
  formatDuration,
  normalizeRecords,
  type PlaybackSegment,
} from './playback-utils'

const STEP_OPTIONS = Array.from({ length: 8 }, (_, i) => ({
  label: String(i + 1),
  value: String(i + 1),
}))

function stepToSpeed(step: number): number {
  return Math.min(255, Math.max(1, step * 32))
}

function formatBitrate(kbps: number): string {
  if (kbps <= 0) return '—'
  if (kbps >= 1000) return `${(kbps / 1000).toFixed(1)} Mbps`
  return `${Math.round(kbps)} kbps`
}

function formatBandwidth(kbps: number, kBps: number): string {
  if (kBps > 0) {
    if (kBps >= 1024) return `${(kBps / 1024).toFixed(2)} MB/s`
    return `${kBps.toFixed(2)} kB/s`
  }
  if (kbps <= 0) return '—'
  const asKBps = kbps / 8
  if (asKBps >= 1024) return `${(asKBps / 1024).toFixed(2)} MB/s`
  return `${asKBps.toFixed(2)} kB/s`
}

function formatLatency(sec: number | null): string {
  if (sec == null || !Number.isFinite(sec)) return '—'
  return `${sec.toFixed(2)} 秒`
}

function safeFilePart(name: string): string {
  return name.replace(/[\\/:*?"<>|]+/g, '_').trim() || 'snapshot'
}

const PROTOCOL_LABEL: Record<string, string> = {
  gb28181: '国标 GB28181',
  rtsp: 'RTSP',
  rtmp: 'RTMP',
  onvif: 'ONVIF',
  hls: 'HLS',
}

type ViewMode = 'live' | 'playback'

export function MonitorDetail({
  camera,
  channelId,
  showAiOverlay = true,
  initialPlayback = false,
  initialSeekAt,
  initialDay,
  playbackOnly = false,
  onClose,
}: {
  camera: Camera
  channelId?: string
  /** 实时预览时叠 AI 检测框 */
  showAiOverlay?: boolean
  /** 打开时直接进入回放 */
  initialPlayback?: boolean
  /** 回放定位时刻（ISO） */
  initialSeekAt?: string
  /** 回放日期（YYYY-MM-DD）；无 seek 时用此日 */
  initialDay?: string
  /** 仅回放：隐藏「直播」切换（视频回放页使用） */
  playbackOnly?: boolean
  onClose: () => void
}) {
  const toast = useToast()
  const navigate = useNavigate()
  const channel = camera.channels?.find(
    (c) => String(c.id) === channelId || c.channelId === channelId,
  )
  const channelName = channel?.name?.trim() || ''
  const title = camera.name
  const deviceId = camera.deviceId || String(camera.id)
  const ptzChannelId =
    (channel ? channel.channelId : undefined) ||
    channelId ||
    camera.channel ||
    deviceId
  const aiChannelId = resolveAiChannelId(camera, channelId)
  const isGb = camera.protocol === 'gb28181'
  const isPushIngest =
    camera.ingestMode === 'push' &&
    (camera.protocol === 'rtsp' || camera.protocol === 'rtmp')
  const pushUrl = isPushIngest
    ? withAdvertiseHost(camera.publishUrl || '') ||
      (camera.streamKey
        ? `${camera.protocol === 'rtsp' ? 'rtsp' : 'rtmp'}://${camera.ip || '127.0.0.1'}:${camera.port || (camera.protocol === 'rtsp' ? 8554 : 1935)}/${camera.streamKey}`
        : '')
    : ''
  const deviceNotReady = isGbDeviceNotReady(camera)
  const liveSrc = resolveStreamSrc(camera, channelId, 'main')

  const seekMs = useMemo(() => {
    if (!initialSeekAt) return null
    const t = new Date(initialSeekAt).getTime()
    return Number.isFinite(t) ? t : null
  }, [initialSeekAt])

  const playerRef = useRef<Go2rtcPlayerHandle>(null)
  const autoSeekDoneRef = useRef(false)
  const wantPlayback =
    (initialPlayback || playbackOnly) && camera.protocol === 'gb28181'
  const [mode, setMode] = useState<ViewMode>(() =>
    wantPlayback ? 'playback' : 'live',
  )
  const [playState, setPlayState] = useState('connecting')
  const [playError, setPlayError] = useState('')
  const [stats, setStats] = useState<StreamPlayStats | null>(null)
  const [muted, setMuted] = useState(true)
  const [step, setStep] = useState(5)
  const [ptzError, setPtzError] = useState('')
  const holdingRef = useRef(false)
  const [visionBoxes, setVisionBoxes] = useState<VisionBox[]>([])
  const [visionHint, setVisionHint] = useState('')

  const [pbDay, setPbDay] = useState<ISODateString>(() => {
    if (seekMs != null) return formatDay(new Date(seekMs))
    const day = initialDay?.trim()
    return day ? (day as ISODateString) : formatDay()
  })
  const [pbSegments, setPbSegments] = useState<PlaybackSegment[]>([])
  const [pbLoading, setPbLoading] = useState(false)
  const [pbError, setPbError] = useState('')
  const [pbSrc, setPbSrc] = useState<string | null>(null)
  const [pbCursorMs, setPbCursorMs] = useState<number | null>(null)
  const [activeSegId, setActiveSegId] = useState<string | null>(null)

  // 始终尝试拉流；未就绪仅作提示，以后端错误码为准
  const streamSrc = mode === 'live' ? liveSrc : pbSrc
  const aiLive = showAiOverlay && mode === 'live'

  useEffect(() => {
    setPlayError('')
  }, [mode, streamSrc])

  useEffect(() => {
    if (!aiLive) {
      setVisionBoxes([])
      setVisionHint('')
    }
  }, [aiLive])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  useEffect(() => {
    if (!initialPlayback && !playbackOnly) return
    if (isGb) return
    toast({ body: '当前设备不支持录像回放，已打开实时监控', type: 'info' })
  }, [initialPlayback, playbackOnly, isGb, toast])

  useEffect(() => {
    if (mode !== 'playback' || !isGb) return
    const ac = new AbortController()
    const { start, end } = dayBounds(pbDay)
    setPbLoading(true)
    setPbError('')
    setPbSegments([])
    setPbSrc(null)
    setPbCursorMs(null)
    setActiveSegId(null)
    void listGb28181Records(
      deviceId,
      ptzChannelId,
      { start, end },
      { signal: ac.signal },
    )
      .then((items) => {
        if (ac.signal.aborted) return
        setPbSegments(normalizeRecords(items))
      })
      .catch((e) => {
        if (ac.signal.aborted) return
        const msg = getErrorMessage(e)
        setPbError(msg)
        toast({ body: msg, type: 'error' })
      })
      .finally(() => {
        if (!ac.signal.aborted) setPbLoading(false)
      })
    return () => {
      ac.abort()
    }
  }, [mode, pbDay, isGb, deviceId, ptzChannelId, toast])

  // 录像列表就绪后：有 seek 则定位；无 seek 且为回放入口则播首段
  useEffect(() => {
    autoSeekDoneRef.current = false
  }, [seekMs, pbDay])

  useEffect(() => {
    if (autoSeekDoneRef.current) return
    if (mode !== 'playback' || pbLoading) return
    if (!pbSegments.length) return
    if (seekMs != null) {
      const hit = findSegmentForTime(pbSegments, seekMs)
      autoSeekDoneRef.current = true
      if (!hit) {
        toast({ body: '该时刻附近无录像片段', type: 'info' })
        return
      }
      playSegment(hit, seekMs)
      return
    }
    if (initialPlayback || playbackOnly) {
      autoSeekDoneRef.current = true
      const first = pbSegments[0]
      playSegment(first, first.startMs)
    }
  }, [mode, pbLoading, pbSegments, seekMs, initialPlayback, playbackOnly, toast])

  async function sendPtz(cmd: PtzCmd) {
    if (!isGb || mode !== 'live') return
    setPtzError('')
    try {
      await postPtz(deviceId, ptzChannelId, {
        cmd,
        speed: stepToSpeed(step),
      })
    } catch (e) {
      setPtzError(getErrorMessage(e))
    }
  }

  function holdStart(cmd: PtzCmd) {
    holdingRef.current = true
    void sendPtz(cmd)
  }

  function holdEnd() {
    if (!holdingRef.current) return
    holdingRef.current = false
    void sendPtz('stop')
  }

  function handleScreenshot() {
    const dataUrl = playerRef.current?.capturePng()
    if (!dataUrl) {
      toast({ body: '截图失败：画面尚未就绪', type: 'error' })
      return
    }
    const stamp = new Date()
      .toISOString()
      .replace(/[:.]/g, '-')
      .slice(0, 19)
    const fileName = `${safeFilePart(title)}_${stamp}.png`
    const a = document.createElement('a')
    a.href = dataUrl
    a.download = fileName
    a.rel = 'noopener'
    document.body.appendChild(a)
    a.click()
    a.remove()
    toast({ body: `已保存截图 ${fileName}`, type: 'info' })
  }

  function enterPlayback() {
    if (!isGb) {
      toast({ body: '仅国标设备支持回放', type: 'error' })
      return
    }
    setMode('playback')
    setPlayState('idle')
  }

  function leavePlayback() {
    setPbSrc(null)
    setPbCursorMs(null)
    setActiveSegId(null)
    setMode('live')
    setPlayError('')
    setPlayState('connecting')
  }

  function handleClose() {
    setPbSrc(null)
    onClose()
  }

  function playSegment(seg: PlaybackSegment, atMs?: number) {
    // 有指定时刻时始终按 start/end 重建 URL，避免沿用整段 streamUrl 导致从片段头播
    let playFrom = seg.startMs
    if (atMs != null && Number.isFinite(atMs)) {
      const latest = Math.max(seg.startMs, seg.endMs - 1000)
      playFrom = Math.min(Math.max(atMs, seg.startMs), latest)
    }
    const startUnix = Math.floor(playFrom / 1000)
    const endUnix = Math.floor(seg.endMs / 1000)
    if (endUnix <= startUnix) {
      toast({ body: '回放时间范围无效', type: 'error' })
      return
    }
    const playSrc = buildPlaybackStreamUrl(
      deviceId,
      ptzChannelId,
      startUnix,
      endUnix,
    )
    setPbSrc(playSrc)
    setPbCursorMs(playFrom)
    setActiveSegId(seg.id)
    setPlayError('')
    setPlayState('connecting')
  }

  function onTimelineSeek(tMs: number, seg: PlaybackSegment | null) {
    if (!seg) {
      toast({ body: '该时段无录像', type: 'info' })
      return
    }
    playSegment(seg, tMs)
  }

  const resText =
    stats && stats.width > 0 && stats.height > 0
      ? `${stats.width}×${stats.height}`
      : camera.resolution || '—'

  const playLabel =
    mode === 'playback'
      ? pbLoading
        ? '查询中'
        : pbSrc
          ? playState === 'playing'
            ? '回放中'
            : playState === 'connecting'
              ? '回放连接中'
              : playState === 'error'
                ? playError || '回放失败'
                : '回放'
          : pbSegments.length
            ? '待播放'
            : '无录像'
      : playState === 'playing'
        ? '播放中'
        : playState === 'connecting'
          ? deviceNotReady
            ? gbDeviceNotReadyHint(camera)
            : '连接中'
          : playState === 'error'
            ? playError || '拉流失败'
            : playState === 'idle'
              ? '空闲'
              : playState

  return (
    <div
      className="ff-monitor-detail"
      role="dialog"
      aria-label={playbackOnly ? '录像回放' : '实时监控详情'}
    >
      <div className="ff-monitor-detail-bar">
        <div className="ff-monitor-detail-bar-title">
          <button
            type="button"
            className="ff-monitor-detail-btn"
            onClick={handleClose}
          >
            ← 返回
          </button>
        </div>
        <div className="ff-monitor-detail-bar-actions">
          {isGb && !playbackOnly ? (
            mode === 'live' ? (
              <button
                type="button"
                className="ff-monitor-detail-btn"
                onClick={enterPlayback}
              >
                回放
              </button>
            ) : (
              <button
                type="button"
                className="ff-monitor-detail-btn"
                onClick={leavePlayback}
              >
                直播
              </button>
            )
          ) : null}
          {!playbackOnly ? (
            <button
              type="button"
              className="ff-monitor-detail-btn"
              onClick={() => {
                void navigate({
                  to: '/devices/$id/channels/$channelId/ai',
                  params: { id: String(camera.id), channelId: aiChannelId },
                })
              }}
            >
              AI 配置
            </button>
          ) : null}
          <button
            type="button"
            className="ff-monitor-detail-btn"
            aria-pressed={muted}
            onClick={() => setMuted((v) => !v)}
          >
            {muted ? '静音' : '有声'}
          </button>
          <button
            type="button"
            className="ff-monitor-detail-btn"
            onClick={handleScreenshot}
          >
            截图
          </button>
        </div>
      </div>

      <div
        className="ff-monitor-detail-body"
        data-mode={mode}
      >
        <div className="ff-monitor-detail-main">
          <div className="ff-monitor-detail-player">
            {streamSrc ? (
              <Go2rtcPlayer
                key={`${mode}:${streamSrc}`}
                ref={playerRef}
                streamSrc={streamSrc}
                mode="webrtc"
                muted={muted}
                className="ff-go2rtc-host"
                onState={(s) => {
                  setPlayState(s)
                  if (s === 'playing') setPlayError('')
                }}
                onError={(msg) => {
                  setPlayError(msg)
                  setPlayState('error')
                }}
                onStats={setStats}
                onVisionFrame={(payload) => {
                  if (!aiLive) return
                  const boxes = parseVisionPayload(payload)
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
            ) : (
              <div className="ff-monitor-detail-empty">
                {mode === 'playback'
                  ? pbLoading
                    ? '正在查询录像…'
                    : '选择片段或时间轴开始回放'
                  : '无流地址'}
              </div>
            )}
            <DetectionOverlay
              boxes={visionBoxes}
              visible={aiLive}
              videoW={stats?.width ?? 0}
              videoH={stats?.height ?? 0}
            />
            {aiLive && !visionBoxes.length && visionHint ? (
              <div className="ff-monitor-detail-ai-hint">{visionHint}</div>
            ) : null}
            {aiLive && visionBoxes.length > 0 ? (
              <div className="ff-monitor-detail-ai-count">
                AI {visionBoxes.length}
              </div>
            ) : null}
            {streamSrc && playState === 'error' && playError ? (
              <div className="ff-monitor-detail-play-error">{playError}</div>
            ) : null}
          </div>

          {mode === 'playback' ? (
            <PlaybackTimeline
              day={pbDay}
              segments={pbSegments}
              cursorMs={pbCursorMs}
              activeSegId={activeSegId}
              onSeek={onTimelineSeek}
            />
          ) : null}
        </div>

        <aside className="ff-monitor-detail-side" data-mode={mode}>
          <section className="ff-monitor-detail-section">
            <h3 className="ff-monitor-detail-section-title">流信息</h3>
            <dl className="ff-monitor-detail-dl">
              <div>
                <dt>名称</dt>
                <dd>{camera.name}</dd>
              </div>
              <div>
                <dt>设备 ID</dt>
                <dd className="ff-monitor-detail-mono">{deviceId}</dd>
              </div>
              <div>
                <dt>通道名称</dt>
                <dd>
                  {channelName ||
                    (channelId ? `通道 ${channelId}` : '—')}
                </dd>
              </div>
              <div>
                <dt>通道</dt>
                <dd className="ff-monitor-detail-mono">
                  {channelId || camera.channel || '—'}
                </dd>
              </div>
              <div>
                <dt>协议</dt>
                <dd>
                  {PROTOCOL_LABEL[camera.protocol] ?? camera.protocol}
                  {isPushIngest ? ' · 推流' : ''}
                </dd>
              </div>
              {isPushIngest && pushUrl ? (
                <div>
                  <dt>推流地址</dt>
                  <dd className="ff-monitor-detail-mono ff-monitor-detail-url">
                    <button
                      type="button"
                      className="ff-monitor-detail-copy"
                      title="点击复制"
                      onClick={() => {
                        void navigator.clipboard.writeText(pushUrl).then(
                          () => toast({ body: '已复制推流地址', type: 'info' }),
                          () => toast({ body: '复制失败', type: 'error' }),
                        )
                      }}
                    >
                      {pushUrl}
                    </button>
                  </dd>
                </div>
              ) : camera.ip ? (
                <div>
                  <dt>地址</dt>
                  <dd>
                    {camera.ip}
                    {camera.port ? `:${camera.port}` : ''}
                  </dd>
                </div>
              ) : null}
              {camera.group ? (
                <div>
                  <dt>分组</dt>
                  <dd>{camera.group}</dd>
                </div>
              ) : null}
              <div>
                <dt>状态</dt>
                <dd>
                  {camera.status === 'online' || camera.status === 'running'
                    ? '在线'
                    : camera.status === 'offline'
                      ? '离线'
                      : camera.status === 'stopped'
                        ? '已停止'
                        : camera.status === 'error'
                          ? '异常'
                          : camera.status}
                </dd>
              </div>
              <div>
                <dt>码流</dt>
                <dd>
                  {mode === 'playback'
                    ? '回放'
                    : isGb
                      ? '主码流'
                      : '—'}
                </dd>
              </div>
              <div>
                <dt>播放</dt>
                <dd>{playLabel}</dd>
              </div>
              <div>
                <dt>分辨率</dt>
                <dd>{resText}</dd>
              </div>
              <div>
                <dt>帧率</dt>
                <dd>{camera.fps ? `${camera.fps} fps` : '—'}</dd>
              </div>
            </dl>
          </section>

          {mode === 'playback' ? (
            <>
              <section className="ff-monitor-detail-section ff-playback-date-section">
                <DateInput
                  label="日期"
                  isLabelHidden
                  size="sm"
                  width="100%"
                  value={pbDay}
                  max={formatDay()}
                  onChange={(v) => setPbDay(v ?? formatDay())}
                  placeholder="选择日期"
                />
                {pbError ? <p className="ff-ptz-error">{pbError}</p> : null}
              </section>

              <section className="ff-monitor-detail-section ff-playback-list-section">
                <h3 className="ff-monitor-detail-section-title">
                  片段列表
                  {pbLoading ? ' · 查询中…' : ` · ${pbSegments.length}`}
                </h3>
                <div className="ff-playback-list">
                  {pbSegments.map((seg) => (
                    <button
                      key={seg.id}
                      type="button"
                      className="ff-playback-list-item"
                      data-active={activeSegId === seg.id ? 'true' : undefined}
                      onClick={() => playSegment(seg)}
                    >
                      <span className="ff-playback-list-time">
                        {formatClock(seg.startMs)} – {formatClock(seg.endMs)}
                      </span>
                      <span className="ff-playback-list-meta">
                        {formatDuration(seg.endMs - seg.startMs)}
                        {seg.label ? ` · ${seg.label}` : ''}
                      </span>
                    </button>
                  ))}
                  {!pbLoading && !pbSegments.length ? (
                    <p className="ff-playback-list-empty">当日无录像片段</p>
                  ) : null}
                </div>
              </section>
            </>
          ) : (
            <>
              <section className="ff-monitor-detail-section">
                <h3 className="ff-monitor-detail-section-title">统计信息</h3>
                <dl className="ff-monitor-detail-dl">
                  <div>
                    <dt>流类型</dt>
                    <dd>{stats?.mode ?? '—'}</dd>
                  </div>
                  <div>
                    <dt>带宽</dt>
                    <dd>
                      {stats
                        ? formatBandwidth(
                            stats.bitrateKbps,
                            stats.bandwidthKBps,
                          )
                        : '—'}
                    </dd>
                  </div>
                  <div>
                    <dt>延迟</dt>
                    <dd>{formatLatency(stats?.latencySec ?? null)}</dd>
                  </div>
                  <div>
                    <dt>总帧数</dt>
                    <dd>{stats?.framesReceived || '—'}</dd>
                  </div>
                  <div>
                    <dt>解码帧数</dt>
                    <dd>{stats?.framesDecoded || '—'}</dd>
                  </div>
                  <div>
                    <dt>丢帧率</dt>
                    <dd>
                      {stats && stats.framesReceived > 0
                        ? `${stats.dropRate.toFixed(2)}%`
                        : '—'}
                    </dd>
                  </div>
                  <div>
                    <dt>码率</dt>
                    <dd>{formatBitrate(stats?.bitrateKbps ?? 0)}</dd>
                  </div>
                  <div>
                    <dt>分辨率</dt>
                    <dd>{resText}</dd>
                  </div>
                </dl>
              </section>

              {isGb ? (
                <section className="ff-monitor-detail-section ff-monitor-ptz">
                  <h3 className="ff-monitor-detail-section-title">云台控制</h3>
                  <div className="ff-ptz-pad" aria-label="云台方向">
                    <button
                      type="button"
                      className="ff-ptz-btn ff-ptz-up"
                      aria-label="上"
                      onPointerDown={(e) => {
                        e.preventDefault()
                        holdStart('up')
                      }}
                      onPointerUp={holdEnd}
                      onPointerLeave={holdEnd}
                      onPointerCancel={holdEnd}
                    />
                    <button
                      type="button"
                      className="ff-ptz-btn ff-ptz-left"
                      aria-label="左"
                      onPointerDown={(e) => {
                        e.preventDefault()
                        holdStart('left')
                      }}
                      onPointerUp={holdEnd}
                      onPointerLeave={holdEnd}
                      onPointerCancel={holdEnd}
                    />
                    <button
                      type="button"
                      className="ff-ptz-btn ff-ptz-center"
                      aria-label="停止"
                      onClick={() => void sendPtz('stop')}
                    />
                    <button
                      type="button"
                      className="ff-ptz-btn ff-ptz-right"
                      aria-label="右"
                      onPointerDown={(e) => {
                        e.preventDefault()
                        holdStart('right')
                      }}
                      onPointerUp={holdEnd}
                      onPointerLeave={holdEnd}
                      onPointerCancel={holdEnd}
                    />
                    <button
                      type="button"
                      className="ff-ptz-btn ff-ptz-down"
                      aria-label="下"
                      onPointerDown={(e) => {
                        e.preventDefault()
                        holdStart('down')
                      }}
                      onPointerUp={holdEnd}
                      onPointerLeave={holdEnd}
                      onPointerCancel={holdEnd}
                    />
                  </div>

                  <div className="ff-ptz-step">
                    <Selector
                      label="步长(1-8)"
                      value={String(step)}
                      onChange={(v) => setStep(Number(v) || 5)}
                      options={STEP_OPTIONS}
                    />
                  </div>

                  {ptzError ? (
                    <p className="ff-ptz-error">{ptzError}</p>
                  ) : null}
                </section>
              ) : null}
            </>
          )}
        </aside>
      </div>
    </div>
  )
}
