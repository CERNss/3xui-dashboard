// API response envelope used by every JSON endpoint on the backend.
export interface ApiEnvelope<T> {
  success: boolean
  msg?: string
  obj?: T
}

// Normalized error surfaced by the Axios response interceptors.
export interface ApiError {
  status: number
  message: string
  // Free-form payload from the server, if any.
  data?: unknown
}
