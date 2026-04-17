import { useQuery } from '@tanstack/react-query'
import { listWorkers } from '@/api/workers'

export function useWorkers() {
  return useQuery({
    queryKey: ['workers'],
    queryFn: listWorkers,
    staleTime: 8_000,
    refetchInterval: 15_000,
  })
}
