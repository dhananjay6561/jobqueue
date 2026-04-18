import { apiClient } from './client'
import type { ApiResponse, CronSchedule, CreateCronRequest } from '@/types'

export async function listCron(): Promise<CronSchedule[]> {
  const { data } = await apiClient.get<ApiResponse<CronSchedule[]>>('/cron')
  if (data.error) throw new Error(data.error)
  return data.data ?? []
}

export async function createCron(req: CreateCronRequest): Promise<CronSchedule> {
  const { data } = await apiClient.post<ApiResponse<CronSchedule>>('/cron', req)
  if (data.error) throw new Error(data.error)
  if (!data.data) throw new Error('Failed to create cron schedule')
  return data.data
}

export async function patchCron(
  id: string,
  patch: { enabled?: boolean; cron_expression?: string; payload?: Record<string, unknown> },
): Promise<CronSchedule> {
  const { data } = await apiClient.patch<ApiResponse<CronSchedule>>(`/cron/${id}`, patch)
  if (data.error) throw new Error(data.error)
  if (!data.data) throw new Error('Failed to update cron schedule')
  return data.data
}

export async function deleteCron(id: string): Promise<void> {
  await apiClient.delete(`/cron/${id}`)
}
