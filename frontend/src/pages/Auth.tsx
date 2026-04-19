import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { motion } from 'framer-motion'
import { register, login } from '@/api/auth'
import { useAuthStore } from '@/store/authStore'
import { useUiStore } from '@/store/uiStore'

export function Auth() {
  const navigate = useNavigate()
  const { setAuth } = useAuthStore()
  const { setApiKey, addToast } = useUiStore()

  const [tab, setTab] = useState<'login' | 'register'>('login')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [newKey, setNewKey] = useState<string | null>(null)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setLoading(true)
    try {
      if (tab === 'register') {
        const res = await register(email, password)
        setAuth(res.token, res.user)
        setApiKey(res.api_key.key)
        setNewKey(res.api_key.key)
      } else {
        const res = await login(email, password)
        setAuth(res.token, res.user)
        if (res.keys?.length) setApiKey('')
        navigate('/')
      }
    } catch (err: unknown) {
      addToast({
        type: 'error',
        message: err instanceof Error ? err.message : 'Authentication failed',
      })
    } finally {
      setLoading(false)
    }
  }

  if (newKey) {
    return (
      <div className="min-h-screen bg-[#0a0a0f] flex items-center justify-center p-4">
        <motion.div
          initial={{ opacity: 0, y: 16 }}
          animate={{ opacity: 1, y: 0 }}
          className="w-full max-w-md bg-[#111118] border border-[#1e1e2e] rounded-2xl p-8 space-y-6"
        >
          <div className="space-y-1">
            <h2 className="text-lg font-semibold text-[#e2e2f0]">Account created</h2>
            <p className="text-sm text-[#6b6b8a]">Your API key is shown once — save it now.</p>
          </div>
          <div className="bg-[#0d0d14] border border-[#2a2a3e] rounded-lg p-4 font-mono text-sm text-green-400 break-all select-all">
            {newKey}
          </div>
          <button
            onClick={() => { setNewKey(null); navigate('/') }}
            className="w-full py-2.5 rounded-lg bg-[#7c6af7] hover:bg-[#6a58e0] text-white text-sm font-medium transition-colors"
          >
            Continue to dashboard
          </button>
        </motion.div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-[#0a0a0f] flex items-center justify-center p-4">
      <motion.div
        initial={{ opacity: 0, y: 16 }}
        animate={{ opacity: 1, y: 0 }}
        className="w-full max-w-md bg-[#111118] border border-[#1e1e2e] rounded-2xl p-8 space-y-6"
      >
        {/* Logo */}
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 rounded-lg bg-[#7c6af7] flex items-center justify-center shadow-lg shadow-[#7c6af7]/30">
            <svg className="w-4 h-4 text-white" viewBox="0 0 16 16" fill="currentColor">
              <path d="M2 2h5v5H2zM9 2h5v5H9zM2 9h5v5H2zM9 9h5v5H9z" />
            </svg>
          </div>
          <span className="font-semibold text-[#e2e2f0] font-mono">JobQueue</span>
        </div>

        {/* Tabs */}
        <div className="flex bg-[#0d0d14] rounded-lg p-1 gap-1">
          {(['login', 'register'] as const).map((t) => (
            <button
              key={t}
              onClick={() => setTab(t)}
              className={`flex-1 py-1.5 rounded-md text-sm font-medium transition-all ${
                tab === t
                  ? 'bg-[#7c6af7] text-white'
                  : 'text-[#6b6b8a] hover:text-[#e2e2f0]'
              }`}
            >
              {t === 'login' ? 'Sign in' : 'Register'}
            </button>
          ))}
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-1.5">
            <label className="text-xs text-[#6b6b8a] uppercase tracking-widest font-mono">Email</label>
            <input
              type="email"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="you@example.com"
              className="w-full bg-[#0d0d14] border border-[#1e1e2e] rounded-lg px-3 py-2.5 text-sm text-[#e2e2f0] placeholder-[#3a3a5e] focus:outline-none focus:border-[#7c6af7] transition-colors"
            />
          </div>
          <div className="space-y-1.5">
            <label className="text-xs text-[#6b6b8a] uppercase tracking-widest font-mono">Password</label>
            <input
              type="password"
              required
              minLength={8}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="••••••••"
              className="w-full bg-[#0d0d14] border border-[#1e1e2e] rounded-lg px-3 py-2.5 text-sm text-[#e2e2f0] placeholder-[#3a3a5e] focus:outline-none focus:border-[#7c6af7] transition-colors"
            />
          </div>
          <button
            type="submit"
            disabled={loading}
            className="w-full py-2.5 rounded-lg bg-[#7c6af7] hover:bg-[#6a58e0] disabled:opacity-50 text-white text-sm font-medium transition-colors"
          >
            {loading ? 'Please wait…' : tab === 'login' ? 'Sign in' : 'Create account'}
          </button>
        </form>

        <p className="text-center text-xs text-[#6b6b8a]">
          {tab === 'login' ? "Don't have an account? " : 'Already have an account? '}
          <button
            onClick={() => setTab(tab === 'login' ? 'register' : 'login')}
            className="text-[#7c6af7] hover:underline"
          >
            {tab === 'login' ? 'Register' : 'Sign in'}
          </button>
        </p>
      </motion.div>
    </div>
  )
}
