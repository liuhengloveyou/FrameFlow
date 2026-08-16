import type { VisionBox } from '@/lib/vision-types'

/** AI 检测框叠层：坐标为相对画面 0~1 */
export function DetectionOverlay({
  boxes,
  visible,
}: {
  boxes: VisionBox[]
  visible: boolean
}) {
  if (!visible || !boxes.length) return null
  return (
    <svg
      className="ff-detection-overlay"
      viewBox="0 0 1 1"
      preserveAspectRatio="none"
      aria-hidden
    >
      {boxes.map((b, i) => {
        const label = (b.text || b.label || '').trim()
        const score =
          typeof b.score === 'number' && b.score > 0
            ? ` ${Math.round(b.score * 100)}%`
            : ''
        return (
          <g key={`${i}-${b.x}-${b.y}`}>
            <rect
              x={b.x}
              y={b.y}
              width={b.w}
              height={b.h}
              className="ff-detection-box"
            />
            {label ? (
              <text
                x={b.x}
                y={Math.max(0.02, b.y - 0.01)}
                className="ff-detection-label"
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
