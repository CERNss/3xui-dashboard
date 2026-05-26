import { useQuery } from '@tanstack/react-query'
import { adminStatsApi } from '@/api/admin/stats'
import { useQueryErrorReporter } from '../error'
import { queryKeys } from '../keys'

const keys = queryKeys('admin', 'stats')

export function useAdminStats(options?: { reportErrors?: boolean }) {
  const result = useQuery({
    queryKey: keys.op('get'),
    queryFn: adminStatsApi.get,
  })
  useQueryErrorReporter(result.error, result.isError && options?.reportErrors !== false)
  return result
}
