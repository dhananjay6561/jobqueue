import axios from 'axios'

const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080'

const authClient = axios.create({
  baseURL: BASE_URL,
  headers: { 'Content-Type': 'application/json' },
  timeout: 15000,
})

export interface RegisterResponse {
  token: string
  user: { id: string; email: string }
  api_key: {
    id: string
    key: string
    key_prefix: string
    tier: string
    jobs_limit: number
    warning: string
  }
}

export interface LoginResponse {
  token: string
  user: { id: string; email: string; stripe_customer_id?: string }
  keys: Array<{ id: string; key_prefix: string; tier: string; jobs_used: number; jobs_limit: number }>
}

export async function register(email: string, password: string): Promise<RegisterResponse> {
  const { data } = await authClient.post<RegisterResponse>('/auth/register', { email, password })
  return data
}

export async function login(email: string, password: string): Promise<LoginResponse> {
  const { data } = await authClient.post<LoginResponse>('/auth/login', { email, password })
  return data
}
