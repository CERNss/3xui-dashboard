import { useMutation } from '@tanstack/react-query'
import { adminAuthApi } from '@/api/admin/auth'
import { useMutationErrorHandler } from '../error'

export function useAdminLogin() {
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: ({ username, password }: { username: string; password: string }) =>
      adminAuthApi.login(username, password),
    onError: (error) => handleError(error),
  })
}
