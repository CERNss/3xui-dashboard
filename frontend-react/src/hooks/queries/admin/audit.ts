import { useQuery } from '@tanstack/react-query'
import { adminAuditApi, type ListAuditParams } from '@/api/admin/audit'
import { useQueryErrorReporter } from '../error'
import { queryKeys } from '../keys'

const keys = queryKeys('admin', 'audit')

export function useAuditLog(params?: ListAuditParams) {
  const result = useQuery({
    queryKey: keys.list(params),
    queryFn: () => adminAuditApi.list(params),
  })
  useQueryErrorReporter(result.error, result.isError)
  return result
}
