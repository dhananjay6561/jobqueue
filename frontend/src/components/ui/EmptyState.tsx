import type { ReactNode } from 'react'
import { Button } from './Button'

interface EmptyStateProps {
  icon?: ReactNode
  title: string
  description: string
  action?: {
    label: string
    onClick: () => void
  }
}

export function EmptyState({ icon, title, description, action }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center py-16 px-8 text-center">
      {/* Icon */}
      <div className="w-16 h-16 rounded-2xl bg-[#7c6af7]/10 border border-[#7c6af7]/20 flex items-center justify-center mb-5">
        {icon ?? (
          <svg
            className="w-8 h-8 text-[#7c6af7]/60"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            aria-hidden="true"
          >
            <path strokeLinecap="round" strokeLinejoin="round" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4" />
          </svg>
        )}
      </div>

      {/* Text */}
      <h3 className="text-[#e2e2f0] font-semibold text-base mb-1.5">{title}</h3>
      <p className="text-[#6b6b8a] text-sm leading-relaxed max-w-xs mb-6">{description}</p>

      {/* Action */}
      {action && (
        <Button variant="primary" size="md" onClick={action.onClick}>
          {action.label}
        </Button>
      )}
    </div>
  )
}
