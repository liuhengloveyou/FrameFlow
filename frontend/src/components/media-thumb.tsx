import { useMemo } from 'react'

const FALLBACKS = [
  '/images/cctv-dock.png',
  '/images/cctv-perimeter.png',
  '/images/cctv-warehouse.png',
]

/**
 * 证据/搜索结果缩略图：16:9，无图时按稳定种子轮换占位图，
 * 保证列表视觉不出现破图。
 */
export function MediaThumb({
  src,
  alt = '',
  seed,
}: {
  src?: string
  alt?: string
  seed?: string
}) {
  const fallback = useMemo(() => {
    let h = 0
    const key = seed ?? src ?? ''
    for (let i = 0; i < key.length; i++) h = (h * 31 + key.charCodeAt(i)) | 0
    return FALLBACKS[Math.abs(h) % FALLBACKS.length]
  }, [seed, src])
  return (
    <div className="ff-media-thumb">
      <img src={src || fallback} alt={alt} loading="lazy" />
    </div>
  )
}
