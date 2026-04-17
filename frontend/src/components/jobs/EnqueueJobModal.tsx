import { useState } from 'react'
import { Modal } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { useEnqueueJob } from '@/hooks/useJobs'
import type { QueueName } from '@/types'
import { QUEUE_NAMES, JOB_TYPES, PRIORITY_MIN, PRIORITY_MAX } from '@/utils/constants'

interface EnqueueJobModalProps {
  isOpen: boolean
  onClose: () => void
}

interface FormState {
  type: string
  customType: string
  payload: string
  priority: number
  maxAttempts: number
  queueName: QueueName
  payloadError: string
}

export function EnqueueJobModal({ isOpen, onClose }: EnqueueJobModalProps) {
  const enqueue = useEnqueueJob()
  const [form, setForm] = useState<FormState>({
    type: JOB_TYPES[0] ?? 'send_email',
    customType: '',
    payload: '{\n  \n}',
    priority: 5,
    maxAttempts: 3,
    queueName: 'default',
    payloadError: '',
  })

  const update = (patch: Partial<FormState>) => setForm((f) => ({ ...f, ...patch }))

  const validatePayload = (val: string): boolean => {
    try {
      JSON.parse(val)
      update({ payloadError: '' })
      return true
    } catch (e) {
      update({ payloadError: (e as Error).message })
      return false
    }
  }

  const handleSubmit = () => {
    if (!validatePayload(form.payload)) return
    const jobType = form.type === '__custom__' ? form.customType.trim() : form.type
    if (!jobType) return

    enqueue.mutate(
      {
        type: jobType,
        payload: JSON.parse(form.payload) as Record<string, unknown>,
        priority: form.priority,
        max_attempts: form.maxAttempts,
        queue_name: form.queueName,
      },
      { onSuccess: onClose },
    )
  }

  const labelClass = 'block text-[11px] text-[#6b6b8a] uppercase tracking-widest mb-1.5 font-mono'
  const inputClass =
    'w-full bg-[#0a0a0f] border border-[#1e1e2e] rounded-lg px-3 py-2 text-sm text-[#e2e2f0] font-mono focus:outline-none focus:border-[#7c6af7]/50 transition-colors'

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Enqueue Job"
      size="lg"
      footer={
        <>
          <Button variant="ghost" size="md" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" size="md" isLoading={enqueue.isPending} onClick={handleSubmit}>
            Enqueue Job
          </Button>
        </>
      }
    >
      <div className="space-y-5">
        {/* Job Type */}
        <div>
          <label className={labelClass} htmlFor="job-type">Job Type</label>
          <select
            id="job-type"
            className={inputClass}
            value={form.type}
            onChange={(e) => update({ type: e.target.value })}
          >
            {JOB_TYPES.map((t) => (
              <option key={t} value={t}>{t}</option>
            ))}
            <option value="__custom__">Custom…</option>
          </select>
          {form.type === '__custom__' && (
            <input
              type="text"
              className={`${inputClass} mt-2`}
              placeholder="my_custom_job_type"
              value={form.customType}
              onChange={(e) => update({ customType: e.target.value })}
            />
          )}
        </div>

        {/* Queue */}
        <div>
          <label className={labelClass} htmlFor="queue-name">Queue</label>
          <select
            id="queue-name"
            className={inputClass}
            value={form.queueName}
            onChange={(e) => update({ queueName: e.target.value as QueueName })}
          >
            {QUEUE_NAMES.map((q) => (
              <option key={q} value={q}>{q}</option>
            ))}
          </select>
        </div>

        {/* Priority */}
        <div>
          <label className={labelClass} htmlFor="priority">
            Priority — <span className="text-[#7c6af7]">P{form.priority}</span>
          </label>
          <div className="flex items-center gap-3">
            <span className="text-[10px] text-[#6b6b8a] font-mono">Low {PRIORITY_MIN}</span>
            <input
              id="priority"
              type="range"
              min={PRIORITY_MIN}
              max={PRIORITY_MAX}
              value={form.priority}
              onChange={(e) => update({ priority: Number(e.target.value) })}
              className="flex-1 accent-[#7c6af7]"
            />
            <span className="text-[10px] text-[#6b6b8a] font-mono">High {PRIORITY_MAX}</span>
          </div>
        </div>

        {/* Max Attempts */}
        <div>
          <label className={labelClass} htmlFor="max-attempts">Max Attempts</label>
          <input
            id="max-attempts"
            type="number"
            min={1}
            max={10}
            className={inputClass}
            value={form.maxAttempts}
            onChange={(e) => update({ maxAttempts: Number(e.target.value) })}
          />
        </div>

        {/* Payload */}
        <div>
          <label className={labelClass} htmlFor="payload">Payload (JSON)</label>
          <textarea
            id="payload"
            rows={8}
            className={`${inputClass} resize-y text-[12px] leading-relaxed ${form.payloadError ? 'border-red-500/50' : ''}`}
            value={form.payload}
            onChange={(e) => update({ payload: e.target.value })}
            onBlur={(e) => validatePayload(e.target.value)}
            spellCheck={false}
          />
          {form.payloadError && (
            <p className="mt-1 text-[11px] text-red-400 font-mono">{form.payloadError}</p>
          )}
        </div>
      </div>
    </Modal>
  )
}
