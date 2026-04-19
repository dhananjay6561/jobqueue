import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { motion } from 'framer-motion'
import { useQuery } from '@tanstack/react-query'
import { createCheckout, createCustomerPortal } from '@/api/billing'
import { portalClient } from '@/api/billing'
import { useAuthStore } from '@/store/authStore'
import { useUiStore } from '@/store/uiStore'

interface UsageData {
  tier: string
  jobs_used: number
  jobs_limit: number
  usage_percent: number
  limit_reached: boolean
  reset_at: string
}

const TIER_LABELS: Record<string, { label: string; color: string; limit: string }> = {
  free:     { label: 'Free',     color: 'text-[#6b6b8a]', limit: '1,000 jobs/mo' },
  pro:      { label: 'Pro',      color: 'text-[#7c6af7]', limit: '100,000 jobs/mo' },
  business: { label: 'Business', color: 'text-green-400',  limit: 'Unlimited' },
}

const PLANS = [
  {
    tier: 'pro' as const,
    name: 'Pro',
    price: '$29/mo',
    limit: '100,000 jobs/month',
    features: ['Priority queue', 'Webhook support', 'Email support'],
  },
  {
    tier: 'business' as const,
    name: 'Business',
    price: '$99/mo',
    limit: 'Unlimited jobs',
    features: ['Everything in Pro', 'Dedicated support', 'SLA guarantee'],
  },
]

export function Billing() {
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const { user } = useAuthStore()
  const { addToast } = useUiStore()
  const [upgrading, setUpgrading] = useState<string | null>(null)
  const [openingPortal, setOpeningPortal] = useState(false)

  const { data: usage, isLoading } = useQuery<UsageData>({
    queryKey: ['portal-usage'],
    queryFn: () => portalClient.get('/portal/usage').then((r) => r.data.data),
  })

  const success = params.get('success') === 'true'

  async function handleUpgrade(tier: 'pro' | 'business') {
    if (!user) { navigate('/auth'); return }
    setUpgrading(tier)
    try {
      const url = await createCheckout(tier)
      window.location.href = url
    } catch (err: unknown) {
      addToast({ variant: 'error', message: err instanceof Error ? err.message : 'Checkout failed' })
      setUpgrading(null)
    }
  }

  async function handlePortal() {
    setOpeningPortal(true)
    try {
      const url = await createCustomerPortal()
      window.location.href = url
    } catch (err: unknown) {
      addToast({ variant: 'error', message: err instanceof Error ? err.message : 'Could not open portal' })
      setOpeningPortal(false)
    }
  }

  const tierInfo = TIER_LABELS[usage?.tier ?? 'free']

  return (
    <div className="p-6 space-y-6 max-w-3xl">
      <div>
        <h1 className="text-lg font-semibold text-[#e2e2f0]">Billing & Usage</h1>
        <p className="text-sm text-[#6b6b8a] mt-0.5">Manage your plan and track job usage.</p>
      </div>

      {success && (
        <motion.div
          initial={{ opacity: 0, y: -8 }}
          animate={{ opacity: 1, y: 0 }}
          className="bg-green-500/10 border border-green-500/30 rounded-lg px-4 py-3 text-sm text-green-400"
        >
          Payment successful — your plan has been upgraded.
        </motion.div>
      )}

      {/* Current usage */}
      <div className="bg-[#111118] border border-[#1e1e2e] rounded-xl p-5 space-y-4">
        <div className="flex items-center justify-between">
          <span className="text-xs text-[#6b6b8a] uppercase tracking-widest font-mono">Current plan</span>
          <span className={`text-sm font-semibold ${tierInfo?.color}`}>{tierInfo?.label}</span>
        </div>

        {isLoading ? (
          <div className="h-10 bg-[#1e1e2e] rounded animate-pulse" />
        ) : usage ? (
          <>
            <div className="space-y-2">
              <div className="flex justify-between text-sm">
                <span className="text-[#6b6b8a]">Jobs this month</span>
                <span className="text-[#e2e2f0] font-mono">
                  {usage.jobs_used.toLocaleString()}
                  {usage.jobs_limit > 0 ? ` / ${usage.jobs_limit.toLocaleString()}` : ' / ∞'}
                </span>
              </div>
              {usage.jobs_limit > 0 && (
                <div className="h-1.5 bg-[#1e1e2e] rounded-full overflow-hidden">
                  <div
                    className={`h-full rounded-full transition-all ${usage.usage_percent > 90 ? 'bg-red-500' : usage.usage_percent > 70 ? 'bg-amber-500' : 'bg-[#7c6af7]'}`}
                    style={{ width: `${Math.min(usage.usage_percent, 100)}%` }}
                  />
                </div>
              )}
            </div>
            <div className="flex items-center justify-between text-xs text-[#6b6b8a]">
              <span>Resets {new Date(usage.reset_at).toLocaleDateString()}</span>
              {usage.limit_reached && (
                <span className="text-red-400 font-medium">Limit reached</span>
              )}
            </div>
          </>
        ) : null}

        {usage?.tier !== 'free' && (
          <button
            onClick={handlePortal}
            disabled={openingPortal}
            className="text-xs text-[#7c6af7] hover:underline disabled:opacity-50"
          >
            {openingPortal ? 'Opening portal…' : 'Manage subscription →'}
          </button>
        )}
      </div>

      {/* Upgrade plans */}
      {usage?.tier === 'free' && (
        <div className="space-y-3">
          <h2 className="text-sm font-medium text-[#e2e2f0]">Upgrade your plan</h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            {PLANS.map((plan) => (
              <div
                key={plan.tier}
                className="bg-[#111118] border border-[#1e1e2e] hover:border-[#7c6af7]/40 rounded-xl p-5 space-y-4 transition-colors"
              >
                <div>
                  <div className="flex items-baseline justify-between">
                    <span className="font-semibold text-[#e2e2f0]">{plan.name}</span>
                    <span className="text-lg font-bold text-[#7c6af7]">{plan.price}</span>
                  </div>
                  <p className="text-xs text-[#6b6b8a] mt-0.5">{plan.limit}</p>
                </div>
                <ul className="space-y-1.5">
                  {plan.features.map((f) => (
                    <li key={f} className="flex items-center gap-2 text-xs text-[#6b6b8a]">
                      <span className="text-[#7c6af7]">✓</span> {f}
                    </li>
                  ))}
                </ul>
                <button
                  onClick={() => handleUpgrade(plan.tier)}
                  disabled={upgrading === plan.tier}
                  className="w-full py-2 rounded-lg bg-[#7c6af7] hover:bg-[#6a58e0] disabled:opacity-50 text-white text-sm font-medium transition-colors"
                >
                  {upgrading === plan.tier ? 'Redirecting…' : `Upgrade to ${plan.name}`}
                </button>
              </div>
            ))}
          </div>
        </div>
      )}

      {!user && (
        <div className="text-center py-8">
          <p className="text-sm text-[#6b6b8a] mb-3">Sign in to manage your billing.</p>
          <button
            onClick={() => navigate('/auth')}
            className="px-4 py-2 rounded-lg bg-[#7c6af7] text-white text-sm"
          >
            Sign in
          </button>
        </div>
      )}
    </div>
  )
}
