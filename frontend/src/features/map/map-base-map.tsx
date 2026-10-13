import type { MapLayer } from '@/types/api'

/**
 * 示例底图：纯装饰性的园区 / 楼层 / 停车场示意图。
 *
 * 设计约束（和 `map-page.tsx` 的坐标系严格对齐）：
 * - 所有坐标都是画布百分比，与 `.ff-map-pin` / `.ff-map-shape` 用的 `left/top` 百分比同一套空间，
 *   所以用户把摄像头拖到「一号楼」上，位置在视觉上就是对的。
 * - 因此底图不用 SVG —— SVG 在非等比缩放下会把文字拉扁；这里用绝对定位的 div，
 *   文字是原生排版不会被拉伸，只有矩形/圆角被拉伸（看不出来）。
 * - 底图整体 `pointer-events: none`，不参与交互；交互只属于点位与图形。
 * - 它是**演示底图**，不是真实地理数据，UI 上必须明确标注（见页面上的「示例底图」开关）。
 */

type Kind =
  | 'green'
  | 'road'
  | 'slab'
  | 'water'
  | 'building'
  | 'utility'
  | 'plaza'
  | 'floor'
  | 'room'
  | 'corridor'
  | 'accent'
  | 'elevator'
  | 'stairs'
  | 'furn'
  | 'screen'
  | 'ev'
  | 'accessible'
  | 'fence'
  | 'gate'
  | 'opening'
  | 'erase'
  | 'arrow-up'
  | 'arrow-down'

type BoxItem = {
  x: number
  y: number
  w: number
  h: number
  kind: Kind
  label?: string
  sub?: string
  /** 房间名靠上排（下方留给家具图形） */
  lp?: 'top'
  /** 圆角（px）；>=999 视为全圆角 */
  r?: number
  dashed?: boolean
}

type LineItem = {
  x1: number
  y1: number
  x2: number
  y2: number
  weight?: number
  dashed?: boolean
  tone?: 'road' | 'stall' | 'wall' | 'lane'
}

type DotItem = { x: number; y: number; size?: number; pine?: boolean }

type TagItem = {
  x: number
  y: number
  text: string
  tone?: 'muted' | 'ink' | 'accent'
  size?: number
  align?: 'left' | 'center' | 'right'
}

type BaseMap = {
  boxes: BoxItem[]
  lines: LineItem[]
  dots: DotItem[]
  tags: TagItem[]
}

/* ---------------- 园区总览 ---------------- */

/** 车位刻度：园区视角只示意两排车位，不画编号 */
function lotTicks(x0: number, span: number, count: number, rows: [number, number][]): LineItem[] {
  const out: LineItem[] = []
  for (let i = 0; i <= count; i++) {
    const x = x0 + (i * span) / count
    for (const [y1, y2] of rows) out.push({ x1: x, y1, x2: x, y2, tone: 'stall' })
  }
  return out
}

