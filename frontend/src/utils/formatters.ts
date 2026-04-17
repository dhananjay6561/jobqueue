import { formatDistanceToNow, format, differenceInMilliseconds } from 'date-fns'

/**
 * Returns a relative time string like "3s ago" or "5 mins ago"
 */
export function formatRelativeTime(dateStr: string): string {
  try {
    return formatDistanceToNow(new Date(dateStr), { addSuffix: true })
  } catch {
    return 'unknown'
  }
}

/**
 * Format an ISO string to a human-readable datetime
 */
export function formatDateTime(dateStr: string): string {
  try {
    return format(new Date(dateStr), 'MMM d, yyyy HH:mm:ss')
  } catch {
    return 'invalid date'
  }
}

/**
 * Calculate and format duration between two ISO timestamps
 */
export function formatDuration(startStr: string, endStr: string | null): string {
  if (!endStr) return '—'
  try {
    const ms = differenceInMilliseconds(new Date(endStr), new Date(startStr))
    if (ms < 1000) return `${ms}ms`
    if (ms < 60_000) return `${(ms / 1000).toFixed(1)}s`
    if (ms < 3_600_000) return `${Math.floor(ms / 60_000)}m ${Math.floor((ms % 60_000) / 1000)}s`
    return `${Math.floor(ms / 3_600_000)}h ${Math.floor((ms % 3_600_000) / 60_000)}m`
  } catch {
    return '—'
  }
}

/**
 * Format a number with thousands separator
 */
export function formatNumber(n: number): string {
  return new Intl.NumberFormat().format(n)
}

/**
 * Format a percentage (0–1 → "42.1%")
 */
export function formatPercent(n: number): string {
  return `${(n * 100).toFixed(1)}%`
}

/**
 * Truncate a UUID to the first 8 chars + "…"
 */
export function truncateId(id: string): string {
  return id.slice(0, 8) + '…'
}

/**
 * Pretty-print a JSON payload
 */
export function formatPayload(payload: Record<string, unknown>): string {
  try {
    return JSON.stringify(payload, null, 2)
  } catch {
    return '{}'
  }
}
