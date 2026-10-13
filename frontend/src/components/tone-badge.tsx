import type { ReactNode } from 'react'

/**
 * 全站统一徽章形态：小圆点 + 文字 + 语义底色（颜色/文字双重编码）。
 * 事件类型、处置状态、报警级别、投递结果、授权状态……全部走这一个积木，
 * 保证不同页面里「同一个语义 = 同一个颜色」。
 */
export type Tone = 'warn' | 'info' | 'ok' | 'off' | 'danger' | 'teal'

export function ToneBadge({
  tone = 'off',
  children,
  title,
}: {
  tone?: Tone
  children: ReactNode
  title?: string
}) {
  return (
    <span className="ff-ev-badge" data-tone={tone} title={title}>
      {children}
    </span>
  )
}
