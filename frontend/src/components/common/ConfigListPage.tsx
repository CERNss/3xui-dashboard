import { Pagination, Space } from 'antd'
import type { TablePaginationConfig } from 'antd/es/table'
import type { ReactNode } from 'react'
import { useEffect, useMemo, useState } from 'react'
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
  const paginationConfig = useMemo<TablePaginationConfig>(() => {
    if (tablePagination && typeof tablePagination === 'object') return tablePagination
    return {}
  }, [tablePagination])
  const [currentPage, setCurrentPage] = useState(() => Number(paginationConfig.current ?? paginationConfig.defaultCurrent ?? 1))
  const [pageSize, setPageSize] = useState(() => Number(paginationConfig.pageSize ?? paginationConfig.defaultPageSize ?? 20))
  const canPaginateTable = showTable && Boolean(rowKey && mobileCard) && !listContent
  const showFooterPagination = viewport && canPaginateTable && rows.length > 0
  const pageCount = Math.max(1, Math.ceil(rows.length / pageSize))
  const shouldRenderFooter = Boolean(footer) || Boolean(listContent) || viewport
  const tableScroll = viewport
    ? {
        ...tableProps.scroll,
        y: tableProps.scroll?.y ?? '100%',
      }
    : tableProps.scroll
  const paginationPosition: TablePaginationConfig['position'] = ['none']

  useEffect(() => {
    const nextPageSize = Number(paginationConfig.pageSize ?? paginationConfig.defaultPageSize ?? 20)
    setPageSize(nextPageSize)
  }, [paginationConfig.defaultPageSize, paginationConfig.pageSize])

  useEffect(() => {
    setCurrentPage((page) => Math.min(Math.max(1, page), pageCount))
  }, [pageCount])

  const changePage = (page: number, nextPageSize: number) => {
    setCurrentPage(page)
    setPageSize(nextPageSize)
    paginationConfig.onChange?.(page, nextPageSize)
  }

  const controlledPagination =
    canPaginateTable && viewport
      ? {
          ...paginationConfig,
          current: currentPage,
          pageSize,
          total: paginationConfig.total ?? rows.length,
          position: paginationPosition,
          onChange: changePage,
        }
      : tablePagination

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
            pagination={controlledPagination}
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
          <div className="config-list-page-footer-content">
            <div className="config-list-page-footer-main">
              {footer ?? <span className="config-list-page-footer-summary">{t('common.resultCount', { n: rows.length })}</span>}
            </div>
            {showFooterPagination ? (
              <Pagination
                className="config-list-page-footer-pagination"
                current={currentPage}
                pageSize={pageSize}
                pageSizeOptions={['10', '20', '50', '100']}
                showSizeChanger
                size="small"
                total={paginationConfig.total ?? rows.length}
                onChange={changePage}
                onShowSizeChange={changePage}
              />
            ) : null}
          </div>
        </div>
      ) : null}
    </div>
  )
}
