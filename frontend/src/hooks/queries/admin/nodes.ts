import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { nodesApi, type NodeInput } from '@/api/admin/nodes'
import { useMutationErrorHandler, useQueryErrorReporter } from '../error'
import { queryKeys } from '../keys'

const keys = queryKeys('admin', 'nodes')

export function useNodesList(params?: Parameters<typeof nodesApi.list>[0]) {
  const result = useQuery({
    queryKey: keys.list(params),
    queryFn: () => nodesApi.list(params),
  })
  useQueryErrorReporter(result.error, result.isError)
  return result
}

export function useNodeDetail(id: number, enabled = true) {
  const result = useQuery({
    queryKey: keys.detail(id),
    queryFn: () => nodesApi.get(id),
    enabled: enabled && Number.isFinite(id),
  })
  useQueryErrorReporter(result.error, result.isError)
  return result
}

export function useNodeMetrics(id: number, params?: Parameters<typeof nodesApi.metrics>[1], enabled = true) {
  const result = useQuery({
    queryKey: keys.op('metrics', id, params),
    queryFn: () => nodesApi.metrics(id, params),
    enabled: enabled && Number.isFinite(id),
  })
  useQueryErrorReporter(result.error, result.isError)
  return result
}

export function useCreateNode() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (body: NodeInput) => nodesApi.create(body),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.root }),
    onError: (error) => handleError(error),
  })
}

export function useUpdateNode() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: ({ id, body }: { id: number; body: NodeInput }) => nodesApi.update(id, body),
    onSuccess: (_data, vars) => {
      queryClient.invalidateQueries({ queryKey: keys.root })
      queryClient.invalidateQueries({ queryKey: keys.detail(vars.id) })
    },
    onError: (error) => handleError(error),
  })
}

export function useRemoveNode() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (id: number) => nodesApi.remove(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.root }),
    onError: (error) => handleError(error),
  })
}

export function useEnableNode() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (id: number) => nodesApi.enable(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.root }),
    onError: (error) => handleError(error),
  })
}

export function useDisableNode() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (id: number) => nodesApi.disable(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.root }),
    onError: (error) => handleError(error),
  })
}

export function useProbeNode() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (id: number) => nodesApi.probe(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.root }),
    onError: (error) => handleError(error),
  })
}
