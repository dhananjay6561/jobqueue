import axios from 'axios'
import { useAuthStore } from '@/store/authStore'

const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080'

export const portalClient = axios.create({
  baseURL: BASE_URL,
  headers: { 'Content-Type': 'application/json' },
  timeout: 15000,
})

portalClient.interceptors.request.use((config) => {
  const token = useAuthStore.getState().token
  if (token) config.headers['Authorization'] = `Bearer ${token}`
  return config
})

export async function createCheckout(tier: 'pro' | 'business', keyId?: string): Promise<string> {
  const { data } = await portalClient.post<{ data: { url: string } }>('/portal/checkout', {
    tier,
    key_id: keyId ?? '',
  })
  return data.data.url
}

export async function createCustomerPortal(): Promise<string> {
  const { data } = await portalClient.post<{ data: { url: string } }>('/portal/customer-portal')
  return data.data.url
}

export interface RegeneratedKey {
  key: string
  key_prefix: string
  tier: string
  jobs_limit: number
  warning: string
}

export async function regenerateKey(): Promise<RegeneratedKey> {
  const { data } = await portalClient.post<{ data: RegeneratedKey }>('/portal/regenerate-key')
  return data.data
}
