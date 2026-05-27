import { Space } from 'antd'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { EmptyState, type EmptyStateProps } from './EmptyState'
import { PageHeader, type PageHeaderProps } from './PageHeader'
import { ResponsiveListTable, type ResponsiveListTableProps } from './ResponsiveListTable'

export interface ConfigListPageProps<T extends object>
  extends Partial<Omit<ResponsiveListTableProps<T>, 'dataSource' | 'footer' | 'title'>> {
  actions?: PageHeaderProps['actions']
  alerts?: ReactNode
  dataSource?: T[]
  emptyContent?: ReactNode
  emptyState?: EmptyStateProps
  filters?: ReactNode
  footer?: ReactNode
  header?: ReactNode
  listClassName?: string
  listContent?: ReactNode
  loading?: boolean
  stats?: ReactNode
  subtitle?: PageHeaderProps['subtitle']
  title?: PageHeaderProps['title']
  viewport?: boolean
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
  listClassName,
  listContent,
  loading,
  mobileCard,
  rowKey,
  stats,
  subtitle,
  title,
  viewport = true,
  ...tableProps
}: ConfigListPageProps<T>) {
  const { t } = useTranslation()
  const rows = dataSource ?? []
  const hasRows = rows.length > 0
  const showTable = hasRows || Boolean(loading)
  const pageClassName = ['config-list-page', viewport ? 'config-list-page--viewport' : null].filter(Boolean).join(' ')
  const listClassNames = ['config-list-page-list', listContent ? 'config-list-page-list--custom' : null, listClassName]
    .filter(Boolean)
    .join(' ')
  const hasToolbar = Boolean(filters || actions)
  const tablePagination = tableProps.pagination
  const shouldRenderFooter = Boolean(footer) || tablePagination === false || Boolean(listContent) || (viewport && !showTable)
  const tableScroll = viewport
    ? {
        ...tableProps.scroll,
        y: tableProps.scroll?.y ?? '100%',
      }
    : tableProps.scroll

  return (
    <div className={pageClassName} data-component="config-list-page">
      {header ?? (title ? <PageHeader title={title} subtitle={subtitle} /> : null)}
      {hasToolbar ? (
        <div className="config-list-page-toolbar">
          {filters ? <div className="config-list-page-filters">{filters}</div> : <div />}
          {actions ? (
            <Space className="config-list-page-actions" wrap>
              {actions}
            </Space>
          ) : null}
        </div>
      ) : null}
      {stats ? <div className="config-list-page-stats">{stats}</div> : null}
      {alerts ? <div className="config-list-page-alerts">{alerts}</div> : null}
      <div className={listClassNames}>
        {listContent ? (
          listContent
        ) : showTable && rowKey && mobileCard ? (
          <ResponsiveListTable
            {...tableProps}
            dataSource={rows}
            loading={loading}
            mobileCard={mobileCard}
            rowKey={rowKey}
            scroll={tableScroll}
          />
        ) : emptyContent ? (
          emptyContent
        ) : emptyState ? (
          <EmptyState {...emptyState} />
        ) : (
          null
        )}
      </div>
      {shouldRenderFooter ? (
        <div className="config-list-page-footer">
          {footer ?? <span className="config-list-page-footer-summary">{t('common.resultCount', { n: rows.length })}</span>}
        </div>
      ) : null}
    </div>
  )
}
