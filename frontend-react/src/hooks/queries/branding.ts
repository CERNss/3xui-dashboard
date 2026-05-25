import { useQuery, type UseQueryResult } from '@tanstack/react-query'
import { brandingApi, type Branding } from '@/api/branding'
import {
  brandingQueryOptions,
  queryKey,
  type RarelyChangedQueryOptions
} from '@/lib/queryClient'
import { useQueryErrorReporter } from './error'

export const BRANDING_QUERY_KEY = queryKey('public', 'branding', 'get')

export type BrandingQueryOptions = RarelyChangedQueryOptions<Branding>

export function useBranding(options?: BrandingQueryOptions): UseQueryResult<Branding, Error> {
  const result = useQuery({
    queryKey: BRANDING_QUERY_KEY,
    queryFn: brandingApi.get,
    ...brandingQueryOptions,
    ...options
  })
  useQueryErrorReporter(result.error, result.isError)
  return result
}
