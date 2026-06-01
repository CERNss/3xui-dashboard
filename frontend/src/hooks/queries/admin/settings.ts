import { useEffect, useMemo } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { settingsApi, type SettingItem } from '@/api/admin/settings'
import { BRANDING_QUERY_KEY } from '@/hooks/queries/branding'
import { settingsQueryOptions } from '@/lib/queryClient'
import { useMutationErrorHandler, useQueryErrorReporter } from '../error'
import { queryKeys } from '../keys'

const keys = queryKeys('admin', 'settings')
export const DASHBOARD_AUTO_REFRESH_INTERVAL_KEY = 'dashboard_auto_refresh_interval_seconds'

export function dashboardAutoRefreshIntervalMs(settings?: SettingItem[]) {
  const raw = settings?.find((item) => item.key === DASHBOARD_AUTO_REFRESH_INTERVAL_KEY)?.value ?? '0'
  const seconds = Number(raw)
  if (!Number.isFinite(seconds) || seconds < 5) return false
  return Math.min(seconds, 3600) * 1000
}

export function useSettingsList() {
  const result = useQuery({
    queryKey: keys.list(),
    queryFn: settingsApi.list,
    ...settingsQueryOptions,
  })
  useQueryErrorReporter(result.error, result.isError)
  return result
}

export function useSetSetting() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: ({ key, value }: { key: string; value: string }) => settingsApi.set(key, value),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: keys.root })
      void queryClient.invalidateQueries({ queryKey: BRANDING_QUERY_KEY })
    },
    onError: (error) => handleError(error),
  })
}

export function useClearSetting() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (key: string) => settingsApi.clear(key),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: keys.root })
      void queryClient.invalidateQueries({ queryKey: BRANDING_QUERY_KEY })
    },
    onError: (error) => handleError(error),
  })
}

export function useUploadBrandIcon() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (file: File) => settingsApi.uploadBrandIcon(file),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: keys.root })
      void queryClient.invalidateQueries({ queryKey: BRANDING_QUERY_KEY })
    },
    onError: (error) => handleError(error),
  })
}

export function useDashboardAutoRefresh() {
  const queryClient = useQueryClient()
  const settingsQuery = useSettingsList()
  const intervalMs = useMemo(() => dashboardAutoRefreshIntervalMs(settingsQuery.data), [settingsQuery.data])

  useEffect(() => {
    if (!intervalMs) return undefined
    const timer = window.setInterval(() => {
      void queryClient.refetchQueries({
        type: 'active',
        predicate: (query) =>
          Array.isArray(query.queryKey) &&
          query.queryKey[0] === 'admin' &&
          query.queryKey[1] !== 'settings',
      })
    }, intervalMs)
    return () => window.clearInterval(timer)
  }, [intervalMs, queryClient])
}

export function useSmtpTest() {
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (to: string) => settingsApi.smtpTest(to),
    onError: (error) => handleError(error),
  })
}
