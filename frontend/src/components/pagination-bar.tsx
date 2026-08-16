import { Button } from '@astryxdesign/core/Button'
import { Selector } from '@astryxdesign/core/Selector'
import { Text } from '@astryxdesign/core/Text'

const DEFAULT_PAGE_SIZE_OPTIONS = [10, 20, 50]

type PaginationBarProps = {
  page: number
  pageSize: number
  total: number
  onPageChange: (page: number) => void
  pageSizeOptions?: number[]
  onPageSizeChange?: (pageSize: number) => void
}

export function PaginationBar({
  page,
  pageSize,
  total,
  onPageChange,
  pageSizeOptions = DEFAULT_PAGE_SIZE_OPTIONS,
  onPageSizeChange,
}: PaginationBarProps) {
  const totalPages = Math.max(1, Math.ceil(total / pageSize))
  const from = total === 0 ? 0 : (page - 1) * pageSize + 1
  const to = Math.min(total, page * pageSize)

  return (
    <div className="ff-pagination-bar">
      <Text type="supporting" color="secondary">
        共 {total} 条 · 第 {from}-{to} 条
      </Text>

      <div className="ff-pagination-bar-controls">
        {onPageSizeChange ? (
          <div className="ff-pagination-bar-size">
            <Selector
              label="每页条数"
              isLabelHidden
              value={String(pageSize)}
              onChange={(v) => {
                const next = Number(v)
                if (next > 0) onPageSizeChange(next)
              }}
              options={pageSizeOptions.map((n) => ({
                label: `${n} 条/页`,
                value: String(n),
              }))}
            />
          </div>
        ) : null}
        <Button
          label="上一页"
          size="sm"
          variant="secondary"
          isDisabled={page <= 1}
          onClick={() => onPageChange(page - 1)}
        />
        <Text type="supporting">
          <span className="ff-pagination-bar-page">
            {page} / {totalPages}
          </span>
        </Text>
        <Button
          label="下一页"
          size="sm"
          variant="secondary"
          isDisabled={page >= totalPages}
          onClick={() => onPageChange(page + 1)}
        />
      </div>
    </div>
  )
}
