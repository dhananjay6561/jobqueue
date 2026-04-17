import { create } from 'zustand'
import type { ToastMessage, JobStatus } from '@/types'

interface JobFilters {
  status: JobStatus | 'all'
  type: string
  priorityMin: number
  priorityMax: number
  sortBy: 'created_at' | 'updated_at' | 'priority' | 'status'
  sortDir: 'asc' | 'desc'
  page: number
  pageSize: number
}

interface UiStoreState {
  sidebarCollapsed: boolean
  toasts: ToastMessage[]
  jobFilters: JobFilters
  setSidebarCollapsed: (collapsed: boolean) => void
  toggleSidebar: () => void
  addToast: (toast: Omit<ToastMessage, 'id'>) => void
  removeToast: (id: string) => void
  setJobFilters: (filters: Partial<JobFilters>) => void
  resetJobFilters: () => void
}

const defaultJobFilters: JobFilters = {
  status: 'all',
  type: '',
  priorityMin: 1,
  priorityMax: 10,
  sortBy: 'created_at',
  sortDir: 'desc',
  page: 0,
  pageSize: 20,
}

export const useUiStore = create<UiStoreState>((set) => ({
  sidebarCollapsed: false,
  toasts: [],
  jobFilters: defaultJobFilters,

  setSidebarCollapsed: (collapsed) => set({ sidebarCollapsed: collapsed }),
  toggleSidebar: () =>
    set((state) => ({ sidebarCollapsed: !state.sidebarCollapsed })),

  addToast: (toast) =>
    set((state) => ({
      toasts: [
        ...state.toasts,
        { ...toast, id: crypto.randomUUID() },
      ],
    })),
  removeToast: (id) =>
    set((state) => ({ toasts: state.toasts.filter((t) => t.id !== id) })),

  setJobFilters: (filters) =>
    set((state) => ({ jobFilters: { ...state.jobFilters, ...filters } })),
  resetJobFilters: () => set({ jobFilters: defaultJobFilters }),
}))
