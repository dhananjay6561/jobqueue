import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import * as jobsApi from '@/api/jobs'
import type { JobListParams, EnqueueJobRequest } from '@/types'
import { useUiStore } from '@/store/uiStore'

export function useJobs(params: JobListParams) {
  return useQuery({
    queryKey: ['jobs', params],
    queryFn: () => jobsApi.listJobs(params),
    staleTime: 10_000,
  })
}

export function useJob(id: string) {
  return useQuery({
    queryKey: ['jobs', id],
    queryFn: () => jobsApi.getJob(id),
    staleTime: 5_000,
    enabled: !!id,
  })
}

export function useEnqueueJob() {
  const queryClient = useQueryClient()
  const { addToast } = useUiStore()
  return useMutation({
    mutationFn: (payload: EnqueueJobRequest) => jobsApi.enqueueJob(payload),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['jobs'] })
      addToast({ message: 'Job enqueued successfully', variant: 'success', duration: 3000 })
    },
    onError: (err: Error) => {
      addToast({ message: `Failed to enqueue: ${err.message}`, variant: 'error' })
    },
  })
}

export function useCancelJob() {
  const queryClient = useQueryClient()
  const { addToast } = useUiStore()
  return useMutation({
    mutationFn: (id: string) => jobsApi.cancelJob(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['jobs'] })
      addToast({ message: 'Job cancelled', variant: 'success', duration: 3000 })
    },
    onError: (err: Error) => {
      addToast({ message: `Failed to cancel: ${err.message}`, variant: 'error' })
    },
  })
}

export function useRetryJob() {
  const queryClient = useQueryClient()
  const { addToast } = useUiStore()
  return useMutation({
    mutationFn: (id: string) => jobsApi.retryJob(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['jobs'] })
      addToast({ message: 'Job retried successfully', variant: 'success', duration: 3000 })
    },
    onError: (err: Error) => {
      addToast({ message: `Failed to retry: ${err.message}`, variant: 'error' })
    },
  })
}
