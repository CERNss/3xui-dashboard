import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { adminPlansApi, type CreatePlanInput, type UpdatePlanInput } from '@/api/admin/plans'
import { useMutationErrorHandler, useQueryErrorReporter } from '../error'
import { queryKeys } from '../keys'

const keys = queryKeys('admin', 'plans')

export function usePlansList() {
  const result = useQuery({
    queryKey: keys.list(),
    queryFn: adminPlansApi.list,
  })
  useQueryErrorReporter(result.error, result.isError)
  return result
}

export function useCreatePlan() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (input: CreatePlanInput) => adminPlansApi.create(input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.root }),
    onError: (error) => handleError(error),
  })
}

export function useUpdatePlan() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: ({ id, input }: { id: number; input: UpdatePlanInput }) => adminPlansApi.update(id, input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.root }),
    onError: (error) => handleError(error),
  })
}

export function useRemovePlan() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (id: number) => adminPlansApi.remove(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.root }),
    onError: (error) => handleError(error),
  })
}
