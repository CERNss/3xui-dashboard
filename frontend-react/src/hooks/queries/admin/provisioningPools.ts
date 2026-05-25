import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  provisioningPoolsApi,
  type ProvisioningPoolInput,
  type ProvisioningPoolTargetInput,
} from '@/api/admin/provisioningPools'
import { useMutationErrorHandler, useQueryErrorReporter } from '../error'
import { queryKeys } from '../keys'

const keys = queryKeys('admin', 'provisioningPools')

export function useProvisioningPoolsList() {
  const result = useQuery({
    queryKey: keys.list(),
    queryFn: provisioningPoolsApi.list,
  })
  useQueryErrorReporter(result.error, result.isError)
  return result
}

export function useCreateProvisioningPool() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (input: ProvisioningPoolInput) => provisioningPoolsApi.create(input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.root }),
    onError: (error) => handleError(error),
  })
}

export function useUpdateProvisioningPool() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: ({ id, input }: { id: number; input: Partial<ProvisioningPoolInput> }) =>
      provisioningPoolsApi.update(id, input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.root }),
    onError: (error) => handleError(error),
  })
}

export function useRemoveProvisioningPool() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (id: number) => provisioningPoolsApi.remove(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.root }),
    onError: (error) => handleError(error),
  })
}

export function useAddProvisioningPoolTarget() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: ({ poolID, input }: { poolID: number; input: ProvisioningPoolTargetInput }) =>
      provisioningPoolsApi.addTarget(poolID, input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.root }),
    onError: (error) => handleError(error),
  })
}

export function useUpdateProvisioningPoolTarget() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: ({ targetID, input }: { targetID: number; input: Partial<ProvisioningPoolTargetInput> }) =>
      provisioningPoolsApi.updateTarget(targetID, input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.root }),
    onError: (error) => handleError(error),
  })
}

export function useRemoveProvisioningPoolTarget() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (targetID: number) => provisioningPoolsApi.removeTarget(targetID),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.root }),
    onError: (error) => handleError(error),
  })
}
