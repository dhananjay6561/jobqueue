import { useNavigate } from 'react-router-dom'
import { motion } from 'framer-motion'
import { useAuthStore } from '@/store/authStore'

const features = [
  {
    icon: 'M3 3h6v6H3zM11 3h6v6h-6zM3 11h6v6H3zM11 11h6v6h-6z',
    title: 'Real-time Dashboard',
    desc: 'Monitor every job, worker, and queue live via WebSocket — no polling, no stale data.',
  },
  {
    icon: 'M13 6a3 3 0 11-6 0 3 3 0 016 0zM3 19a7 7 0 0114 0',
    title: 'Distributed Workers',
    desc: 'Scale workers independently. Each heartbeats autonomously so the system self-heals on failure.',
  },
  {
    icon: 'M12 9v4m0 4h.01M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z',
    title: 'Dead Letter Queue',
    desc: 'Failed jobs land in the DLQ with full context. Inspect, requeue, or discard — one click.',
  },
  {
    icon: 'M10 2a8 8 0 100 16A8 8 0 0010 2zm0 3v5l3 3',
    title: 'Cron Scheduling',
    desc: 'Define recurring jobs with standard cron expressions. Promotions are automatic and reliable.',
  },
  {
    icon: 'M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1',
    title: 'Webhook Delivery',
    desc: 'Get notified on job lifecycle events. Register endpoints and let Queuely push updates to you.',
  },
  {
    icon: 'M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4',
    title: 'API-first',
    desc: 'Everything is a REST endpoint. Enqueue jobs, manage keys, query stats — all over HTTP.',
  },
]

const plans = [
  {
    name: 'Free',
    price: '$0',
    period: '/mo',
    limit: '1,000 jobs/month',
    features: ['REST API access', 'Real-time dashboard', 'Dead letter queue', 'Community support'],
    cta: 'Get started free',
    highlight: false,
    available: true,
  },
  {
    name: 'Pro',
    price: '$29',
    period: '/mo',
    limit: '100,000 jobs/month',
    features: ['Everything in Free', 'Priority queue', 'Webhook support', 'Email support'],
    cta: 'Coming Soon',
    highlight: true,
    available: false,
  },
  {
    name: 'Business',
    price: '$99',
    period: '/mo',
    limit: 'Unlimited jobs',
    features: ['Everything in Pro', 'Dedicated support', 'SLA guarantee', 'Custom limits'],
    cta: 'Coming Soon',
    highlight: false,
    available: false,
  },
]

const codeExample = `curl -X POST https://yourdomain.com/api/v1/jobs \\
  -H "X-API-Key: jq_live_••••••••••••••••" \\
  -H "Content-Type: application/json" \\
  -d '{
    "type": "send_email",
    "payload": { "to": "user@example.com" },
    "queue": "default",
    "max_attempts": 3
  }'`

const fade = { hidden: { opacity: 0, y: 20 }, show: { opacity: 1, y: 0 } }

