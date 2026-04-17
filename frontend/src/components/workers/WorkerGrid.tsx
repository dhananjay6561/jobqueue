import { useWorkers } from '@/hooks/useWorkers'
import { WorkerCard } from './WorkerCard'
import { SkeletonCard } from '@/components/ui/Skeleton'
import { EmptyState } from '@/components/ui/EmptyState'

export function WorkerGrid() {
  const { data, isLoading, error } = useWorkers()

  if (error) {
    return (
      <EmptyState
        title="Failed to load workers"
        description={(error as Error).message}
      />
    )
  }

  if (isLoading) {
    return (
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
        {Array.from({ length: 5 }).map((_, i) => (
          <SkeletonCard key={i} />
        ))}
      </div>
    )
  }

  if (!data?.data.length) {
    return (
      <EmptyState
        title="No workers registered"
        description="Workers will appear here once the job queue server starts worker goroutines."
      />
    )
  }

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
      {data.data.map((worker) => (
        <WorkerCard key={worker.id} worker={worker} />
      ))}
    </div>
  )
}
