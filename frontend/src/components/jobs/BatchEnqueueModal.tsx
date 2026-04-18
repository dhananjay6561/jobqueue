import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Modal } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { enqueueBatch } from '@/api/jobs'
import type { EnqueueJobRequest } from '@/types'

interface Props {
  isOpen: boolean
  onClose: () => void
}

const PLACEHOLDER = `[
  { "type": "send_email", "payload": { "to": "a@example.com" }, "priority": 5 },
  { "type": "resize_image", "payload": { "url": "https://..." }, "priority": 3 }
]`

export function BatchEnqueueModal({ isOpen, onClose }: Props) {
  const qc = useQueryClient()
  const [json, setJson] = useState('')
  const [parseErr, setParseErr] = useState('')
  const [result, setResult] = useState<{ created: number } | null>(null)

  const mutation = useMutation({
    mutationFn: (jobs: EnqueueJobRequest[]) => enqueueBatch(jobs),
    onSuccess: (jobs) => {
      qc.invalidateQueries({ queryKey: ['jobs'] })
      setResult({ created: jobs.length })
    },
    onError: (e: Error) => setParseErr(e.message),
  })

  function handleSubmit() {
    setParseErr('')
    let parsed: unknown
    try {
      parsed = JSON.parse(json)
    } catch (e) {
      setParseErr('Invalid JSON: ' + (e as Error).message)
      return
    }
    if (!Array.isArray(parsed)) {
      setParseErr('Expected a JSON array of job objects')
      return
    }
    mutation.mutate(parsed as EnqueueJobRequest[])
  }

  function handleClose() {
    setJson('')
    setParseErr('')
    setResult(null)
    onClose()
  }

  const inputClass =
    'w-full bg-[#0a0a0f] border border-[#1e1e2e] rounded-lg px-3 py-2 text-sm text-[#e2e2f0] font-mono focus:outline-none focus:border-[#7c6af7]/50 transition-colors'

  return (
    <Modal
      isOpen={isOpen}
      onClose={handleClose}
      title="Batch Enqueue"
      size="lg"
      footer={
        result ? (
          <Button variant="primary" size="md" onClick={handleClose}>Done</Button>
        ) : (
          <>
            <Button variant="ghost" size="md" onClick={handleClose}>Cancel</Button>
            <Button variant="primary" size="md" isLoading={mutation.isPending} onClick={handleSubmit}>
              Enqueue Batch
            </Button>
          </>
        )
      }
    >
      {result ? (
        <div className="flex flex-col items-center gap-3 py-6">
          <div className="w-12 h-12 rounded-full bg-green-500/10 flex items-center justify-center">
            <svg className="w-6 h-6 text-green-400" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
            </svg>
          </div>
          <p className="text-sm text-[#e2e2f0] font-semibold">{result.created} job{result.created !== 1 ? 's' : ''} enqueued</p>
        </div>
      ) : (
        <div className="space-y-4">
          <p className="text-xs text-[#6b6b8a]">
            Paste a JSON array of job objects. Each item supports the same fields as a single enqueue request
            (<code className="text-[#7c6af7]">type</code>, <code className="text-[#7c6af7]">payload</code>,{' '}
            <code className="text-[#7c6af7]">priority</code>, <code className="text-[#7c6af7]">queue_name</code>,{' '}
            <code className="text-[#7c6af7]">max_attempts</code>, <code className="text-[#7c6af7]">ttl_seconds</code>).
            Maximum 500 jobs per batch.
          </p>
          <div>
            <label className="block text-[11px] text-[#6b6b8a] uppercase tracking-widest mb-1.5 font-mono">
              Jobs JSON Array
            </label>
            <textarea
              rows={12}
              className={`${inputClass} resize-y text-[12px] leading-relaxed ${parseErr ? 'border-red-500/50' : ''}`}
              value={json}
              onChange={(e) => setJson(e.target.value)}
              placeholder={PLACEHOLDER}
              spellCheck={false}
            />
            {parseErr && <p className="mt-1 text-[11px] text-red-400 font-mono">{parseErr}</p>}
          </div>
        </div>
      )}
    </Modal>
  )
}