export function Landing() {
  const navigate = useNavigate()
  const { token } = useAuthStore()

  return (
    <div className="min-h-screen bg-[#0a0a0f] text-[#e2e2f0] font-sans">

      {/* ── Navbar ── */}
      <header className="sticky top-0 z-50 border-b border-[#1e1e2e] bg-[#0a0a0f]/80 backdrop-blur-md">
        <div className="max-w-6xl mx-auto px-6 h-14 flex items-center justify-between">
          <div className="flex items-center gap-2.5">
            <div className="w-7 h-7 rounded-lg bg-[#7c6af7] flex items-center justify-center shadow-lg shadow-[#7c6af7]/30">
              <svg className="w-4 h-4 text-white" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
                <path d="M2 2h5v5H2zM9 2h5v5H9zM2 9h5v5H2zM9 9h5v5H9z" />
              </svg>
            </div>
            <span className="font-semibold text-sm tracking-tight font-mono">Queuely</span>
          </div>
          <nav className="flex items-center gap-1">
            {token ? (
              <button
                onClick={() => navigate('/dashboard')}
                className="px-4 py-1.5 rounded-lg bg-[#7c6af7] hover:bg-[#6a58e0] text-white text-sm font-medium transition-colors"
              >
                Dashboard →
              </button>
            ) : (
              <>
                <button
                  onClick={() => navigate('/auth')}
                  className="px-4 py-1.5 rounded-lg text-[#6b6b8a] hover:text-[#e2e2f0] text-sm transition-colors"
                >
                  Sign in
                </button>
                <button
                  onClick={() => navigate('/auth')}
                  className="px-4 py-1.5 rounded-lg bg-[#7c6af7] hover:bg-[#6a58e0] text-white text-sm font-medium transition-colors"
                >
                  Get started free
                </button>
              </>
            )}
          </nav>
        </div>
      </header>

      {/* ── Hero ── */}
      <section className="relative overflow-hidden">
        <div className="absolute inset-0 pointer-events-none">
          <div className="absolute top-[-200px] left-1/2 -translate-x-1/2 w-[900px] h-[500px] bg-[#7c6af7]/10 rounded-full blur-[120px]" />
        </div>
        <div className="relative max-w-6xl mx-auto px-6 pt-24 pb-20 text-center">
          <motion.div
            variants={fade}
            initial="hidden"
            animate="show"
            transition={{ duration: 0.5 }}
          >
            <span className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-[#7c6af7]/10 border border-[#7c6af7]/25 text-[#7c6af7] text-xs font-mono mb-6">
              <span className="w-1.5 h-1.5 rounded-full bg-[#7c6af7] animate-pulse" />
              Open source · Free tier available
            </span>
            <h1 className="text-4xl sm:text-5xl md:text-6xl font-bold tracking-tight text-[#e2e2f0] leading-tight max-w-3xl mx-auto">
              Job queues that{' '}
              <span className="text-[#7c6af7]">just work</span>
            </h1>
            <p className="mt-5 text-lg text-[#6b6b8a] max-w-xl mx-auto leading-relaxed">
              Enqueue, schedule, and monitor distributed jobs via a clean REST API.
              Real-time dashboard. Dead letter queue. Cron. Webhooks. No infra headaches.
            </p>
            <div className="mt-8 flex items-center justify-center gap-3 flex-wrap">
              <button
                onClick={() => navigate('/auth')}
                className="px-6 py-2.5 rounded-lg bg-[#7c6af7] hover:bg-[#6a58e0] text-white font-medium transition-colors shadow-lg shadow-[#7c6af7]/20"
              >
                Get started free
              </button>
              <a
                href="https://github.com/dhananjay6561/jobqueue"
                target="_blank"
                rel="noopener noreferrer"
                className="px-6 py-2.5 rounded-lg border border-[#2a2a3e] text-[#6b6b8a] hover:text-[#e2e2f0] hover:border-[#3a3a5e] font-medium transition-colors"
              >
                View on GitHub
              </a>
            </div>
          </motion.div>

          {/* Mini dashboard preview */}
          <motion.div
            variants={fade}
            initial="hidden"
            animate="show"
            transition={{ duration: 0.5, delay: 0.15 }}
            className="mt-14 rounded-xl border border-[#1e1e2e] bg-[#0d0d14] overflow-hidden shadow-2xl shadow-black/40 max-w-3xl mx-auto"
          >
            <div className="flex items-center gap-1.5 px-4 py-3 border-b border-[#1e1e2e]">
              <span className="w-3 h-3 rounded-full bg-red-500/60" />
              <span className="w-3 h-3 rounded-full bg-amber-500/60" />
              <span className="w-3 h-3 rounded-full bg-green-500/60" />
              <span className="ml-3 text-xs text-[#6b6b8a] font-mono">Queuely Dashboard</span>
            </div>
            <div className="p-5 grid grid-cols-2 sm:grid-cols-4 gap-3">
              {[
                { label: 'Queued', value: '1,284', color: 'text-[#7c6af7]' },
                { label: 'Running', value: '42', color: 'text-amber-400' },
                { label: 'Completed', value: '98,310', color: 'text-green-400' },
                { label: 'Failed', value: '7', color: 'text-red-400' },
              ].map((s) => (
                <div key={s.label} className="bg-[#111118] rounded-lg p-3 border border-[#1e1e2e]">
                  <div className={`text-xl font-bold font-mono ${s.color}`}>{s.value}</div>
                  <div className="text-[11px] text-[#6b6b8a] mt-0.5">{s.label}</div>
                </div>
              ))}
            </div>
            <div className="px-5 pb-5 space-y-2">
              {[
                { type: 'send_email', status: 'completed', id: 'job_9f2c' },
                { type: 'resize_image', status: 'running', id: 'job_7a1b' },
                { type: 'generate_report', status: 'queued', id: 'job_3d8e' },
              ].map((job) => (
                <div key={job.id} className="flex items-center justify-between bg-[#111118] border border-[#1e1e2e] rounded-lg px-3 py-2">
                  <div className="flex items-center gap-2">
                    <span className={`w-1.5 h-1.5 rounded-full ${job.status === 'completed' ? 'bg-green-400' : job.status === 'running' ? 'bg-amber-400 animate-pulse' : 'bg-[#6b6b8a]'}`} />
                    <span className="text-xs font-mono text-[#e2e2f0]">{job.type}</span>
                  </div>
                  <div className="flex items-center gap-3">
                    <span className="text-[10px] font-mono text-[#6b6b8a]">{job.id}</span>
                    <span className={`text-[10px] font-mono ${job.status === 'completed' ? 'text-green-400' : job.status === 'running' ? 'text-amber-400' : 'text-[#6b6b8a]'}`}>{job.status}</span>
                  </div>
                </div>
              ))}
            </div>
          </motion.div>
        </div>
      </section>

      {/* ── Features ── */}
      <section className="max-w-6xl mx-auto px-6 py-20">
        <div className="text-center mb-12">
          <h2 className="text-2xl sm:text-3xl font-bold text-[#e2e2f0]">Everything you need</h2>
          <p className="mt-3 text-[#6b6b8a]">Built for reliability. Designed for developers.</p>
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {features.map((f, i) => (
            <motion.div
              key={f.title}
              variants={fade}
              initial="hidden"
              whileInView="show"
              viewport={{ once: true }}
              transition={{ duration: 0.4, delay: i * 0.05 }}
              className="bg-[#0d0d14] border border-[#1e1e2e] hover:border-[#7c6af7]/30 rounded-xl p-5 transition-colors group"
            >
              <div className="w-8 h-8 rounded-lg bg-[#7c6af7]/10 flex items-center justify-center mb-4 group-hover:bg-[#7c6af7]/20 transition-colors">
                <svg className="w-4 h-4 text-[#7c6af7]" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
                  <path strokeLinecap="round" strokeLinejoin="round" d={f.icon} />
                </svg>
              </div>
              <h3 className="text-sm font-semibold text-[#e2e2f0] mb-1.5">{f.title}</h3>
              <p className="text-xs text-[#6b6b8a] leading-relaxed">{f.desc}</p>
            </motion.div>
          ))}
        </div>
      </section>

      {/* ── Code snippet ── */}
      <section className="max-w-6xl mx-auto px-6 py-20 border-t border-[#1e1e2e]">
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-12 items-center">
          <div>
            <h2 className="text-2xl sm:text-3xl font-bold text-[#e2e2f0]">Enqueue in seconds</h2>
            <p className="mt-3 text-[#6b6b8a] leading-relaxed">
              A single HTTP request is all it takes. Any language, any framework.
              No SDKs to install, no queuing library to learn.
            </p>
            <ul className="mt-6 space-y-2.5">
              {['JSON payload — send anything', 'Per-job retry limits', 'Named queues for priority routing', 'Idempotency via job IDs'].map((item) => (
                <li key={item} className="flex items-center gap-2 text-sm text-[#6b6b8a]">
                  <span className="text-[#7c6af7] shrink-0">✓</span> {item}
                </li>
              ))}
            </ul>
          </div>
          <div className="rounded-xl border border-[#1e1e2e] bg-[#0d0d14] overflow-hidden">
            <div className="flex items-center gap-1.5 px-4 py-3 border-b border-[#1e1e2e]">
              <span className="w-2.5 h-2.5 rounded-full bg-red-500/50" />
              <span className="w-2.5 h-2.5 rounded-full bg-amber-500/50" />
              <span className="w-2.5 h-2.5 rounded-full bg-green-500/50" />
              <span className="ml-2 text-xs text-[#6b6b8a] font-mono">bash</span>
            </div>
            <pre className="p-5 text-xs font-mono text-[#e2e2f0] overflow-x-auto leading-relaxed">
              <code>{codeExample}</code>
            </pre>
          </div>
        </div>
      </section>

      {/* ── Pricing ── */}
      <section className="max-w-6xl mx-auto px-6 py-20 border-t border-[#1e1e2e]">
        <div className="text-center mb-12">
          <h2 className="text-2xl sm:text-3xl font-bold text-[#e2e2f0]">Simple pricing</h2>
          <p className="mt-3 text-[#6b6b8a]">Start free. Scale when you're ready.</p>
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-5 max-w-4xl mx-auto">
          {plans.map((plan) => (
            <div
              key={plan.name}
              className={`relative rounded-xl p-6 border flex flex-col gap-5 ${plan.highlight ? 'border-[#7c6af7]/50 bg-[#7c6af7]/5' : 'border-[#1e1e2e] bg-[#0d0d14]'}`}
            >
              {plan.highlight && (
                <div className="absolute -top-3 left-1/2 -translate-x-1/2 px-3 py-0.5 rounded-full bg-[#7c6af7] text-[10px] font-semibold text-white">
                  Popular
                </div>
              )}
              <div>
                <div className="text-sm font-semibold text-[#e2e2f0]">{plan.name}</div>
                <div className="flex items-baseline gap-0.5 mt-1">
                  <span className="text-2xl font-bold text-[#e2e2f0]">{plan.price}</span>
                  <span className="text-sm text-[#6b6b8a]">{plan.period}</span>
                </div>
                <div className="text-xs text-[#6b6b8a] mt-0.5">{plan.limit}</div>
              </div>
              <ul className="space-y-2 flex-1">
                {plan.features.map((f) => (
                  <li key={f} className="flex items-center gap-2 text-xs text-[#6b6b8a]">
                    <span className="text-[#7c6af7]">✓</span> {f}
                  </li>
                ))}
              </ul>
              {plan.available ? (
                <button
                  onClick={() => navigate('/auth')}
                  className="w-full py-2 rounded-lg bg-[#7c6af7] hover:bg-[#6a58e0] text-white text-sm font-medium transition-colors"
                >
                  {plan.cta}
                </button>
              ) : (
                <div className="w-full py-2 rounded-lg bg-[#1e1e2e] border border-[#2a2a3e] text-center text-sm font-medium text-[#6b6b8a] cursor-default select-none">
                  Coming Soon
                </div>
              )}
            </div>
          ))}
        </div>
      </section>

      {/* ── CTA ── */}
      <section className="max-w-6xl mx-auto px-6 py-20 border-t border-[#1e1e2e] text-center">
        <h2 className="text-2xl sm:text-3xl font-bold text-[#e2e2f0]">Ready to ship?</h2>
        <p className="mt-3 text-[#6b6b8a] max-w-md mx-auto">
          Free tier, no credit card required. Start processing jobs in under a minute.
        </p>
        <button
          onClick={() => navigate('/auth')}
          className="mt-8 px-8 py-3 rounded-lg bg-[#7c6af7] hover:bg-[#6a58e0] text-white font-medium transition-colors shadow-lg shadow-[#7c6af7]/20"
        >
          Get started free
        </button>
      </section>

      {/* ── Footer ── */}
      <footer className="border-t border-[#1e1e2e] py-8">
        <div className="max-w-6xl mx-auto px-6 flex flex-col sm:flex-row items-center justify-between gap-4 text-xs text-[#6b6b8a]">
          <div className="flex items-center gap-2">
            <div className="w-5 h-5 rounded bg-[#7c6af7] flex items-center justify-center">
              <svg className="w-3 h-3 text-white" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
                <path d="M2 2h5v5H2zM9 2h5v5H9zM2 9h5v5H2zM9 9h5v5H9z" />
              </svg>
            </div>
            <span className="font-mono font-semibold text-[#e2e2f0]">Queuely</span>
            <span className="text-[#3a3a5e]">·</span>
            <span>© {new Date().getFullYear()}</span>
          </div>
          <div className="flex items-center gap-5">
            <a href="https://github.com/dhananjay6561/jobqueue" target="_blank" rel="noopener noreferrer" className="hover:text-[#e2e2f0] transition-colors">GitHub</a>
            <button onClick={() => navigate('/auth')} className="hover:text-[#e2e2f0] transition-colors">Sign in</button>
          </div>
        </div>
      </footer>
    </div>
  )
}
