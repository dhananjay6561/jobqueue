import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { motion } from 'framer-motion'
import { register, login, forgotPassword, resetPassword } from '@/api/auth'
import { useAuthStore } from '@/store/authStore'
import { useUiStore } from '@/store/uiStore'

function Logo() {
  return (
    <div className="flex items-center gap-3">
      <div className="w-8 h-8 rounded-lg bg-[#7c6af7] flex items-center justify-center shadow-lg shadow-[#7c6af7]/30">
        <svg className="w-4 h-4 text-white" viewBox="0 0 16 16" fill="currentColor">
          <path d="M2 2h5v5H2zM9 2h5v5H9zM2 9h5v5H2zM9 9h5v5H9z" />
        </svg>
      </div>
      <span className="font-semibold text-[#e2e2f0] font-mono">JobQueue</span>
    </div>
  )
}

function Card({ children }: { children: React.ReactNode }) {
  return (
    <div className="min-h-screen bg-[#0a0a0f] flex items-center justify-center p-4">
      <motion.div
        initial={{ opacity: 0, y: 16 }}
        animate={{ opacity: 1, y: 0 }}
        className="w-full max-w-md bg-[#111118] border border-[#1e1e2e] rounded-2xl p-8 space-y-6"
      >
        {children}
      </motion.div>
    </div>
  )
}

// ── Reset password page (rendered when ?token= is in URL) ────────────────────
function ResetPasswordPage({ token }: { token: string }) {
  const navigate = useNavigate()
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [loading, setLoading] = useState(false)
  const [done, setDone] = useState(false)
  const { addToast } = useUiStore()

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (password !== confirm) {
      addToast({ variant: 'error', message: 'Passwords do not match' })
      return
    }
    setLoading(true)
    try {
      await resetPassword(token, password)
      setDone(true)
    } catch (err: unknown) {
      addToast({ variant: 'error', message: err instanceof Error ? err.message : 'Reset failed' })
    } finally {
      setLoading(false)
    }
  }

  if (done) {
    return (
      <Card>
        <Logo />
        <div className="space-y-1">
          <h2 className="text-lg font-semibold text-[#e2e2f0]">Password updated</h2>
          <p className="text-sm text-[#6b6b8a]">You can now sign in with your new password.</p>
        </div>
        <button
          onClick={() => navigate('/auth')}
          className="w-full py-2.5 rounded-lg bg-[#7c6af7] hover:bg-[#6a58e0] text-white text-sm font-medium transition-colors"
        >
          Go to sign in
        </button>
      </Card>
    )
  }

  return (
    <Card>
      <Logo />
      <div className="space-y-1">
        <h2 className="text-lg font-semibold text-[#e2e2f0]">Set new password</h2>
        <p className="text-sm text-[#6b6b8a]">Choose a password with at least 8 characters.</p>
      </div>
      <form onSubmit={handleSubmit} className="space-y-4">
        <div className="space-y-1.5">
          <label className="text-xs text-[#6b6b8a] uppercase tracking-widest font-mono">New password</label>
          <input
            type="password" required minLength={8}
            value={password} onChange={(e) => setPassword(e.target.value)}
            placeholder="••••••••"
            className="w-full bg-[#0d0d14] border border-[#1e1e2e] rounded-lg px-3 py-2.5 text-sm text-[#e2e2f0] placeholder-[#3a3a5e] focus:outline-none focus:border-[#7c6af7] transition-colors"
          />
        </div>
        <div className="space-y-1.5">
          <label className="text-xs text-[#6b6b8a] uppercase tracking-widest font-mono">Confirm password</label>
          <input
            type="password" required minLength={8}
            value={confirm} onChange={(e) => setConfirm(e.target.value)}
            placeholder="••••••••"
            className="w-full bg-[#0d0d14] border border-[#1e1e2e] rounded-lg px-3 py-2.5 text-sm text-[#e2e2f0] placeholder-[#3a3a5e] focus:outline-none focus:border-[#7c6af7] transition-colors"
          />
        </div>
        <button
          type="submit" disabled={loading}
          className="w-full py-2.5 rounded-lg bg-[#7c6af7] hover:bg-[#6a58e0] disabled:opacity-50 text-white text-sm font-medium transition-colors"
        >
          {loading ? 'Updating…' : 'Set new password'}
        </button>
      </form>
    </Card>
  )
}

