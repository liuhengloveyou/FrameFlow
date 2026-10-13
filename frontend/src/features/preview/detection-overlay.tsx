import { useEffect, useRef, useState } from 'react'
import type { VisionBox } from '@/lib/vision-types'

/**
 * AI 检测框叠层：坐标为相对画面 0~1。
 *
 * 传入 videoW/videoH 时用视频实际宽高做 viewBox + `xMidYMid meet`：
 * 与 `<video>` 的 `object-fit: contain` 内容盒自动对齐——瓦片长宽比与视频不一致
 * 出现黑边时，框仍精确贴合画面内容（否则会整体拉伸偏移）。
 * 未传入时保持旧行为（铺满容器，适用于容器比例即画面比例的场景，如区域编辑器）。
 *
 * 抗闪烁：后端航迹跟踪（track_id/miss）+ 本组件的位置插值共同实现「一直框着对象」：
 *   - track_id 做稳定 key：React 只更新属性、不重建元素，CSS/属性过渡才可能生效；
 *   - rAF 指数插值（时间常数 TAU_S）：检测坐标逐帧抖动被抹平，框快速跟上新位置；
 *   - 后端 hold 中的框（miss>0）以半透明显示，目标短暂丢失时框原地不消失；
 *   - 航迹真正被删除时本地快速淡出，避免「唰」地消失。
 */
export function DetectionOverlay({
  boxes,
  visible,
  videoW = 0,
  videoH = 0,
}: {
  boxes: VisionBox[]
  visible: boolean
  /** 视频实际像素宽高（videoWidth/videoHeight），用于内容盒对齐；0 = 未知 */
  videoW?: number
  videoH?: number
}) {
  const smoothed = useSmoothedBoxes(visible ? boxes : [])
  if (!visible || !smoothed.length) return null
  const fit = videoW > 0 && videoH > 0
  const sx = fit ? videoW : 1
  const sy = fit ? videoH : 1
  // 标签字号取画面高的 3.5%（视频像素坐标系下 1 单位 = 1 像素）
  const labelSize = fit ? Math.max(10, Math.round(videoH * 0.035)) : 0
  return (
    <svg
      className="ff-detection-overlay"
      viewBox={fit ? `0 0 ${videoW} ${videoH}` : '0 0 1 1'}
      preserveAspectRatio={fit ? 'xMidYMid meet' : 'none'}
      aria-hidden
    >
      {smoothed.map((b) => {
        const label = b.label
        const score = b.score > 0 ? ` ${Math.round(b.score * 100)}%` : ''
        const x = b.x * sx
        const y = b.y * sy
        // 航迹保持中的框（miss>0，原地保持未外推）：半透明，表示「预测中，本拍未真实检出」
        const opacity = b.opacity * (b.predicted ? 0.55 : 1)
        return (
          <g key={b.key} opacity={opacity}>
            <rect
              x={x}
              y={y}
              width={b.w * sx}
              height={b.h * sy}
              className="ff-detection-box"
            />
            {label ? (
              <text
                x={x}
                y={Math.max(labelSize * 0.6, y - labelSize * 0.2)}
                className="ff-detection-label"
                style={labelSize ? { fontSize: labelSize } : undefined}
              >
                {label}
                {score}
              </text>
            ) : null}
          </g>
        )
      })}
    </svg>
  )
}

/** 位置插值时间常数（秒）：越小跟得越紧，越大越滑。40ms ≈  detection 15fps 下小于半拍，
 *  只抹逐帧抖动、不产生可见拖影。 */
const TAU_S = 0.04
const FADE_IN_S = 0.1
const FADE_OUT_S = 0.12
/** 位置差小于该值（归一化）视为已到位，可停掉 rAF。 */
const SETTLE_EPS = 1e-4

type SmoothedBox = {
  key: string
  x: number
  y: number
  w: number
  h: number
  label: string
  score: number
  /** 0~1 淡入淡出进度 */
  opacity: number
  /** 后端航迹保持中的框（miss>0，停在最后检出位置） */
  predicted: boolean
}

type TrackedState = SmoothedBox & {
  tx: number
  ty: number
  tw: number
  th: number
}

