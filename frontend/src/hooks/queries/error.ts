import { useEffect } from 'react'
import type { HandleErrorOptions } from '@/hooks/useErrorHandler'
import { useErrorHandler } from '@/hooks/useErrorHandler'

export function useQueryErrorReporter(error: unknown, isError: boolean, options?: HandleErrorOptions) {
  const { handleError } = useErrorHandler()

  useEffect(() => {
    if (isError) {
      handleError(error, options)
    }
  }, [error, handleError, isError, options])
}

export function useMutationErrorHandler() {
  const { handleError } = useErrorHandler()
  return handleError
}
