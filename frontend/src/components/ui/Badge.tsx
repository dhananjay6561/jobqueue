import type { ReactNode } from 'react'
import { cva, type VariantProps } from 'class-variance-authority'

// ─── Badge Variants ────────────────────────────────────────────────────────────

const badgeVariants = cva(
  'inline-flex items-center gap-1.5 px-2 py-0.5 rounded-md text-xs font-medium border font-mono',
  {
    variants: {
      variant: {
        default: 'text-[#e2e2f0] bg-white/5 border-white/10',
        success: 'text-green-400 bg-green-400/10 border-green-400/20',
        warning: 'text-amber-400 bg-amber-400/10 border-amber-400/20',
        error: 'text-red-400 bg-red-400/10 border-red-400/20',
        info: 'text-blue-400 bg-blue-400/10 border-blue-400/20',
        accent: 'text-[#7c6af7] bg-[#7c6af7]/10 border-[#7c6af7]/20',
        muted: 'text-[#6b6b8a] bg-white/5 border-white/5',
        critical: 'text-red-300 bg-red-500/15 border-red-500/25',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  },
)

interface BadgeProps extends VariantProps<typeof badgeVariants> {
  children: ReactNode
  dot?: boolean
  pulse?: boolean
  className?: string
}

const dotColors: Record<string, string> = {
  success: 'bg-green-400',
  warning: 'bg-amber-400',
  error: 'bg-red-400',
  info: 'bg-blue-400',
  accent: 'bg-[#7c6af7]',
  muted: 'bg-[#6b6b8a]',
  critical: 'bg-red-500',
  default: 'bg-[#e2e2f0]',
}

export function Badge({ children, variant = 'default', dot, pulse, className }: BadgeProps) {
  const dotColor = dotColors[variant ?? 'default'] ?? dotColors['default']
  return (
    <span className={badgeVariants({ variant, className })}>
      {dot && (
        <span className="relative flex items-center justify-center w-1.5 h-1.5">
          {pulse && (
            <span
              className={`absolute inline-flex w-full h-full rounded-full opacity-75 animate-ping ${dotColor}`}
            />
          )}
          <span className={`relative inline-flex w-1.5 h-1.5 rounded-full ${dotColor}`} />
        </span>
      )}
      {children}
    </span>
  )
}
