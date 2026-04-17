import { useEffect } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { useUiStore } from '@/store/uiStore'
import type { ToastMessage } from '@/types'

const VARIANT_STYLES: Record<ToastMessage['variant'], string> = {
  success: 'border-green-500/30 bg-green-500/10 text-green-300',
  error: 'border-red-500/30 bg-red-500/10 text-red-300',
  warning: 'border-amber-500/30 bg-amber-500/10 text-amber-300',
  info: 'border-blue-500/30 bg-blue-500/10 text-blue-300',
}

const VARIANT_ICONS: Record<ToastMessage['variant'], string> = {
  success: 'M5 13l4 4L19 7',
  error: 'M6 18L18 6M6 6l12 12',
  warning: 'M12 9v4m0 4h.01M12 2L2 20h20L12 2z',
  info: 'M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z',
}

function ToastItem({ toast }: { toast: ToastMessage }) {
  const { removeToast } = useUiStore()
  const duration = toast.duration ?? 5000

  useEffect(() => {
    const timer = setTimeout(() => removeToast(toast.id), duration)
    return () => clearTimeout(timer)
  }, [toast.id, duration, removeToast])

  return (
    <motion.div
      layout
      initial={{ opacity: 0, y: -12, scale: 0.96 }}
      animate={{ opacity: 1, y: 0, scale: 1 }}
      exit={{ opacity: 0, scale: 0.96, y: -8 }}
      transition={{ duration: 0.2, ease: [0.16, 1, 0.3, 1] }}
      className={`flex items-start gap-3 p-3.5 rounded-lg border text-sm max-w-xs shadow-xl shadow-black/40 backdrop-blur-md ${VARIANT_STYLES[toast.variant]}`}
      role="alert"
    >
      <svg
        className="shrink-0 w-4 h-4 mt-0.5"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden="true"
      >
        <path d={VARIANT_ICONS[toast.variant]} />
      </svg>
      <span className="flex-1 text-sm leading-snug">{toast.message}</span>
      <button
        onClick={() => removeToast(toast.id)}
        className="shrink-0 opacity-60 hover:opacity-100 transition-opacity cursor-pointer"
        aria-label="Dismiss notification"
      >
        <svg width="12" height="12" viewBox="0 0 12 12" fill="none" aria-hidden="true">
          <path d="M1 1l10 10M11 1L1 11" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </svg>
      </button>
    </motion.div>
  )
}

export function ToastContainer() {
  const { toasts } = useUiStore()

  return (
    <div
      className="fixed top-4 right-4 z-[100] flex flex-col gap-2"
      aria-live="polite"
      aria-label="Notifications"
    >
      <AnimatePresence mode="popLayout">
        {toasts.map((toast) => (
          <ToastItem key={toast.id} toast={toast} />
        ))}
      </AnimatePresence>
    </div>
  )
}
