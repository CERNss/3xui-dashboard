import { useQuery } from '@tanstack/react-query'
import { portalTrafficApi } from '@/api/portal/traffic'
import { useQueryErrorReporter } from '../error'
import { queryKeys } from '../keys'

const keys = queryKeys('portal', 'traffic')

export function useOwnTraffic() {
  const result = useQuery({
    queryKey: keys.op('own'),
    queryFn: portalTrafficApi.own,
  })
  useQueryErrorReporter(result.error, result.isError)
  return result
}
