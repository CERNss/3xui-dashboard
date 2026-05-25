import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { adminWebhooksApi, type WebhookInput } from '@/api/admin/webhooks'
import { useMutationErrorHandler, useQueryErrorReporter } from '../error'
import { queryKeys } from '../keys'

const keys = queryKeys('admin', 'webhooks')

export function useWebhooksList() {
  const result = useQuery({
    queryKey: keys.list(),
    queryFn: adminWebhooksApi.list,
  })
  useQueryErrorReporter(result.error, result.isError)
  return result
}

export function useWebhookDetail(id: number, enabled = true) {
  const result = useQuery({
    queryKey: keys.detail(id),
    queryFn: () => adminWebhooksApi.get(id),
    enabled: enabled && Number.isFinite(id),
  })
  useQueryErrorReporter(result.error, result.isError)
  return result
}

export function useWebhookDeliveries(id: number, enabled = true) {
  const result = useQuery({
    queryKey: keys.op('deliveries', id),
    queryFn: () => adminWebhooksApi.deliveries(id),
    enabled: enabled && Number.isFinite(id),
  })
  useQueryErrorReporter(result.error, result.isError)
  return result
}

export function useCreateWebhook() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (input: WebhookInput) => adminWebhooksApi.create(input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.root }),
    onError: (error) => handleError(error),
  })
}

export function useUpdateWebhook() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: ({ id, patch }: { id: number; patch: Partial<WebhookInput> }) =>
      adminWebhooksApi.update(id, patch),
    onSuccess: (_data, vars) => {
      queryClient.invalidateQueries({ queryKey: keys.root })
      queryClient.invalidateQueries({ queryKey: keys.detail(vars.id) })
    },
    onError: (error) => handleError(error),
  })
}

export function useRemoveWebhook() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (id: number) => adminWebhooksApi.remove(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.root }),
    onError: (error) => handleError(error),
  })
}

export function useTestWebhook() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (id: number) => adminWebhooksApi.test(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.root }),
    onError: (error) => handleError(error),
  })
}

export function useReplayWebhookDelivery() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (deliveryID: number) => adminWebhooksApi.replay(deliveryID),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.root }),
    onError: (error) => handleError(error),
  })
}
