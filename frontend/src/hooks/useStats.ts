import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getStats, listDLQ, requeueDLQEntry } from '@/api/stats'
import { useUiStore } from '@/store/uiStore'

export function useStats() {
  return useQuery({
    queryKey: ['stats'],
    queryFn: getStats,
    staleTime: 5_000,
    refetchInterval: 15_000,
  })
}

export function useDLQ(params: { limit?: number; offset?: number } = {}) {
  return useQuery({
    queryKey: ['dlq', params],
    queryFn: () => listDLQ(params),
    staleTime: 10_000,
  })
}

export function useRequeueDLQ() {
  const queryClient = useQueryClient()
  const { addToast } = useUiStore()
  return useMutation({
    mutationFn: (id: string) => requeueDLQEntry(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['dlq'] })
      void queryClient.invalidateQueries({ queryKey: ['jobs'] })
      addToast({ message: 'Job requeued from DLQ', variant: 'success', duration: 3000 })
    },
    onError: (err: Error) => {
      addToast({ message: `Requeue failed: ${err.message}`, variant: 'error' })
    },
  })
}
