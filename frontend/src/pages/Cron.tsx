import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { listCron, patchCron, deleteCron, createCron } from '@/api/cron'
import type { CronSchedule, CreateCronRequest } from '@/types'

function formatDate(s: string | null) {
  if (!s) return '—'
  return new Date(s).toLocaleString()
}

function Toggle({ enabled, onChange }: { enabled: boolean; onChange: (v: boolean) => void }) {
  return (
    <button
      onClick={() => onChange(!enabled)}
      className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors ${enabled ? 'bg-[#7c6af7]' : 'bg-[#2e2e4a]'}`}
      aria-label={enabled ? 'Disable' : 'Enable'}
    >
      <span
        className={`inline-block h-3.5 w-3.5 transform rounded-full bg-white transition-transform ${enabled ? 'translate-x-4' : 'translate-x-1'}`}
      />
    </button>
  )
}

const defaultForm: CreateCronRequest = {
  name: '',
  job_type: '',
  cron_expression: '',
  queue_name: 'default',
  priority: 5,
  max_attempts: 3,
}

export default function CronPage() {
  const qc = useQueryClient()
  const [showForm, setShowForm] = useState(false)
  const [form, setForm] = useState<CreateCronRequest>(defaultForm)
  const [formErr, setFormErr] = useState('')

  const { data: schedules = [], isLoading } = useQuery({
    queryKey: ['cron'],
    queryFn: listCron,
    refetchInterval: 30_000,
  })

  const toggleMutation = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      patchCron(id, { enabled }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['cron'] }),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteCron(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['cron'] }),
  })

  const createMutation = useMutation({
    mutationFn: (req: CreateCronRequest) => createCron(req),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['cron'] })
      setShowForm(false)
      setForm(defaultForm)
      setFormErr('')
    },
    onError: (e: Error) => setFormErr(e.message),
  })

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!form.name || !form.job_type || !form.cron_expression) {
      setFormErr('name, job_type, and cron_expression are required')
      return
    }
    createMutation.mutate(form)
  }

  return (
    <div className="flex flex-col h-full overflow-auto p-6 space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-base font-semibold text-[#e2e2f0]">Cron Schedules</h2>
          <p className="text-[11px] text-[#6b6b8a] mt-0.5">{schedules.length} schedule{schedules.length !== 1 ? 's' : ''}</p>
        </div>
        <button
          onClick={() => { setShowForm(true); setFormErr('') }}
          className="flex items-center gap-1.5 h-8 px-3 rounded bg-[#7c6af7] text-white text-xs font-medium hover:bg-[#9b8cf9]"
        >
          <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" d="M12 4v16m8-8H4" />
          </svg>
          New schedule
        </button>
      </div>

      {/* Create form */}
      {showForm && (
        <form onSubmit={handleSubmit} className="rounded-lg border border-[#2e2e4a] bg-[#0d0d14] p-4 space-y-3">
          <p className="text-xs font-medium text-[#e2e2f0]">New cron schedule</p>
          <div className="grid grid-cols-2 gap-3">
            {[
              { label: 'Name', key: 'name', placeholder: 'daily-report' },
              { label: 'Job type', key: 'job_type', placeholder: 'generate_report' },
              { label: 'Cron expression', key: 'cron_expression', placeholder: '0 9 * * *' },
              { label: 'Queue', key: 'queue_name', placeholder: 'default' },
            ].map(({ label, key, placeholder }) => (
              <label key={key} className="flex flex-col gap-1">
                <span className="text-[10px] text-[#6b6b8a] uppercase tracking-wide">{label}</span>
                <input
                  value={(form as unknown as Record<string, string>)[key]}
                  onChange={e => setForm(f => ({ ...f, [key]: e.target.value }))}
                  placeholder={placeholder}
                  className="h-8 px-2 rounded bg-[#12121e] border border-[#2e2e4a] text-xs text-[#e2e2f0] focus:outline-none focus:border-[#7c6af7]"
                />
              </label>
            ))}
          </div>
          {formErr && <p className="text-xs text-red-400">{formErr}</p>}
          <div className="flex gap-2">
            <button type="submit" disabled={createMutation.isPending}
              className="h-8 px-3 rounded bg-[#7c6af7] text-white text-xs font-medium hover:bg-[#9b8cf9] disabled:opacity-50">
              {createMutation.isPending ? 'Creating…' : 'Create'}
            </button>
            <button type="button" onClick={() => setShowForm(false)}
              className="h-8 px-3 rounded bg-[#1e1e2e] text-[#6b6b8a] text-xs hover:text-[#e2e2f0]">
              Cancel
            </button>
          </div>
        </form>
      )}

      {/* Table */}
      {isLoading ? (
        <p className="text-xs text-[#6b6b8a]">Loading…</p>
      ) : schedules.length === 0 ? (
        <div className="flex flex-col items-center justify-center flex-1 gap-2 text-[#6b6b8a]">
          <svg className="w-10 h-10 opacity-30" fill="none" stroke="currentColor" strokeWidth={1} viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" d="M12 6v6l4 2m6-2a10 10 0 11-20 0 10 10 0 0120 0z" />
          </svg>
          <p className="text-sm">No cron schedules yet</p>
        </div>
      ) : (
        <div className="rounded-lg border border-[#1e1e2e] overflow-hidden">
          <table className="w-full text-xs">
            <thead>
              <tr className="border-b border-[#1e1e2e] bg-[#0d0d14]">
                {['Name', 'Job type', 'Expression', 'Queue', 'Next run', 'Last run', 'Enabled', ''].map(h => (
                  <th key={h} className="text-left px-4 py-2.5 text-[10px] uppercase tracking-wide text-[#6b6b8a] font-medium">{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {schedules.map((s: CronSchedule) => (
                <tr key={s.id} className="border-b border-[#1e1e2e] last:border-0 hover:bg-white/[0.02]">
                  <td className="px-4 py-3 text-[#e2e2f0] font-medium">{s.name}</td>
                  <td className="px-4 py-3 text-[#a0a0c0] font-mono">{s.job_type}</td>
                  <td className="px-4 py-3 text-[#7c6af7] font-mono">{s.cron_expression}</td>
                  <td className="px-4 py-3 text-[#a0a0c0]">{s.queue_name}</td>
                  <td className="px-4 py-3 text-[#a0a0c0]">{formatDate(s.next_run_at)}</td>
                  <td className="px-4 py-3 text-[#6b6b8a]">{formatDate(s.last_run_at)}</td>
                  <td className="px-4 py-3">
                    <Toggle
                      enabled={s.enabled}
                      onChange={(v) => toggleMutation.mutate({ id: s.id, enabled: v })}
                    />
                  </td>
                  <td className="px-4 py-3">
                    <button
                      onClick={() => { if (confirm(`Delete "${s.name}"?`)) deleteMutation.mutate(s.id) }}
                      className="text-[#6b6b8a] hover:text-red-400 transition-colors"
                      aria-label="Delete"
                    >
                      <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                      </svg>
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
