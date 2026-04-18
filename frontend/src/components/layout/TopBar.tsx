import { useState } from 'react'
import { useLocation } from 'react-router-dom'
import { useUiStore } from '@/store/uiStore'

const PAGE_TITLES: Record<string, string> = {
  '/': 'Dashboard',
  '/jobs': 'Jobs',
  '/workers': 'Workers',
  '/dlq': 'Dead Letter Queue',
}

export function TopBar() {
  const { pathname } = useLocation()
  const title = PAGE_TITLES[pathname] ?? 'Dashboard'
  const { apiKey, setApiKey } = useUiStore()
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState('')

  function openEdit() {
    setDraft(apiKey)
    setEditing(true)
  }

  function save() {
    setApiKey(draft.trim())
    setEditing(false)
  }

  return (
    <header className="flex items-center px-6 h-14 border-b border-[#1e1e2e] bg-[#0a0a0f]/80 backdrop-blur-sm shrink-0 gap-3">
      <h1 className="text-sm font-semibold text-[#e2e2f0]">{title}</h1>

      <div className="flex items-center gap-2 ml-auto">
        {/* API key input */}
        {editing ? (
          <form
            className="flex items-center gap-1"
            onSubmit={(e) => { e.preventDefault(); save() }}
          >
            <input
              autoFocus
              type="password"
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              placeholder="paste API key…"
              className="h-7 px-2 rounded bg-[#12121e] border border-[#2e2e4a] text-[11px] text-[#e2e2f0] font-mono w-52 focus:outline-none focus:border-[#7c6af7]"
            />
            <button
              type="submit"
              className="h-7 px-2 rounded bg-[#7c6af7] text-white text-[11px] font-medium hover:bg-[#9b8cf9]"
            >
              Save
            </button>
            <button
              type="button"
              onClick={() => setEditing(false)}
              className="h-7 px-2 rounded bg-[#1e1e2e] text-[#6b6b8a] text-[11px] hover:text-[#e2e2f0]"
            >
              Cancel
            </button>
          </form>
        ) : (
          <button
            onClick={openEdit}
            title={apiKey ? `Key: ${apiKey.slice(0, 8)}…` : 'Set API key'}
            className="flex items-center gap-1.5 h-7 px-2 rounded bg-[#12121e] border border-[#2e2e4a] text-[11px] text-[#6b6b8a] hover:text-[#e2e2f0] hover:border-[#7c6af7] transition-colors"
          >
            <svg className="w-3 h-3" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 5.25a3 3 0 013 3m3 0a6 6 0 01-7.029 5.912c-.563-.097-1.159.026-1.563.43L10.5 17.25H8.25v2.25H6v2.25H2.25v-2.818c0-.597.237-1.17.659-1.591l6.499-6.499c.404-.404.527-1 .43-1.563A6 6 0 1121.75 8.25z" />
            </svg>
            {apiKey ? (
              <span className="font-mono">{apiKey.slice(0, 8)}…</span>
            ) : (
              <span>API key</span>
            )}
          </button>
        )}

        {/* Time */}
        <span className="text-[11px] text-[#6b6b8a] font-mono tabular-nums">
          {new Date().toUTCString().slice(17, 25)} UTC
        </span>
        {/* Status dot */}
        <div className={`w-2 h-2 rounded-full shadow-[0_0_8px] ${apiKey ? 'bg-[#7c6af7] shadow-[#7c6af7]' : 'bg-[#3a3a5a] shadow-transparent'}`} />
      </div>
    </header>
  )
}
