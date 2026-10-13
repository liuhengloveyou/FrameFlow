import { useEffect, useRef, useState } from 'react'
import { Button } from '@astryxdesign/core/Button'
import { Text } from '@astryxdesign/core/Text'
import { ToneBadge } from '@/components/tone-badge'
import { Go2rtcPlayer } from '@/lib/go2rtc/go2rtc-player'
import { DetectionOverlay } from '@/features/preview/detection-overlay'
import { parseVisionPayload, type VisionBox } from '@/lib/vision-types'
import type { Point2D } from '@/types/api'

type Mode = 'rect' | 'poly' | 'select'

const COLORS = [
  { stroke: '#38bdf8', fill: 'rgba(56,189,248,0.22)' },
  { stroke: '#a78bfa', fill: 'rgba(167,139,250,0.22)' },
  { stroke: '#34d399', fill: 'rgba(52,211,153,0.22)' },
  { stroke: '#f472b6', fill: 'rgba(244,114,182,0.22)' },
]

function pointInPoly(p: Point2D, poly: Point2D[]) {
  if (poly.length < 3) return false
  let inside = false
  for (let i = 0, j = poly.length - 1; i < poly.length; j = i++) {
    const xi = poly[i].x
    const yi = poly[i].y
    const xj = poly[j].x
    const yj = poly[j].y
    const intersect =
      yi > p.y !== yj > p.y && p.x < ((xj - xi) * (p.y - yi)) / (yj - yi + 1e-12) + xi
    if (intersect) inside = !inside
  }
  return inside
}