// ── Forgot password view ─────────────────────────────────────────────────────
function ForgotPasswordView({ onBack }: { onBack: () => void }) {
  const [email, setEmail] = useState('')
  const [loading, setLoading] = useState(false)
  const [sent, setSent] = useState(false)
  const { addToast } = useUiStore()

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setLoading(true)
    try {
      await forgotPassword(email)
      setSent(true)
    } catch (err: unknown) {
      addToast({ variant: 'error', message: err instanceof Error ? err.message : 'Request failed' })
    } finally {
      setLoading(false)
    }
  }

  if (sent) {
    return (
      <div className="space-y-4">
        <div className="bg-green-500/10 border border-green-500/30 rounded-lg px-4 py-3 text-sm text-green-400">
          Check your inbox — a reset link is on its way to <strong>{email}</strong>.
          <br /><span className="text-xs opacity-70">If SMTP is not configured, check the server logs.</span>
        </div>
        <button onClick={onBack} className="text-xs text-[#7c6af7] hover:underline">← Back to sign in</button>
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <div className="space-y-1">
        <h3 className="text-sm font-semibold text-[#e2e2f0]">Forgot your password?</h3>
        <p className="text-xs text-[#6b6b8a]">Enter your email and we'll send a reset link.</p>
      </div>
      <form onSubmit={handleSubmit} className="space-y-3">
        <input
          type="email" required
          value={email} onChange={(e) => setEmail(e.target.value)}
          placeholder="you@example.com"
          className="w-full bg-[#0d0d14] border border-[#1e1e2e] rounded-lg px-3 py-2.5 text-sm text-[#e2e2f0] placeholder-[#3a3a5e] focus:outline-none focus:border-[#7c6af7] transition-colors"
        />
        <button
          type="submit" disabled={loading}
          className="w-full py-2.5 rounded-lg bg-[#7c6af7] hover:bg-[#6a58e0] disabled:opacity-50 text-white text-sm font-medium transition-colors"
        >
          {loading ? 'Sending…' : 'Send reset link'}
        </button>
      </form>
      <button onClick={onBack} className="text-xs text-[#6b6b8a] hover:text-[#e2e2f0]">← Back to sign in</button>
    </div>
  )
}

// ── Main Auth page ───────────────────────────────────────────────────────────
export function Auth() {
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const { setAuth } = useAuthStore()
  const { setApiKey, addToast } = useUiStore()

  const [tab, setTab] = useState<'login' | 'register'>('login')
  const [view, setView] = useState<'auth' | 'forgot'>('auth')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [newKey, setNewKey] = useState<string | null>(null)

  // Handle /auth/reset?token=... route
  const resetToken = params.get('token')
  if (resetToken) return <ResetPasswordPage token={resetToken} />

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
        navigate('/')
      }
    } catch (err: unknown) {
      addToast({ variant: 'error', message: err instanceof Error ? err.message : 'Authentication failed' })
    } finally {
      setLoading(false)
    }
  }

  if (newKey) {
    return (
      <Card>
        <Logo />
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
      </Card>
    )
  }

  return (
    <Card>
      <Logo />

      {view === 'forgot' ? (
        <ForgotPasswordView onBack={() => setView('auth')} />
      ) : (
        <>
          {/* Tabs */}
          <div className="flex bg-[#0d0d14] rounded-lg p-1 gap-1">
            {(['login', 'register'] as const).map((t) => (
              <button
                key={t}
                onClick={() => setTab(t)}
                className={`flex-1 py-1.5 rounded-md text-sm font-medium transition-all ${
                  tab === t ? 'bg-[#7c6af7] text-white' : 'text-[#6b6b8a] hover:text-[#e2e2f0]'
                }`}
              >
                {t === 'login' ? 'Sign in' : 'Register'}
              </button>
            ))}
          </div>

          {/* Demo button */}
          <button
            type="button"
            disabled={loading}
            onClick={async () => {
              const demoEmail = 'demo@jobqueue.dev'
              const demoPass = 'demo1234'
              setEmail(demoEmail)
              setPassword(demoPass)
              setLoading(true)
              try {
                const res = await login(demoEmail, demoPass)
                setAuth(res.token, res.user)
                navigate('/')
              } catch {
                try {
                  const res = await register(demoEmail, demoPass)
                  setAuth(res.token, res.user)
                  setApiKey(res.api_key.key)
                  setNewKey(res.api_key.key)
                } catch (err: unknown) {
                  addToast({ variant: 'error', message: err instanceof Error ? err.message : 'Demo login failed' })
                }
              } finally {
                setLoading(false)
              }
            }}
            className="w-full py-2 rounded-lg border border-dashed border-[#2a2a3e] text-xs text-[#6b6b8a] hover:text-[#e2e2f0] hover:border-[#7c6af7]/40 transition-colors disabled:opacity-50"
          >
            {loading ? 'Signing in…' : 'Continue with demo account'}
          </button>

          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-1.5">
              <label className="text-xs text-[#6b6b8a] uppercase tracking-widest font-mono">Email</label>
              <input
                type="email" required
                value={email} onChange={(e) => setEmail(e.target.value)}
                placeholder="you@example.com"
                className="w-full bg-[#0d0d14] border border-[#1e1e2e] rounded-lg px-3 py-2.5 text-sm text-[#e2e2f0] placeholder-[#3a3a5e] focus:outline-none focus:border-[#7c6af7] transition-colors"
              />
            </div>
            <div className="space-y-1.5">
              <div className="flex items-center justify-between">
                <label className="text-xs text-[#6b6b8a] uppercase tracking-widest font-mono">Password</label>
                {tab === 'login' && (
                  <button
                    type="button"
                    onClick={() => setView('forgot')}
                    className="text-xs text-[#6b6b8a] hover:text-[#7c6af7] transition-colors"
                  >
                    Forgot password?
                  </button>
                )}
              </div>
              <input
                type="password" required minLength={8}
                value={password} onChange={(e) => setPassword(e.target.value)}
                placeholder="••••••••"
                className="w-full bg-[#0d0d14] border border-[#1e1e2e] rounded-lg px-3 py-2.5 text-sm text-[#e2e2f0] placeholder-[#3a3a5e] focus:outline-none focus:border-[#7c6af7] transition-colors"
              />
            </div>
            <button
              type="submit" disabled={loading}
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
        </>
      )}
    </Card>
  )
}