const CAMPUS: BaseMap = {
  boxes: [
    // 绿化围合带（沿围墙内侧）
    { x: 3.5, y: 5, w: 93, h: 4, kind: 'green', r: 6 },
    { x: 3.5, y: 91, w: 93, h: 4, kind: 'green', r: 6 },
    { x: 3.5, y: 5, w: 3.4, h: 90, kind: 'green', r: 6 },
    { x: 93.1, y: 5, w: 3.4, h: 90, kind: 'green', r: 6 },

    // 道路：东西主路 + 南向引道（通南门）；两端出画布，避免出现断头路
    { x: -4, y: 38.5, w: 108, h: 11.5, kind: 'road' },
    { x: 44.6, y: 50, w: 10.8, h: 54, kind: 'road' },

    // 建筑
    { x: 6, y: 12, w: 34, h: 24, kind: 'building', label: '一号楼', sub: '办公', r: 6 },
    { x: 60, y: 12, w: 34, h: 24, kind: 'building', label: '二号楼', sub: '研发', r: 6 },
    { x: 6, y: 55, w: 34, h: 24, kind: 'building', label: '三号楼', sub: '仓储', r: 6 },

    // 中央广场 + 景观池
    { x: 41, y: 12, w: 17, h: 25, kind: 'plaza', r: 6 },
    { x: 44.4, y: 17, w: 10.2, h: 10.5, kind: 'water', r: 999 },

    // 露天停车场（园区视角只示意）
    { x: 62, y: 55, w: 32, h: 33, kind: 'slab', r: 5 },
    { x: 63.4, y: 68.4, w: 29.2, h: 5.6, kind: 'road', r: 3 },

    // 附属房 / 门卫
    { x: 6, y: 82, w: 11, h: 6.6, kind: 'utility', label: '配电房', r: 4 },
    { x: 18.5, y: 82, w: 11, h: 6.6, kind: 'utility', label: '水泵房', r: 4 },
    { x: 37.2, y: 82.4, w: 6.4, h: 7.2, kind: 'utility', label: '门卫', r: 4 },
    { x: 56.4, y: 82.4, w: 5.2, h: 7.2, kind: 'utility', label: '门卫', r: 4 },

    // 围墙 + 南门开口（先铺底再挖口）
    { x: 2.6, y: 4, w: 94.8, h: 92, kind: 'fence', r: 8, dashed: true },
    { x: 42.6, y: 93, w: 14.8, h: 5, kind: 'erase' },
    { x: 42.8, y: 92.6, w: 1.5, h: 5.6, kind: 'gate' },
    { x: 55.7, y: 92.6, w: 1.5, h: 5.6, kind: 'gate' },
  ],
  lines: [
    // 道路中线
    { x1: 0, y1: 44.2, x2: 100, y2: 44.2, tone: 'road', weight: 2, dashed: true },
    { x1: 50, y1: 51, x2: 50, y2: 100, tone: 'road', weight: 2, dashed: true },
    // 南门道闸
    { x1: 44.4, y1: 95.2, x2: 55.6, y2: 95.2, tone: 'wall', weight: 2, dashed: true },
    ...lotTicks(63.4, 29.2, 11, [
      [57.6, 67.6],
      [74.8, 85.6],
    ]),
  ],
  dots: [
    // 北侧绿化带
    { x: 9, y: 7 },
    { x: 16, y: 6.7 },
    { x: 23, y: 7.3, pine: true },
    { x: 30, y: 7 },
    { x: 37, y: 7.4 },
    { x: 63, y: 7 },
    { x: 70, y: 7.4, pine: true },
    { x: 77, y: 6.8 },
    { x: 84, y: 7.3 },
    { x: 91, y: 7 },
    // 南侧绿化带（避开南门）
    { x: 12, y: 92.8 },
    { x: 22, y: 93.2, pine: true },
    { x: 32, y: 92.8 },
    { x: 68, y: 93.1 },
    { x: 78, y: 92.8, pine: true },
    { x: 88, y: 93.2 },
    // 东侧绿化带
    { x: 94.8, y: 14 },
    { x: 94.8, y: 22, pine: true },
    { x: 94.8, y: 30 },
    { x: 94.8, y: 58 },
    { x: 94.8, y: 66, pine: true },
    { x: 94.8, y: 74 },
    // 西侧绿化带
    { x: 5.2, y: 14, pine: true },
    { x: 5.2, y: 22 },
    { x: 5.2, y: 30, pine: true },
    { x: 5.2, y: 58 },
    { x: 5.2, y: 66, pine: true },
    { x: 5.2, y: 74 },
    // 中央广场点缀
    { x: 42.8, y: 14.4 },
    { x: 56.2, y: 14.4, pine: true },
    { x: 42.8, y: 34.6 },
    { x: 56.2, y: 34.6, pine: true },
  ],
  tags: [
    { x: 49.7, y: 22.4, text: '景观池', size: 9.5 },
    { x: 49.5, y: 33.8, text: '中央广场', size: 10 },
    { x: 78, y: 52.6, text: '露天停车场', size: 10 },
    { x: 20, y: 47.4, text: '园区主路', size: 9.5 },
  ],
}

/* ---------------- 一号楼内部 ---------------- */