/**
 * 把逐帧检测框平滑成连续渲染框：
 * 新目标从目标位置淡入；已有目标指数插值滑向新位置；目标消失先淡出再移除。
 * 稳定 key：优先用后端 track_id；没有时（跟踪关闭/旧协议）按「标签 + 最近中心」
 * 复用已有框，同样避免逐帧跳变。
 */
function useSmoothedBoxes(boxes: VisionBox[]): SmoothedBox[] {
  const targetsRef = useRef(new Map<string, VisionBox>())
  const stateRef = useRef(new Map<string, TrackedState>())
  const rafRef = useRef(0)
  const lastTsRef = useRef(0)
  const synthIdRef = useRef(1)
  const [, forceRender] = useState(0)

  const stop = () => {
    if (rafRef.current) {
      cancelAnimationFrame(rafRef.current)
      rafRef.current = 0
    }
    lastTsRef.current = 0
  }

  useEffect(() => stop, [])

  // 新 payload：更新目标集合，并给新目标建立渲染状态
  useEffect(() => {
    const state = stateRef.current
    const next = new Map<string, VisionBox>()
    const used = new Set<string>()
    for (const b of boxes) {
      let key: string
      if (typeof b.track_id === 'number' && b.track_id > 0) {
        key = `t${b.track_id}`
      } else {
        // 无 track_id：按标签相同 + 中心最近 复用已有框
        key = ''
        let bestDist = 0.25
        for (const [k, s] of state) {
          if (used.has(k) || k.startsWith('t')) continue
          const dcx = b.x + b.w / 2 - (s.tx + s.tw / 2)
          const dcy = b.y + b.h / 2 - (s.ty + s.th / 2)
          const d = Math.hypot(dcx, dcy)
          if (d < bestDist) {
            bestDist = d
            key = k
          }
        }
        if (!key) key = `n${synthIdRef.current++}`
      }
      used.add(key)
      next.set(key, b)
      const s = state.get(key)
      if (s) {
        s.tx = b.x
        s.ty = b.y
        s.tw = b.w
        s.th = b.h
        s.label = (b.text || b.label || '').trim()
        s.score = typeof b.score === 'number' ? b.score : 0
        s.predicted = (b.miss ?? 0) > 0
      } else {
        state.set(key, {
          key,
          x: b.x,
          y: b.y,
          w: b.w,
          h: b.h,
          tx: b.x,
          ty: b.y,
          tw: b.w,
          th: b.h,
          label: (b.text || b.label || '').trim(),
          score: typeof b.score === 'number' ? b.score : 0,
          opacity: 0,
          predicted: (b.miss ?? 0) > 0,
        })
      }
    }
    targetsRef.current = next
    if (!rafRef.current) {
      rafRef.current = requestAnimationFrame(tick)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [boxes])

  function tick(ts: number) {
    const dt = lastTsRef.current
      ? Math.min(0.12, (ts - lastTsRef.current) / 1000)
      : 1 / 60
    lastTsRef.current = ts
    const targets = targetsRef.current
    const state = stateRef.current
    const alpha = 1 - Math.exp(-dt / TAU_S)
    let moving = false
    for (const [k, s] of state) {
      const t = targets.get(k)
      if (t) {
        s.x += (t.x - s.x) * alpha
        s.y += (t.y - s.y) * alpha
        s.w += (t.w - s.w) * alpha
        s.h += (t.h - s.h) * alpha
        s.opacity = Math.min(1, s.opacity + dt / FADE_IN_S)
        if (
          Math.abs(t.x - s.x) > SETTLE_EPS ||
          Math.abs(t.y - s.y) > SETTLE_EPS ||
          Math.abs(t.w - s.w) > SETTLE_EPS ||
          Math.abs(t.h - s.h) > SETTLE_EPS ||
          s.opacity < 1
        ) {
          moving = true
        }
      } else {
        s.opacity -= dt / FADE_OUT_S
        if (s.opacity > 0.001) {
          moving = true
        } else {
          state.delete(k)
        }
      }
    }
    forceRender((v) => v + 1)
    if (moving) {
      rafRef.current = requestAnimationFrame(tick)
    } else {
      rafRef.current = 0
      lastTsRef.current = 0
    }
  }

  const out: SmoothedBox[] = []
  for (const s of stateRef.current.values()) {
    if (s.opacity <= 0) continue
    out.push(s)
  }
  return out
}
