import { apiClient } from './client'
import type {
  ApiResponse,
  Job,
  JobListParams,
  EnqueueJobRequest,
  PaginatedResponse,
} from '@/types'

export async function listJobs(params: JobListParams): Promise<PaginatedResponse<Job>> {
  const { data } = await apiClient.get<ApiResponse<Job[]>>('/jobs', { params })
  if (data.error) throw new Error(data.error)
  return { data: data.data ?? [], meta: data.meta }
}

export async function getJob(id: string): Promise<Job> {
  const { data } = await apiClient.get<ApiResponse<Job>>(`/jobs/${id}`)
  if (data.error) throw new Error(data.error)
  if (!data.data) throw new Error('Job not found')
  return data.data
}

export async function enqueueJob(payload: EnqueueJobRequest): Promise<Job> {
  const { data } = await apiClient.post<ApiResponse<Job>>('/jobs', payload)
  if (data.error) throw new Error(data.error)
  if (!data.data) throw new Error('Failed to enqueue job')
  return data.data
}

export async function enqueueBatch(jobs: EnqueueJobRequest[]): Promise<Job[]> {
  const { data } = await apiClient.post<ApiResponse<Job[]>>('/jobs/batch', jobs)
  if (data.error) throw new Error(data.error)
  return data.data ?? []
}

export async function cancelJob(id: string): Promise<void> {
  await apiClient.delete(`/jobs/${id}`)
}

export async function retryJob(id: string): Promise<Job> {
  const { data } = await apiClient.post<ApiResponse<Job>>(`/jobs/${id}/retry`)
  if (data.error) throw new Error(data.error)
  if (!data.data) throw new Error('Failed to retry job')
  return data.data
}
