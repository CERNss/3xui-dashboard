import { Card, Input, Select, Space, Tag, Typography } from 'antd'
import type { ColumnsType, TableProps } from 'antd/es/table'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { AdminAction, ListAuditParams } from '@/api/admin/audit'
import { ConfigListPage, RefreshButton } from '@/components/common'
import { useAuditLog } from '@/hooks/queries/admin/audit'

type SortKey = 'created_at' | 'admin_username' | 'method' | 'status_code'

const METHODS = ['POST', 'PUT', 'DELETE', 'PATCH'] as const

function formatDate(value: string) {
  return new Date(value).toLocaleString()
}

function methodTag(method: string) {
  const colorByMethod: Record<string, string> = {
    GET: 'blue',
    POST: 'green',
    PUT: 'gold',
    DELETE: 'red',
    PATCH: 'purple',
  }

  return <Tag color={colorByMethod[method] ?? 'default'}>{method}</Tag>
}

function statusTag(code: number) {
  if (code >= 200 && code < 300) {
    return <Tag color="green">{code}</Tag>
  }
  if (code >= 400 && code < 500) {
    return <Tag color="gold">{code}</Tag>
  }
  return <Tag color="red">{code}</Tag>
}

function compareRows(a: AdminAction, b: AdminAction, key: SortKey) {
  if (key === 'status_code') {
    return a.status_code - b.status_code
  }

  return String(a[key] ?? '').localeCompare(String(b[key] ?? ''))
}

export default function AuditLog() {
  const { t } = useTranslation()
  const [username, setUsername] = useState('')
  const [resource, setResource] = useState('')
  const [method, setMethod] = useState<string | undefined>()
  const [debouncedFilters, setDebouncedFilters] = useState<ListAuditParams>({ limit: 100 })
  const mounted = useRef(false)

  useEffect(() => {
    if (!mounted.current) {
      mounted.current = true
      return undefined
    }

    const timer = window.setTimeout(() => {
      setDebouncedFilters({
        limit: 100,
        username: username.trim() || undefined,
        resource: resource.trim() || undefined,
        method,
      })
    }, 300)

    return () => window.clearTimeout(timer)
  }, [method, resource, username])

  const auditQuery = useAuditLog(debouncedFilters)
  const rows = auditQuery.data?.actions ?? []
  const hasFilters = Boolean(debouncedFilters.username || debouncedFilters.resource || debouncedFilters.method)

  const columns: ColumnsType<AdminAction> = useMemo(
    () => [
      {
        title: t('admin.auditLog.column.time'),
        dataIndex: 'created_at',
        align: 'center',
        className: 'table-cell-nowrap',
        width: 180,
        sorter: (a, b) => compareRows(a, b, 'created_at'),
        defaultSortOrder: 'descend',
        render: formatDate,
      },
      {
        title: t('admin.auditLog.column.admin'),
        dataIndex: 'admin_username',
        sorter: (a, b) => compareRows(a, b, 'admin_username'),
        render: (value: string) => value || t('admin.auditLog.unknownAdmin'),
      },
      {
        title: t('admin.auditLog.column.method'),
        dataIndex: 'method',
        align: 'center',
        width: 110,
        sorter: (a, b) => compareRows(a, b, 'method'),
        render: methodTag,
      },
      {
        title: t('admin.auditLog.column.path'),
        dataIndex: 'path',
        render: (path: string, row) => (
          <Typography.Text code>
            {path}
            {row.query_string ? `?${row.query_string}` : ''}
          </Typography.Text>
        ),
      },
      {
        title: t('admin.auditLog.column.target'),
        key: 'target',
        render: (_, row) =>
          row.target_resource ? (
            <Typography.Text code>
              {row.target_resource}
              {row.target_id ? ` #${row.target_id}` : ''}
            </Typography.Text>
          ) : (
            <Typography.Text type="secondary">-</Typography.Text>
          ),
      },
      {
        title: t('admin.auditLog.column.status'),
        dataIndex: 'status_code',
        align: 'center',
        width: 130,
        sorter: (a, b) => compareRows(a, b, 'status_code'),
        render: (code: number, row) => (
          <Space direction="vertical" size={2}>
            {statusTag(code)}
            {row.error_msg ? <Typography.Text type="danger">{row.error_msg}</Typography.Text> : null}
          </Space>
        ),
      },
      {
        title: t('admin.auditLog.column.ip'),
        dataIndex: 'ip',
        align: 'center',
        className: 'table-cell-nowrap',
        width: 140,
        render: (ip: string) => ip || '-',
      },
    ],
    [t],
  )

  const onTableChange: TableProps<AdminAction>['onChange'] = () => undefined

  return (
    <section className="admin-audit-log-page">
      <ConfigListPage
        title={t('admin.auditLog.title')}
        subtitle={t('admin.auditLog.subtitle')}
        actions={<RefreshButton loading={auditQuery.isFetching} onClick={() => auditQuery.refetch()} />}
        filters={
          <Space wrap>
            <Input
              allowClear
              aria-label={t('admin.auditLog.filterUsername')}
              placeholder={t('admin.auditLog.filterUsername')}
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              style={{ width: 240 }}
            />
            <Input
              allowClear
              aria-label={t('admin.auditLog.filterResource')}
              placeholder={t('admin.auditLog.filterResource')}
              value={resource}
              onChange={(event) => setResource(event.target.value)}
              style={{ width: 240 }}
            />
            <Select
              allowClear
              aria-label={t('admin.auditLog.filterMethod')}
              placeholder={t('admin.auditLog.anyMethod')}
              value={method}
              onChange={setMethod}
              style={{ width: 160 }}
              options={METHODS.map((value) => ({ label: value, value }))}
            />
          </Space>
        }
        rowKey="id"
        columns={columns}
        dataSource={rows}
        loading={auditQuery.isLoading}
        pagination={{ pageSize: 20, showSizeChanger: false }}
        onChange={onTableChange}
        emptyState={{
          description: hasFilters ? t('admin.auditLog.emptyFiltered') : t('admin.auditLog.emptyTotal'),
        }}
        mobileCard={(row) => (
          <Card size="small" style={{ width: '100%' }}>
            <Space direction="vertical" size={4}>
              <Typography.Text type="secondary">{formatDate(row.created_at)}</Typography.Text>
              <Typography.Text strong>{row.admin_username || t('admin.auditLog.unknownAdmin')}</Typography.Text>
              <Space wrap>
                {methodTag(row.method)}
                {statusTag(row.status_code)}
              </Space>
              <Typography.Text code>
                {row.path}
                {row.query_string ? `?${row.query_string}` : ''}
              </Typography.Text>
              <Typography.Text>{row.target_resource ? `${row.target_resource} #${row.target_id}` : '-'}</Typography.Text>
              <Typography.Text type="secondary">{row.ip || '-'}</Typography.Text>
            </Space>
          </Card>
        )}
      />
    </section>
  )
}