const BUILDING: BaseMap = {
  boxes: [
    { x: 4, y: 7, w: 92, h: 86, kind: 'floor', r: 6 },
    { x: 6, y: 44, w: 88, h: 12, kind: 'corridor', r: 3 },

    // 北侧房间
    { x: 6, y: 9, w: 16, h: 35, kind: 'room', label: '前台 / 接待', lp: 'top' },
    { x: 22, y: 9, w: 25, h: 35, kind: 'accent', label: '监控中心', sub: '常驻值班' },
    { x: 47, y: 9, w: 17, h: 35, kind: 'room', label: '会议室 A', lp: 'top' },
    { x: 64, y: 9, w: 16, h: 35, kind: 'room', label: '会议室 B', lp: 'top' },
    { x: 80, y: 9, w: 14, h: 35, kind: 'room', label: '电梯厅' },

    // 南侧房间
    { x: 6, y: 56, w: 13, h: 35, kind: 'room', label: '配电间' },
    { x: 19, y: 56, w: 22, h: 35, kind: 'room', label: '办公区 A', lp: 'top' },
    { x: 41, y: 56, w: 22, h: 35, kind: 'room', label: '办公区 B', lp: 'top' },
    { x: 63, y: 56, w: 12, h: 35, kind: 'room', label: '茶水间', lp: 'top' },
    { x: 75, y: 56, w: 10, h: 35, kind: 'room', label: '楼梯间' },
    { x: 85, y: 56, w: 9, h: 35, kind: 'room', label: '卫生间' },

    // 家具与设备（让平面图不像空盒子）
    { x: 25.6, y: 12.4, w: 17.8, h: 4.6, kind: 'screen' }, // 监控大屏
    { x: 8, y: 26, w: 12, h: 4.4, kind: 'furn' }, // 前台柜台
    { x: 52.5, y: 24, w: 6, h: 13, kind: 'furn' }, // 会议桌 A
    { x: 68.8, y: 24, w: 6, h: 13, kind: 'furn' }, // 会议桌 B
    { x: 64.4, y: 62, w: 9.2, h: 3.6, kind: 'furn' }, // 茶水间操作台
    ...[0, 1].map((c) =>
      [0, 1, 2].map((r) => ({
        x: 20.6 + c * 10.8,
        y: 66 + r * 8,
        w: 7.6,
        h: 4.4,
        kind: 'furn' as const,
      })),
    ).flat(),
    ...[0, 1].map((c) =>
      [0, 1, 2].map((r) => ({
        x: 42.6 + c * 10.8,
        y: 66 + r * 8,
        w: 7.6,
        h: 4.4,
        kind: 'furn' as const,
      })),
    ).flat(),

    // 井道与楼梯
    { x: 81.6, y: 13, w: 4.6, h: 6, kind: 'elevator' },
    { x: 87.6, y: 13, w: 4.6, h: 6, kind: 'elevator' },
    { x: 76.4, y: 62, w: 7.2, h: 22, kind: 'stairs' },

    // 主入口：在墙上开洞 + 两侧墙垛
    { x: 45, y: 92.1, w: 10, h: 2, kind: 'opening' },
    { x: 44.5, y: 91.4, w: 0.7, h: 3.4, kind: 'gate' },
    { x: 54.8, y: 91.4, w: 0.7, h: 3.4, kind: 'gate' },
  ],
  lines: [],
  dots: [],
  tags: [
    { x: 50, y: 4.6, text: '2F 平面图', tone: 'ink', size: 10 },
    { x: 20, y: 50, text: '中央走廊', size: 10 },
    { x: 50, y: 97, text: '主入口', size: 10 },
  ],
}

/* ---------------- 停车场 ---------------- */

const LOT_X = 5
const LOT_W = 77
const STALLS = 12

function stallNumberTags(): TagItem[] {
  const out: TagItem[] = []
  const step = LOT_W / STALLS
  for (let i = 0; i < STALLS; i++) {
    const x = LOT_X + (i + 0.5) * step
    out.push({ x, y: 36.4, text: `A${String(i + 1).padStart(2, '0')}`, size: 9 })
    out.push({ x, y: 57.6, text: `B${String(i + 1).padStart(2, '0')}`, size: 9 })
  }
  return out
}

