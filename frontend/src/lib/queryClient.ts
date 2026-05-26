import { QueryClient, type QueryClientConfig, type QueryKey, type UseQueryOptions } from '@tanstack/react-query'

export const LIST_STALE_TIME_MS = 30_000
export const RARELY_CHANGED_STALE_TIME_MS = 5 * 60_000
export const ONE_SHOT_STALE_TIME_MS = 0

export const listQueryOptions = {
  staleTime: LIST_STALE_TIME_MS
} as const

export const brandingQueryOptions = {
  staleTime: RARELY_CHANGED_STALE_TIME_MS
} as const

export const settingsQueryOptions = {
  staleTime: RARELY_CHANGED_STALE_TIME_MS
} as const

export type QueryArea = 'admin' | 'portal' | 'public'
export type AppQueryKey = readonly [QueryArea, string, string, ...unknown[]]

export function queryKey(area: QueryArea, resource: string, op: string, ...args: unknown[]): AppQueryKey {
  return [area, resource, op, ...args]
}

export function invalidatePrefix(area: QueryArea, resource: string): QueryKey {
  return [area, resource]
}

export function createAppQueryClient(config?: QueryClientConfig): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: LIST_STALE_TIME_MS,
        retry: 1,
        refetchOnWindowFocus: false,
        ...config?.defaultOptions?.queries
      },
      mutations: {
        retry: false,
        ...config?.defaultOptions?.mutations
      }
    },
    ...config
  })
}

export type RarelyChangedQueryOptions<TQueryFnData, TError = Error, TData = TQueryFnData> = Omit<
  UseQueryOptions<TQueryFnData, TError, TData, AppQueryKey>,
  'queryKey' | 'queryFn' | 'staleTime'
>
