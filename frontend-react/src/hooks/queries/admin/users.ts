import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { adminUsersApi, type AdminUser } from '@/api/admin/users'
import { useMutationErrorHandler, useQueryErrorReporter } from '../error'
import { queryKeys } from '../keys'

const keys = queryKeys('admin', 'users')

type CreateUserInput = Parameters<typeof adminUsersApi.create>[0]
type UpdateUserInput = Partial<Pick<AdminUser, 'email' | 'email_verified' | 'status' | 'auto_renew' | 'balance_cents'>> & {
  password?: string
}

export function useUsersList(params?: Parameters<typeof adminUsersApi.list>[0]) {
  const result = useQuery({
    queryKey: keys.list(params),
    queryFn: () => adminUsersApi.list(params),
  })
  useQueryErrorReporter(result.error, result.isError)
  return result
}

export function useUserDetail(id: number, enabled = true) {
  const result = useQuery({
    queryKey: keys.detail(id),
    queryFn: () => adminUsersApi.get(id),
    enabled: enabled && Number.isFinite(id),
  })
  useQueryErrorReporter(result.error, result.isError)
  return result
}

export function useCreateUser() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (input: CreateUserInput) => adminUsersApi.create(input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.root }),
    onError: (error) => handleError(error),
  })
}

export function useUpdateUser() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: ({ id, fields }: { id: number; fields: UpdateUserInput }) => adminUsersApi.update(id, fields),
    onSuccess: (_data, vars) => {
      queryClient.invalidateQueries({ queryKey: keys.root })
      queryClient.invalidateQueries({ queryKey: keys.detail(vars.id) })
    },
    onError: (error) => handleError(error),
  })
}

export function useSuspendUser() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (id: number) => adminUsersApi.suspend(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.root }),
    onError: (error) => handleError(error),
  })
}

export function useUnsuspendUser() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (id: number) => adminUsersApi.unsuspend(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.root }),
    onError: (error) => handleError(error),
  })
}

export function useAdjustUserBalance() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: ({ id, deltaCents, reason }: { id: number; deltaCents: number; reason: string }) =>
      adminUsersApi.adjustBalance(id, deltaCents, reason),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.root }),
    onError: (error) => handleError(error),
  })
}

export function useRemoveUser() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (id: number) => adminUsersApi.remove(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.root }),
    onError: (error) => handleError(error),
  })
}
