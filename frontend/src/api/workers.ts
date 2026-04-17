import { apiClient } from './client'
import type { ApiResponse, Worker, PaginatedResponse } from '@/types'

export async function listWorkers(): Promise<PaginatedResponse<Worker>> {
  const { data } = await apiClient.get<ApiResponse<Worker[]>>('/workers')
  if (data.error) throw new Error(data.error)
  return { data: data.data ?? [], meta: data.meta }
}
