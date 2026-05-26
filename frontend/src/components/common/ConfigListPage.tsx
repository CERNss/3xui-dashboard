import type { ReactNode } from 'react'
import { EmptyState, type EmptyStateProps } from './EmptyState'
import { PageHeader, type PageHeaderProps } from './PageHeader'
import { ResponsiveListTable, type ResponsiveListTableProps } from './ResponsiveListTable'

export interface ConfigListPageProps<T extends object>
  extends Omit<ResponsiveListTableProps<T>, 'dataSource' | 'footer' | 'title'> {
  actions?: PageHeaderProps['actions']
  alerts?: ReactNode
  dataSource: T[]
  emptyContent?: ReactNode
  emptyState?: EmptyStateProps
  filters?: ReactNode
  footer?: ReactNode
  header?: ReactNode
  loading?: boolean
  stats?: ReactNode
  subtitle?: PageHeaderProps['subtitle']
  title?: PageHeaderProps['title']
}

export function ConfigListPage<T extends object>({
  actions,
  alerts,
  dataSource,
  emptyContent,
  emptyState,
  filters,
  footer,
  header,
  loading,
  stats,
  subtitle,
  title,
  ...tableProps
}: ConfigListPageProps<T>) {
  const hasRows = dataSource.length > 0
  const showTable = hasRows || Boolean(loading)

  return (
    <div data-component="config-list-page">
      {header ?? (title ? <PageHeader title={title} subtitle={subtitle} actions={actions} /> : null)}
      {filters ? <div className="config-list-page-filters">{filters}</div> : null}
      {stats ? <div className="config-list-page-stats">{stats}</div> : null}
      {alerts ? <div className="config-list-page-alerts">{alerts}</div> : null}
      {showTable ? (
        <ResponsiveListTable {...tableProps} dataSource={dataSource} loading={loading} />
      ) : emptyContent ? (
        emptyContent
      ) : emptyState ? (
        <EmptyState {...emptyState} />
      ) : (
        null
      )}
      {footer ? <div className="config-list-page-footer">{footer}</div> : null}
    </div>
  )
}
