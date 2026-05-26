import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { portalBillingApi, type PaymentMethod, type PurchaseInput } from '@/api/portal/billing'
import { useMutationErrorHandler, useQueryErrorReporter } from '../error'
import { queryKeys } from '../keys'

const planKeys = queryKeys('portal', 'plans')
const orderKeys = queryKeys('portal', 'orders')
const billingKeys = queryKeys('portal', 'billing')

export function usePortalPlansList() {
  const result = useQuery({
    queryKey: planKeys.list(),
    queryFn: portalBillingApi.listPlans,
  })
  useQueryErrorReporter(result.error, result.isError)
  return result
}

export function usePortalOrdersList() {
  const result = useQuery({
    queryKey: orderKeys.list(),
    queryFn: portalBillingApi.listOrders,
  })
  useQueryErrorReporter(result.error, result.isError)
  return result
}

export function usePaymentMethods() {
  const result = useQuery({
    queryKey: billingKeys.op('paymentMethods'),
    queryFn: portalBillingApi.paymentMethods,
  })
  useQueryErrorReporter(result.error, result.isError)
  return result
}

export function usePortalOrder(id: number, enabled = true) {
  const result = useQuery({
    queryKey: orderKeys.detail(id),
    queryFn: () => portalBillingApi.getOrder(id),
    enabled: enabled && Number.isFinite(id),
  })
  useQueryErrorReporter(result.error, result.isError)
  return result
}

export function usePurchasePlan() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (input: PurchaseInput) => portalBillingApi.purchase(input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: orderKeys.root }),
    onError: (error) => handleError(error),
  })
}

export function usePurchaseViaPayment() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: ({ provider, input }: { provider: PaymentMethod; input: PurchaseInput }) =>
      portalBillingApi.purchaseViaPayment(provider, input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: orderKeys.root }),
    onError: (error) => handleError(error),
  })
}
