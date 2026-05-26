import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { settingsApi } from '@/api/admin/settings'
import { BRANDING_QUERY_KEY } from '@/hooks/queries/branding'
import { settingsQueryOptions } from '@/lib/queryClient'
import { useMutationErrorHandler, useQueryErrorReporter } from '../error'
import { queryKeys } from '../keys'

const keys = queryKeys('admin', 'settings')

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

export function useSmtpTest() {
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (to: string) => settingsApi.smtpTest(to),
    onError: (error) => handleError(error),
  })
}
