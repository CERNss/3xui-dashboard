import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { adminOrdersApi, type ListOrdersParams } from '@/api/admin/orders'
import { useMutationErrorHandler, useQueryErrorReporter } from '../error'
import { queryKeys } from '../keys'

const keys = queryKeys('admin', 'orders')

export function useOrdersList(params?: ListOrdersParams) {
  const result = useQuery({
    queryKey: keys.list(params),
    queryFn: () => adminOrdersApi.list(params),
  })
  useQueryErrorReporter(result.error, result.isError)
  return result
}

export function useRefundOrder() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: ({ id, reason }: { id: number; reason: string }) => adminOrdersApi.refund(id, reason),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.root }),
    onError: (error) => handleError(error),
  })
}
