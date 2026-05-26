import { useMutation, useQuery } from '@tanstack/react-query'
import {
  portalAuthApi,
  type OIDCBindExistingInput,
  type OIDCCreateAccountInput,
  type PortalLoginInput,
  type PortalRegisterInput,
} from '@/api/portal/auth'
import type { PublicEmailVerificationPurpose } from '@/api/portal/emailVerification'
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
    mutationFn: (input: PortalLoginInput) => portalAuthApi.login(input),
    onError: (error) => handleError(error),
  })
}

export function usePortalRegister() {
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (input: PortalRegisterInput) => portalAuthApi.register(input),
    onError: (error) => handleError(error),
  })
}

export function useStartAuthEmailVerification() {
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (input: { email: string; purpose: PublicEmailVerificationPurpose }) =>
      portalAuthApi.startEmailVerification(input),
    onError: (error) => handleError(error),
  })
}

export function useOidcStart() {
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: ({ redirectAfter, providerKey }: { redirectAfter?: string; providerKey?: string } = {}) =>
      portalAuthApi.oidcStart(redirectAfter, providerKey),
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

export function useOidcBindExisting() {
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (input: OIDCBindExistingInput) => portalAuthApi.oidcBindExisting(input),
    onError: (error) => handleError(error),
  })
}

export function useOidcCreateAccount() {
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (input: OIDCCreateAccountInput) => portalAuthApi.oidcCreateAccount(input),
    onError: (error) => handleError(error),
  })
}
