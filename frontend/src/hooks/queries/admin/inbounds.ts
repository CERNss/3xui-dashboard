import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  clientsApi,
  inboundsApi,
  trafficApi,
  type Client,
  type Inbound,
} from '@/api/admin/inbounds'
import { useMutationErrorHandler, useQueryErrorReporter } from '../error'
import { queryKeys } from '../keys'

const inboundKeys = queryKeys('admin', 'inbounds')
const clientKeys = queryKeys('admin', 'clients')
const trafficKeys = queryKeys('admin', 'traffic')

function invalidateInbounds(queryClient: ReturnType<typeof useQueryClient>) {
  queryClient.invalidateQueries({ queryKey: inboundKeys.root })
  queryClient.invalidateQueries({ queryKey: clientKeys.root })
  queryClient.invalidateQueries({ queryKey: trafficKeys.root })
}

export function useInboundsFleet() {
  const result = useQuery({
    queryKey: inboundKeys.list(),
    queryFn: inboundsApi.fleet,
  })
  useQueryErrorReporter(result.error, result.isError)
  return result
}

export function useInboundDetail(nodeID: number, tag: string, enabled = true) {
  const result = useQuery({
    queryKey: inboundKeys.detail(nodeID, tag),
    queryFn: () => inboundsApi.get(nodeID, tag),
    enabled: enabled && Number.isFinite(nodeID) && tag.length > 0,
  })
  useQueryErrorReporter(result.error, result.isError)
  return result
}

export function useClientSnapshot(nodeID: number, enabled = true) {
  const result = useQuery({
    queryKey: clientKeys.op('snapshot', nodeID),
    queryFn: () => clientsApi.snapshot(nodeID),
    enabled: enabled && Number.isFinite(nodeID),
  })
  useQueryErrorReporter(result.error, result.isError)
  return result
}

export function useCreateInbound() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: ({ nodeID, body }: { nodeID: number; body: Partial<Inbound> }) =>
      inboundsApi.create(nodeID, body),
    onSuccess: () => invalidateInbounds(queryClient),
    onError: (error) => handleError(error),
  })
}

export function useUpdateInbound() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: ({ nodeID, tag, body }: { nodeID: number; tag: string; body: Partial<Inbound> }) =>
      inboundsApi.update(nodeID, tag, body),
    onSuccess: (_data, vars) => {
      invalidateInbounds(queryClient)
      queryClient.invalidateQueries({ queryKey: inboundKeys.detail(vars.nodeID, vars.tag) })
    },
    onError: (error) => handleError(error),
  })
}

export function useSetInboundEnable() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: ({ nodeID, tag, enable }: { nodeID: number; tag: string; enable: boolean }) =>
      inboundsApi.setEnable(nodeID, tag, enable),
    onSuccess: () => invalidateInbounds(queryClient),
    onError: (error) => handleError(error),
  })
}

export function useRemoveInbound() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: ({ nodeID, tag }: { nodeID: number; tag: string }) => inboundsApi.remove(nodeID, tag),
    onSuccess: () => invalidateInbounds(queryClient),
    onError: (error) => handleError(error),
  })
}

export function useAddClient() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: ({
      nodeID,
      tag,
      client,
      userID,
    }: {
      nodeID: number
      tag: string
      client: Partial<Client>
      userID?: number
    }) => clientsApi.add(nodeID, tag, client, userID),
    onSuccess: () => invalidateInbounds(queryClient),
    onError: (error) => handleError(error),
  })
}

export function useUpdateClient() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: ({
      nodeID,
      tag,
      email,
      client,
    }: {
      nodeID: number
      tag: string
      email: string
      client: Partial<Client>
    }) => clientsApi.update(nodeID, tag, email, client),
    onSuccess: () => invalidateInbounds(queryClient),
    onError: (error) => handleError(error),
  })
}

export function useRemoveClient() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: ({ nodeID, tag, email }: { nodeID: number; tag: string; email: string }) =>
      clientsApi.remove(nodeID, tag, email),
    onSuccess: () => invalidateInbounds(queryClient),
    onError: (error) => handleError(error),
  })
}

export function useResetClientTraffic() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: ({ nodeID, tag, email }: { nodeID: number; tag: string; email: string }) =>
      trafficApi.resetClient(nodeID, tag, email),
    onSuccess: () => invalidateInbounds(queryClient),
    onError: (error) => handleError(error),
  })
}

export function useResetInboundTraffic() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: ({ nodeID, tag }: { nodeID: number; tag: string }) => trafficApi.resetInbound(nodeID, tag),
    onSuccess: () => invalidateInbounds(queryClient),
    onError: (error) => handleError(error),
  })
}

export function useResetNodeTraffic() {
  const queryClient = useQueryClient()
  const handleError = useMutationErrorHandler()
  return useMutation({
    mutationFn: (nodeID: number) => trafficApi.resetNode(nodeID),
    onSuccess: () => invalidateInbounds(queryClient),
    onError: (error) => handleError(error),
  })
}
