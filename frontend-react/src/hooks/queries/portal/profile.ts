import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { portalProfileApi, type EmailVerificationPurpose } from '@/api/portal/profile'
import { useMutationErrorHandler, useQueryErrorReporter } from '../error'
import { queryKeys } from '../keys'

const keys = queryKeys('portal', 'profile')

export function useProfile() {
  const result = useQuery({
    queryKey: keys.op('get'),
    queryFn: portalProfileApi.get,
  })
  useQueryErrorReporter(result.error, result.isError)
  return result
}

export function useLoginMethods() {
  const result = useQuery({
    queryKey: keys.op('loginMethods'),
    queryFn: portalProfileApi.loginMethods,
  })
  useQueryErrorReporter(result.error, result.isError)
  return result
}

export function useUpdateProfile() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (input: { display_name?: string | null }) => portalProfileApi.updateProfile(input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.root }),
    onError: (error) => handleError(error),
  })
}

export function useStartEmailVerification() {
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (input: { email: string; purpose: EmailVerificationPurpose }) =>
      portalProfileApi.startEmailVerification(input),
    onError: (error) => handleError(error),
  })
}

export function useChangeEmail() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (input: { email: string; verificationToken: string }) => portalProfileApi.changeEmail(input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.root }),
    onError: (error) => handleError(error),
  })
}

export function useChangePassword() {
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: ({ oldPassword, newPassword }: { oldPassword: string; newPassword: string }) =>
      portalProfileApi.changePassword(oldPassword, newPassword),
    onError: (error) => handleError(error),
  })
}

export function useBindEmail() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (email: string) => portalProfileApi.bindEmail(email),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.root }),
    onError: (error) => handleError(error),
  })
}

export function useStartOidcLink() {
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: ({ providerKey, redirectAfter }: { providerKey: string; redirectAfter?: string }) =>
      portalProfileApi.startOIDCLink(providerKey, redirectAfter),
    onError: (error) => handleError(error),
  })
}

export function useRotateSubId() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: portalProfileApi.rotateSubID,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.root }),
    onError: (error) => handleError(error),
  })
}
