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
  const { data } = await authClient.post<{ data: RegisterResponse }>('/auth/register', { email, password })
  return data.data
}

export async function login(email: string, password: string): Promise<LoginResponse> {
  const { data } = await authClient.post<{ data: LoginResponse }>('/auth/login', { email, password })
  return data.data
}

export async function forgotPassword(email: string): Promise<string> {
  const { data } = await authClient.post<{ data: { message: string } }>('/auth/forgot-password', { email })
  return data.data.message
}

export async function resetPassword(token: string, password: string): Promise<string> {
  const { data } = await authClient.post<{ data: { message: string } }>('/auth/reset-password', { token, password })
  return data.data.message
}
