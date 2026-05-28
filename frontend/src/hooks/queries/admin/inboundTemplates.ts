import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  inboundTemplatesApi,
  type InboundTemplateInput,
} from '@/api/admin/inboundTemplates'
import { useMutationErrorHandler, useQueryErrorReporter } from '../error'
import { queryKeys } from '../keys'

const keys = queryKeys('admin', 'inboundTemplates')

export function useInboundTemplatesList() {
  const result = useQuery({
    queryKey: keys.list(),
    queryFn: inboundTemplatesApi.list,
  })
  useQueryErrorReporter(result.error, result.isError)
  return result
}

export function useCreateInboundTemplate() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (input: InboundTemplateInput) => inboundTemplatesApi.create(input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.root }),
    onError: (error) => handleError(error),
  })
}

export function useUpdateInboundTemplate() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: ({ id, input }: { id: number; input: Partial<InboundTemplateInput> }) =>
      inboundTemplatesApi.update(id, input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.root }),
    onError: (error) => handleError(error),
  })
}

export function useRemoveInboundTemplate() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (id: number) => inboundTemplatesApi.remove(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.root }),
    onError: (error) => handleError(error),
  })
}
