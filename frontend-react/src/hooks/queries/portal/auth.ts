import { useMutation, useQuery } from '@tanstack/react-query'
import { portalAuthApi, type OIDCResolveAction } from '@/api/portal/auth'
import { useMutationErrorHandler, useQueryErrorReporter } from '../error'
import { queryKeys } from '../keys'

const keys = queryKeys('portal', 'auth')

export function useRegistrationPolicy() {
  const result = useQuery({
    queryKey: keys.op('registrationPolicy'),
    queryFn: portalAuthApi.registrationPolicy,
  })
  useQueryErrorReporter(result.error, result.isError)
  return result
}

export function useOidcProviders() {
  const result = useQuery({
    queryKey: keys.op('oidcProviders'),
    queryFn: portalAuthApi.oidcProviders,
  })
  useQueryErrorReporter(result.error, result.isError)
  return result
}

export function usePortalLogin() {
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: ({ email, password }: { email: string; password: string }) =>
      portalAuthApi.login(email, password),
    onError: (error) => handleError(error),
  })
}

export function usePortalRegister() {
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: ({ email, password, code }: { email: string; password: string; code?: string }) =>
      portalAuthApi.register(email, password, code),
    onError: (error) => handleError(error),
  })
}

export function useSendCode() {
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (email: string) => portalAuthApi.sendCode(email),
    onError: (error) => handleError(error),
  })
}

export function useOidcStart() {
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (redirectAfter?: string) => portalAuthApi.oidcStart(redirectAfter),
    onError: (error) => handleError(error),
  })
}

export function useOidcCallback() {
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: ({ code, state }: { code: string; state: string }) => portalAuthApi.oidcCallback(code, state),
    onError: (error) => handleError(error),
  })
}

export function useOidcResolve() {
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: ({ pendingToken, action }: { pendingToken: string; action: OIDCResolveAction }) =>
      portalAuthApi.oidcResolve(pendingToken, action),
    onError: (error) => handleError(error),
  })
}