const PARKING: BaseMap = {
  boxes: [
    // 场坪
    { x: 4, y: 11, w: 92, h: 76, kind: 'slab', r: 8 },
    // 车道：横向主道 + 右侧纵向引道
    { x: LOT_X, y: 40, w: LOT_W, h: 14, kind: 'road', r: 3 },
    { x: 82, y: 12, w: 10, h: 74, kind: 'road', r: 4 },
    // 上下两排车位地面
    { x: LOT_X, y: 13, w: LOT_W, h: 27, kind: 'plaza', r: 3 },
    { x: LOT_X, y: 54, w: LOT_W, h: 27, kind: 'plaza', r: 3 },
    // 充电车位（上排前 4 个）/ 无障碍车位（下排最后 2 个）
    ...[0, 1, 2, 3].map((i) => ({
      x: LOT_X + i * (LOT_W / STALLS) + 0.2,
      y: 13.5,
      w: LOT_W / STALLS - 0.4,
      h: 26,
      kind: 'ev' as const,
    })),
    ...[10, 11].map((i) => ({
      x: LOT_X + i * (LOT_W / STALLS) + 0.2,
      y: 54.5,
      w: LOT_W / STALLS - 0.4,
      h: 26,
      kind: 'accessible' as const,
    })),
    // 岗亭 + 道闸 + 出入口箭头
    { x: 82, y: 88, w: 12, h: 8, kind: 'utility', label: '岗亭', r: 4 },
    { x: 89.2, y: 72, w: 1.7, h: 5.2, kind: 'arrow-up' },
    { x: 83.4, y: 72, w: 1.7, h: 5.2, kind: 'arrow-down' },
  ],
  lines: [
    ...lotTicks(LOT_X, LOT_W, STALLS, [
      [13, 40],
      [54, 81],
    ]),
    { x1: LOT_X, y1: 47, x2: 82, y2: 47, tone: 'road', weight: 2, dashed: true },
    { x1: 87, y1: 12, x2: 87, y2: 86, tone: 'road', weight: 2, dashed: true },
    { x1: 82, y1: 86.4, x2: 92, y2: 86.4, tone: 'wall', weight: 2, dashed: true },
  ],
  dots: [],
  tags: [
    { x: 5.5, y: 84, text: '露天停车场 · 24 车位', tone: 'ink', size: 11, align: 'left' },
    { x: 18, y: 12.2, text: '充电车位', size: 9 },
    { x: 72.5, y: 60.6, text: '无障碍', size: 9 },
    { x: 90, y: 80.6, text: '入口', size: 9 },
    { x: 84.2, y: 80.6, text: '出口', size: 9 },
    ...stallNumberTags(),
  ],
}

const BASE_MAPS: Record<MapLayer, BaseMap> = {
  campus: CAMPUS,
  building: BUILDING,
  parking: PARKING,
}

export function MapBaseMap({ layer }: { layer: MapLayer }) {
  const map = BASE_MAPS[layer]
  if (!map) return null

  return (
    <div className="ff-map-base" data-layer={layer}>
      {map.boxes.map((b, i) => (
        <div
          key={`box-${i}`}
          className="ff-bm ff-bm-box"
          data-kind={b.kind}
          data-dashed={b.dashed ? 'true' : undefined}
          data-lp={b.lp}
          style={{
            left: `${b.x}%`,
            top: `${b.y}%`,
            width: `${b.w}%`,
            height: `${b.h}%`,
            borderRadius: b.r == null ? undefined : b.r >= 999 ? '999px' : `${b.r}px`,
          }}
        >
          {b.label ? (
            <span className="ff-bm-text">
              <span className="ff-bm-name">{b.label}</span>
              {b.sub ? <span className="ff-bm-sub">{b.sub}</span> : null}
            </span>
          ) : null}
        </div>
      ))}

      {map.lines.map((l, i) => {
        const horizontal = Math.abs(l.y2 - l.y1) < 0.001
        return (
          <div
            key={`line-${i}`}
            className="ff-bm ff-bm-line"
            data-tone={l.tone ?? 'wall'}
            data-dashed={l.dashed ? 'true' : undefined}
            data-dir={horizontal ? 'h' : 'v'}
            style={
              horizontal
                ? {
                    left: `${Math.min(l.x1, l.x2)}%`,
                    top: `${l.y1}%`,
                    width: `${Math.abs(l.x2 - l.x1)}%`,
                    height: `${l.weight ?? 1}px`,
                  }
                : {
                    left: `${l.x1}%`,
                    top: `${Math.min(l.y1, l.y2)}%`,
                    width: `${l.weight ?? 1}px`,
                    height: `${Math.abs(l.y2 - l.y1)}%`,
                  }
            }
          />
        )
      })}

      {map.tags.map((t, i) => (
        <span
          key={`tag-${i}`}
          className="ff-bm ff-bm-tag"
          data-tone={t.tone ?? 'muted'}
          style={{
            left: `${t.x}%`,
            top: `${t.y}%`,
            fontSize: `${t.size ?? 10}px`,
            transform:
              t.align === 'left'
                ? 'translateY(-50%)'
                : t.align === 'right'
                  ? 'translate(-100%, -50%)'
                  : 'translate(-50%, -50%)',
          }}
        >
          {t.text}
        </span>
      ))}

      {map.dots.map((d, i) => (
        <span
          key={`dot-${i}`}
          className="ff-bm ff-bm-dot"
          data-pine={d.pine ? 'true' : undefined}
          style={{
            left: `${d.x}%`,
            top: `${d.y}%`,
            width: `${d.size ?? 1.3}%`,
          }}
        />
      ))}
    </div>
  )
}
