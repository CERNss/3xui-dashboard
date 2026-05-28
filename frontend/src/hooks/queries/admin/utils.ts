import { useMutation } from '@tanstack/react-query'
import { utilsApi } from '@/api/admin/utils'
import { useMutationErrorHandler } from '../error'

export function useGenerateX25519() {
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: () => utilsApi.x25519(),
    onError: (error) => handleError(error),
  })
}
