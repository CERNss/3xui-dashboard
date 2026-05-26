export type QueryArea = 'admin' | 'portal' | 'public'

export function queryKeys(area: QueryArea, resource: string) {
  const root = [area, resource] as const
  return {
    root,
    list: (params?: unknown) =>
      params === undefined ? ([...root, 'list'] as const) : ([...root, 'list', params] as const),
    detail: (...args: readonly unknown[]) => [...root, 'detail', ...args] as const,
    op: (op: string, ...args: readonly unknown[]) => [...root, op, ...args] as const,
  }
}
