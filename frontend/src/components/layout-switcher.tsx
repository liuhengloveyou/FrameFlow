import type { ReactNode } from 'react'

export type ScreenLayout = '1' | '4' | '9' | '16'

/** 由疏到密的实心方块图标，与设计稿一致 */
function BlockIcon({ n }: { n: ScreenLayout }) {
  const side = n === '1' ? 1 : n === '4' ? 2 : n === '9' ? 3 : 4
  const size = 15
  const gap = 2
  const cell = (size - gap * (side - 1)) / side
  const cells: ReactNode[] = []
  for (let r = 0; r < side; r++) {
    for (let c = 0; c < side; c++) {
      cells.push(
        <rect
          key={`${r}-${c}`}
          x={c * (cell + gap)}
          y={r * (cell + gap)}
          width={cell}
          height={cell}
          rx={side >= 4 ? 0.6 : 1.2}
          fill="currentColor"
        />,
      )
    }
  }
  return (
    <svg
      viewBox={`0 0 ${size} ${size}`}
      width={size}
      height={size}
      fill="none"
      aria-hidden
    >
      {cells}
    </svg>
  )
}

/** 画面布局分段控件：1 / 4 / 9 / 16 路 */
export function LayoutSwitcher({
  value,
  onChange,
  options = ['1', '4', '9', '16'],
}: {
  value: ScreenLayout
  onChange: (v: ScreenLayout) => void
  options?: ScreenLayout[]
}) {
  return (
    <div className="ff-seg" role="radiogroup" aria-label="画面布局">
      {options.map((v) => (
        <button
          key={v}
          type="button"
          role="radio"
          aria-checked={value === v}
          aria-label={`${v} 画面`}
          title={`${v} 画面`}
          className="ff-seg-item"
          data-active={value === v ? 'true' : undefined}
          onClick={() => onChange(v)}
        >
          <BlockIcon n={v} />
        </button>
      ))}
    </div>
  )
}

/** 宫格翻页：‹ 2 / 33 › */
export function GridPager({
  page,
  pageCount,
  onChange,
}: {
  page: number
  pageCount: number
  onChange: (page: number) => void
}) {
  return (
    <div className="ff-grid-pager">
      <button
        type="button"
        className="ff-grid-pager-btn"
        aria-label="上一页"
        disabled={page <= 1}
        onClick={() => onChange(page - 1)}
      >
        <svg viewBox="0 0 16 16" fill="none" aria-hidden>
          <path
            d="M10 3.5 5.5 8l4.5 4.5"
            stroke="currentColor"
            strokeWidth="1.8"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
      </button>
      <span className="ff-grid-pager-page">
        {page} / {Math.max(1, pageCount)}
      </span>
      <button
        type="button"
        className="ff-grid-pager-btn"
        aria-label="下一页"
        disabled={page >= pageCount}
        onClick={() => onChange(page + 1)}
      >
        <svg viewBox="0 0 16 16" fill="none" aria-hidden>
          <path
            d="M6 3.5 10.5 8 6 12.5"
            stroke="currentColor"
            strokeWidth="1.8"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
      </button>
    </div>
  )
}
