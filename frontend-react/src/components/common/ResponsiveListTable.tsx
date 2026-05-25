import { List, Table } from 'antd'
import type { ListProps, TableProps } from 'antd'
import type { Key, ReactNode } from 'react'
import { useEffect, useState } from 'react'
import { MD_BREAKPOINT } from '@/theme'

export interface ResponsiveListTableProps<T extends object> extends TableProps<T> {
  dataSource: T[]
  rowKey: TableProps<T>['rowKey']
  mobileCard: (record: T, index: number) => ReactNode
  listProps?: Omit<ListProps<T>, 'dataSource' | 'renderItem'>
}

function resolveRowKey<T extends object>(rowKey: TableProps<T>['rowKey'], record: T, index: number): Key {
  if (typeof rowKey === 'function') {
    return rowKey(record, index)
  }

  return String(record[rowKey as keyof T] ?? index)
}

export function ResponsiveListTable<T extends object>({
  dataSource,
  rowKey,
  mobileCard,
  listProps,
  ...tableProps
}: ResponsiveListTableProps<T>) {
  const [isMobile, setIsMobile] = useState(() => {
    if (typeof window === 'undefined') {
      return false
    }

    return window.matchMedia(`(max-width: ${MD_BREAKPOINT - 1}px)`).matches
  })

  useEffect(() => {
    const mediaQuery = window.matchMedia(`(max-width: ${MD_BREAKPOINT - 1}px)`)
    const handleChange = () => setIsMobile(mediaQuery.matches)

    handleChange()
    mediaQuery.addEventListener('change', handleChange)
    window.addEventListener('resize', handleChange)

    return () => {
      mediaQuery.removeEventListener('change', handleChange)
      window.removeEventListener('resize', handleChange)
    }
  }, [])

  return (
    <div data-component="responsive-list-table">
      {isMobile ? (
        <List
          {...listProps}
          dataSource={dataSource}
          renderItem={(record, index) => (
            <List.Item key={resolveRowKey(rowKey, record, index)}>{mobileCard(record, index)}</List.Item>
          )}
        />
      ) : (
        <Table<T> {...tableProps} dataSource={dataSource} rowKey={rowKey} />
      )}
    </div>
  )
}
