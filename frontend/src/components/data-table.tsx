import React, { useEffect, useMemo, useState, type ReactNode } from 'react'
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableCell,
  TableHeaderCell,
  proportional,
  pixel,
  resolveColumnWidths,
  type TableColumn,
} from '@astryxdesign/core/Table'
import { HStack } from '@astryxdesign/core/HStack'
import { Badge } from '@astryxdesign/core/Badge'
import { Icon } from '@astryxdesign/core/Icon'
import { Text } from '@astryxdesign/core/Text'
import { ChevronRightIcon, ChevronDownIcon } from '@heroicons/react/24/outline'

export { proportional, pixel }
export type { TableColumn }

const groupHeaderCell: React.CSSProperties = {
  cursor: 'pointer',
  backgroundColor: 'var(--color-background-muted)',
  padding: 'var(--spacing-2) var(--spacing-3)',
}

export type DataTableColumn<T> = TableColumn<T & Record<string, unknown>> & {
  render?: (row: T) => ReactNode
  /** 列对齐，默认 start */
  align?: 'start' | 'center' | 'end'
}

type DataTableProps<T> = {
  columns: DataTableColumn<T>[]
  data: T[]
  idKey: keyof T & string
  /** 按字段分组；不传则平铺 */
  groupBy?: (keyof T & string) | null
  groupLabel?: (key: string) => string
  emptyLabel?: string
  density?: 'compact' | 'balanced' | 'spacious'
  /** 斑马纹行（Astryx Table isStriped） */
  isStriped?: boolean
  /** 分隔线：rows | columns | grid | none */
  dividers?: 'rows' | 'columns' | 'grid' | 'none'
  /** 固定首列 / 末列（横向滚动时） */
  stickyEdges?: boolean
  showHeader?: boolean
  onRowClick?: (row: T) => void
  className?: string
}

/** 基于 Astryx Table 的分组表格；强制 fixed 布局保证表头/单元格列对齐 */
export function DataTable<T extends object>({
  columns,
  data,
  idKey,
  groupBy = null,
  groupLabel,
  emptyLabel = '暂无数据',
  density = 'balanced',
  isStriped = false,
  dividers = 'rows',
  stickyEdges = false,
  showHeader = true,
  onRowClick,
  className,
}: DataTableProps<T>) {
  const grouped = useMemo(() => {
    if (!groupBy) return new Map([['__all__', data]])
    const map = new Map<string, T[]>()
    for (const row of data) {
      const raw = (row as Record<string, unknown>)[groupBy]
      const key = raw == null || raw === '' ? '—' : String(raw)
      const list = map.get(key) ?? []
      list.push(row)
      map.set(key, list)
    }
    return map
  }, [data, groupBy])

  const groupKeys = useMemo(() => Array.from(grouped.keys()), [grouped])
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set(groupKeys))

  useEffect(() => {
    setExpanded(new Set(groupKeys))
  }, [groupKeys])

  const tableColumns = columns as TableColumn<Record<string, unknown>>[]
  const resolvedWidths = useMemo(
    () => resolveColumnWidths(tableColumns),
    // columns 引用随渲染变化；按 key/width 签名稳定即可
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [columns.map((c) => `${c.key}:${JSON.stringify(c.width)}`).join('|')],
  )
  const colCount = columns.length
  const showGroups = Boolean(groupBy)

  function toggle(key: string) {
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
  }

  function colStyle(key: string): React.CSSProperties {
    return resolvedWidths.columns.get(key)?.style ?? {}
  }

  return (
    <div
      className={[
        'ff-data-table',
        stickyEdges ? 'ff-data-table--sticky-edges' : '',
        className,
      ]
        .filter(Boolean)
        .join(' ')}
      style={
        {
          '--ff-table-min-width': `${resolvedWidths.tableMinWidth}px`,
        } as React.CSSProperties
      }
    >
      <Table
        columns={tableColumns}
        density={density}
        dividers={dividers}
        textOverflow="truncate"
        hasHover
        isStriped={isStriped}
      >
        <colgroup>
          {columns.map((col) => (
            <col key={col.key} style={colStyle(col.key)} />
          ))}
        </colgroup>

        {showHeader ? (
          <TableHeader>
            <TableRow isHeaderRow>
              {columns.map((col) => (
                <TableHeaderCell
                  key={col.key}
                  style={{
                    ...colStyle(col.key),
                    textAlign: col.align ?? 'start',
                  }}
                >
                  {col.header ?? col.key}
                </TableHeaderCell>
              ))}
            </TableRow>
          </TableHeader>
        ) : null}

        <TableBody>
          {data.length === 0 ? (
            <TableRow>
              <TableCell colSpan={colCount}>
                <Text type="supporting" color="secondary">
                  {emptyLabel}
                </Text>
              </TableCell>
            </TableRow>
          ) : (
            groupKeys.map((key) => {
              const rows = grouped.get(key) ?? []
              if (!rows.length) return null
              const isExpanded = !showGroups || expanded.has(key)
              return (
                <React.Fragment key={key}>
                  {showGroups ? (
                    <TableRow
                      role="button"
                      tabIndex={0}
                      onClick={() => toggle(key)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter' || e.key === ' ') {
                          e.preventDefault()
                          toggle(key)
                        }
                      }}
                    >
                      <TableCell colSpan={colCount} style={groupHeaderCell}>
                        <HStack gap={2} vAlign="center">
                          <Icon
                            icon={
                              isExpanded ? ChevronDownIcon : ChevronRightIcon
                            }
                            size="sm"
                            color="secondary"
                          />
                          <Text type="body" weight="bold">
                            {groupLabel ? groupLabel(key) : key}
                          </Text>
                          <Badge
                            variant="neutral"
                            label={String(rows.length)}
                          />
                        </HStack>
                      </TableCell>
                    </TableRow>
                  ) : null}
                  {isExpanded
                    ? rows.map((row) => (
                        <TableRow
                          key={String(row[idKey])}
                          onClick={
                            onRowClick ? () => onRowClick(row) : undefined
                          }
                        >
                          {columns.map((col) => (
                            <TableCell
                              key={col.key}
                              style={{
                                ...colStyle(col.key),
                                textAlign: col.align ?? 'start',
                              }}
                            >
                              {col.render
                                ? col.render(row)
                                : String(
                                    (row as Record<string, unknown>)[
                                      col.key
                                    ] ?? '—',
                                  )}
                            </TableCell>
                          ))}
                        </TableRow>
                      ))
                    : null}
                </React.Fragment>
              )
            })
          )}
        </TableBody>
      </Table>
    </div>
  )
}