/** 区域编辑：实况底图上绘制矩形 / 多边形，坐标归一化 0~1；同时叠 AI 检测框 */
export function RegionEditor({
  regions,
  onChange,
  cameraLabel,
  streamSrc,
  readOnly = false,
  showAiOverlay = true,
}: {
  regions: Point2D[][]
  onChange: (regions: Point2D[][]) => void
  cameraLabel?: string
  /** go2rtc 流源；有则叠实况，无则暗色占位 */
  streamSrc?: string
  /** 只看画面、不可绘制（未绑定算法时的引导态） */
  readOnly?: boolean
  /** 是否显示 WebRTC DataChannel 推送的 AI 检测框 */
  showAiOverlay?: boolean
}) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const wrapRef = useRef<HTMLDivElement>(null)
  const stageRef = useRef<HTMLDivElement>(null)
  const [mode, setMode] = useState<Mode>('rect')
  const [draft, setDraft] = useState<Point2D[]>([])
  const [selectedIndex, setSelectedIndex] = useState<number | null>(null)
  const [size, setSize] = useState({ w: 640, h: 360 })
  const [playState, setPlayState] = useState<'connecting' | 'playing' | 'error' | 'idle'>(
    'idle',
  )
  const [visionBoxes, setVisionBoxes] = useState<VisionBox[]>([])
  const [visionHint, setVisionHint] = useState('')
  const dragStart = useRef<Point2D | null>(null)
  const live = Boolean(streamSrc)

  useEffect(() => {
    const el = stageRef.current
    if (!el) return
    const ro = new ResizeObserver((entries) => {
      const cr = entries[0]?.contentRect
      if (!cr) return
      const w = Math.max(160, Math.floor(cr.width))
      const h = Math.max(120, Math.floor(cr.height))
      setSize((prev) => (prev.w === w && prev.h === h ? prev : { w, h }))
    })
    ro.observe(el)
    return () => ro.disconnect()
  }, [])

  useEffect(() => {
    setPlayState(streamSrc ? 'connecting' : 'idle')
    setVisionBoxes([])
    setVisionHint('')
  }, [streamSrc])

  useEffect(() => {
    if (!showAiOverlay) {
      setVisionBoxes([])
      setVisionHint('')
    }
  }, [showAiOverlay])

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return
    const { w: width, h: height } = size
    canvas.width = width
    canvas.height = height
    ctx.clearRect(0, 0, width, height)

    if (!live) {
      const grad = ctx.createLinearGradient(0, 0, width, height)
      grad.addColorStop(0, '#1c2620')
      grad.addColorStop(0.45, '#121816')
      grad.addColorStop(1, '#0d1110')
      ctx.fillStyle = grad
      ctx.fillRect(0, 0, width, height)

      ctx.fillStyle = 'rgba(40, 70, 55, 0.45)'
      ctx.fillRect(width * 0.08, height * 0.55, width * 0.35, height * 0.28)
      ctx.fillStyle = 'rgba(50, 60, 80, 0.4)'
      ctx.fillRect(width * 0.55, height * 0.2, width * 0.3, height * 0.35)

      ctx.strokeStyle = 'rgba(255,255,255,0.08)'
      ctx.lineWidth = 1
      for (let i = 1; i < 4; i++) {
        ctx.beginPath()
        ctx.moveTo((width * i) / 4, 0)
        ctx.lineTo((width * i) / 4, height)
        ctx.stroke()
        ctx.beginPath()
        ctx.moveTo(0, (height * i) / 4)
        ctx.lineTo(width, (height * i) / 4)
        ctx.stroke()
      }
    }

    if (cameraLabel) {
      ctx.fillStyle = 'rgba(0,0,0,0.5)'
      ctx.fillRect(8, 8, Math.min(width - 16, cameraLabel.length * 9 + 24), 22)
      ctx.fillStyle = 'rgba(255,255,255,0.9)'
      ctx.font = '12px ui-sans-serif, system-ui, sans-serif'
      ctx.fillText(cameraLabel, 16, 23)
    }

    const drawPoly = (
      pts: Point2D[],
      color: string,
      fill: string,
      label?: string,
      selected?: boolean,
    ) => {
      if (pts.length < 2) return
      ctx.beginPath()
      ctx.moveTo(pts[0].x * width, pts[0].y * height)
      for (let i = 1; i < pts.length; i++) {
        ctx.lineTo(pts[i].x * width, pts[i].y * height)
      }
      if (pts.length >= 3) ctx.closePath()
      ctx.fillStyle = fill
      ctx.fill()
      ctx.strokeStyle = color
      ctx.lineWidth = selected ? 3 : 2
      ctx.stroke()

      if (selected) {
        for (const pt of pts) {
          ctx.beginPath()
          ctx.arc(pt.x * width, pt.y * height, 4, 0, Math.PI * 2)
          ctx.fillStyle = '#fff'
          ctx.fill()
          ctx.strokeStyle = color
          ctx.lineWidth = 1.5
          ctx.stroke()
        }
      }

      if (label && pts.length) {
        const cx = pts.reduce((s, p) => s + p.x, 0) / pts.length
        const cy = pts.reduce((s, p) => s + p.y, 0) / pts.length
        ctx.fillStyle = 'rgba(0,0,0,0.55)'
        ctx.beginPath()
        ctx.arc(cx * width, cy * height, 11, 0, Math.PI * 2)
        ctx.fill()
        ctx.fillStyle = '#fff'
        ctx.font = '11px ui-sans-serif, system-ui, sans-serif'
        ctx.textAlign = 'center'
        ctx.textBaseline = 'middle'
        ctx.fillText(label, cx * width, cy * height)
        ctx.textAlign = 'start'
        ctx.textBaseline = 'alphabetic'
      }
    }

    regions.forEach((region, i) => {
      const c = COLORS[i % COLORS.length]
      drawPoly(region, c.stroke, c.fill, String(i + 1), selectedIndex === i)
    })
    if (draft.length) drawPoly(draft, '#fbbf24', 'rgba(251,191,36,0.28)')

    if (live && regions.length === 0 && draft.length === 0) {
      ctx.strokeStyle = 'rgba(56,189,248,0.55)'
      ctx.lineWidth = 2
      ctx.setLineDash([8, 6])
      ctx.strokeRect(6, 6, width - 12, height - 12)
      ctx.setLineDash([])
      ctx.fillStyle = 'rgba(0,0,0,0.5)'
      ctx.fillRect(12, height - 30, 88, 20)
      ctx.fillStyle = 'rgba(255,255,255,0.92)'
      ctx.font = '12px ui-sans-serif, system-ui, sans-serif'
      ctx.fillText('整帧检测', 20, height - 16)
    }
  }, [regions, draft, cameraLabel, selectedIndex, size, live])

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      const tag = (e.target as HTMLElement | null)?.tagName
      if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return
      if (e.key === 'Escape') {
        setDraft([])
        dragStart.current = null
        return
      }
      if ((e.key === 'Delete' || e.key === 'Backspace') && selectedIndex != null) {
        e.preventDefault()
        onChange(regions.filter((_, i) => i !== selectedIndex))
        setSelectedIndex(null)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [selectedIndex, regions, onChange])

  function toNorm(e: React.MouseEvent<HTMLCanvasElement>): Point2D {
    const canvas = canvasRef.current!
    const rect = canvas.getBoundingClientRect()
    return {
      x: Math.min(1, Math.max(0, (e.clientX - rect.left) / rect.width)),
      y: Math.min(1, Math.max(0, (e.clientY - rect.top) / rect.height)),
    }
  }

  function removeSelected() {
    if (selectedIndex == null) return
    onChange(regions.filter((_, i) => i !== selectedIndex))
    setSelectedIndex(null)
  }

  function undoLast() {
    if (!regions.length) return
    onChange(regions.slice(0, -1))
    setSelectedIndex(null)
  }

  const streamHint =
    !streamSrc
      ? '无可用码流'
      : playState === 'error'
        ? '拉流失败，仍可画区域'
        : playState === 'connecting'
          ? '正在拉流…'
          : ''

  const showToolbar = !readOnly

  return (
    <div className="ff-region-editor" ref={wrapRef}>
      {showToolbar ? (
        <div className="ff-region-toolbar">
          {!readOnly ? (
            <>
              <div className="ff-region-group">
                <span className="ff-region-group-label">范围</span>
                <div className="ff-region-tools">
                  <Button
                    label="整帧"
                    size="sm"
                    variant={regions.length === 0 ? 'primary' : 'secondary'}
                    onClick={() => {
                      onChange([])
                      setDraft([])
                      setSelectedIndex(null)
                      setMode('select')
                    }}
                  />
                  <Button
                    label="矩形"
                    size="sm"
                    variant={
                      regions.length > 0 && mode === 'rect'
                        ? 'primary'
                        : 'secondary'
                    }
                    onClick={() => {
                      setMode('rect')
                      setDraft([])
                      setSelectedIndex(null)
                    }}
                  />
                  <Button
                    label="多边形"
                    size="sm"
                    variant={
                      regions.length > 0 && mode === 'poly'
                        ? 'primary'
                        : 'secondary'
                    }
                    onClick={() => {
                      setMode('poly')
                      setDraft([])
                      setSelectedIndex(null)
                    }}
                  />
                  <Button
                    label="选择"
                    size="sm"
                    variant={
                      regions.length > 0 && mode === 'select'
                        ? 'primary'
                        : 'secondary'
                    }
                    onClick={() => {
                      setMode('select')
                      setDraft([])
                    }}
                  />
                  <Button
                    label="完成"
                    size="sm"
                    variant="ghost"
                    isDisabled={mode !== 'poly' || draft.length < 3}
                    onClick={() => {
                      if (draft.length >= 3) {
                        onChange([...regions, draft])
                        setDraft([])
                      }
                    }}
                  />
                  <Button
                    label="撤销"
                    size="sm"
                    variant="ghost"
                    isDisabled={!regions.length}
                    onClick={undoLast}
                  />
                  <Button
                    label="删除"
                    size="sm"
                    variant="ghost"
                    isDisabled={selectedIndex == null}
                    onClick={removeSelected}
                  />
                  <Button
                    label="清空"
                    size="sm"
                    variant="ghost"
                    isDisabled={!regions.length && !draft.length}
                    onClick={() => {
                      onChange([])
                      setDraft([])
                      setSelectedIndex(null)
                    }}
                  />
                </div>
              </div>
              {regions.length === 0 ? (
                <div className="ff-region-group">
                  <span className="ff-region-group-label">当前</span>
                  <ToneBadge tone="off">整帧检测</ToneBadge>
                </div>
              ) : (
                <div className="ff-region-group">
                  <span className="ff-region-group-label">区域</span>
                  <div className="ff-region-chips">
                    {regions.map((_, i) => (
                      <button
                        key={i}
                        type="button"
                        className="ff-region-chip"
                        data-selected={selectedIndex === i ? 'true' : undefined}
                        onClick={() => {
                          setMode('select')
                          setSelectedIndex(i)
                        }}
                      >
                        <span className="ff-region-chip-index">{i + 1}</span>
                        <span>{regions[i].length} 点</span>
                      </button>
                    ))}
                  </div>
                </div>
              )}
              {regions.length === 0 ||
              mode === 'poly' ||
              mode === 'select' ||
              streamHint ? (
                <Text type="supporting" color="secondary">
                  {regions.length === 0
                    ? '整帧检测：不对区域过滤；画矩形/多边形可改为局部'
                    : mode === 'poly'
                      ? '点击加点，再点「完成」'
                      : mode === 'select'
                        ? '点选区域后可删除'
                        : streamHint}
                </Text>
              ) : null}
            </>
          ) : null}
        </div>
      ) : null}

      <div
        ref={stageRef}
        className="ff-region-stage"
        data-live={live ? 'true' : undefined}
      >
        {streamSrc ? (
          <div className="ff-region-video">
            <Go2rtcPlayer
              streamSrc={streamSrc}
              mode="webrtc"
              muted
              className="ff-region-go2rtc"
              onState={setPlayState}
              onVisionFrame={(payload) => {
                if (!showAiOverlay) return
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
          </div>
        ) : null}
        <DetectionOverlay boxes={visionBoxes} visible={showAiOverlay} />
        {showAiOverlay && visionHint ? (
          <div className="ff-region-vision-hint">{visionHint}</div>
        ) : null}
        <canvas
          ref={canvasRef}
          className="ff-region-canvas"
          style={{
            cursor: readOnly ? 'default' : mode === 'select' ? 'pointer' : 'crosshair',
            pointerEvents: readOnly ? 'none' : undefined,
          }}
          onMouseDown={(e) => {
            if (readOnly) return
            const p = toNorm(e)
            if (mode === 'select') {
              let hit: number | null = null
              for (let i = regions.length - 1; i >= 0; i--) {
                if (pointInPoly(p, regions[i])) {
                  hit = i
                  break
                }
              }
              setSelectedIndex(hit)
              return
            }
            if (mode === 'rect') {
              dragStart.current = p
              setDraft([p])
              setSelectedIndex(null)
            } else {
              setDraft((d) => [...d, p])
              setSelectedIndex(null)
            }
          }}
          onMouseMove={(e) => {
            if (readOnly) return
            if (mode !== 'rect' || !dragStart.current) return
            const p = toNorm(e)
            const s = dragStart.current
            setDraft([
              { x: s.x, y: s.y },
              { x: p.x, y: s.y },
              { x: p.x, y: p.y },
              { x: s.x, y: p.y },
            ])
          }}
          onMouseUp={() => {
            if (readOnly) return
            if (mode === 'rect' && draft.length >= 3) {
              const area =
                Math.abs(draft[2].x - draft[0].x) *
                Math.abs(draft[2].y - draft[0].y)
              if (area > 0.002) onChange([...regions, draft])
              setDraft([])
            }
            dragStart.current = null
          }}
          onMouseLeave={() => {
            if (readOnly) return
            if (mode === 'rect' && dragStart.current) {
              dragStart.current = null
              setDraft([])
            }
          }}
        />
      </div>
    </div>
  )
}
