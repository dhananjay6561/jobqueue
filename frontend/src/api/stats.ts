import { apiClient } from './client'
import type {
  ApiResponse,
  QueueStats,
  DLQEntry,
  PaginatedResponse,
} from '@/types'

export async function getStats(): Promise<QueueStats> {
  const { data } = await apiClient.get<ApiResponse<QueueStats>>('/stats')
  if (data.error) throw new Error(data.error)
  if (!data.data) throw new Error('No stats available')
  return data.data
}

export async function listDLQ(params: {
  limit?: number
  offset?: number
}): Promise<PaginatedResponse<DLQEntry>> {
  const { data } = await apiClient.get<ApiResponse<DLQEntry[]>>('/dlq', { params })
  if (data.error) throw new Error(data.error)
  return { data: data.data ?? [], meta: data.meta }
}

export async function requeueDLQEntry(id: string): Promise<void> {
  await apiClient.post(`/dlq/${id}/requeue`)
}
