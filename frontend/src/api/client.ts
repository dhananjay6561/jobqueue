import axios from 'axios'

const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? ''

export const apiClient = axios.create({
  baseURL: `${BASE_URL}/api/v1`,
  headers: {
    'Content-Type': 'application/json',
  },
  timeout: 15000,
})

// Request interceptor — inject JWT (preferred) or API key, plus request ID
apiClient.interceptors.request.use((config) => {
  config.headers['X-Request-ID'] = crypto.randomUUID()
  try {
    // JWT takes priority — if logged in, backend resolves key from JWT
    const authStored = localStorage.getItem('jobqueue-auth')
    const token: string = authStored ? (JSON.parse(authStored)?.state?.token ?? '') : ''
    if (token) {
      config.headers['Authorization'] = `Bearer ${token}`
      return config
    }
    // Fallback: raw API key (for users who pasted a key manually)
    const uiStored = localStorage.getItem('jobqueue-ui')
    const key: string = uiStored ? (JSON.parse(uiStored)?.state?.apiKey ?? '') : ''
    if (key) config.headers['X-API-Key'] = key
  } catch {
    // ignore parse errors
  }
  return config
})

// Response interceptor — unwrap data field
apiClient.interceptors.response.use(
  (response) => response,
  (error: unknown) => {
    if (axios.isAxiosError(error)) {
      const message =
        (error.response?.data as { error?: string } | undefined)?.error ??
        error.message ??
        'An unexpected error occurred'
      return Promise.reject(new Error(message))
    }
    return Promise.reject(error)
  },
)
