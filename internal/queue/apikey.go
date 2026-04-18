package queue

import (
	"time"

	"github.com/google/uuid"
)

// APIKeyTier represents the subscription tier of an API key.
type APIKeyTier string

const (
	TierFree     APIKeyTier = "free"
	TierPro      APIKeyTier = "pro"
	TierBusiness APIKeyTier = "business"
)

// TierLimits maps each tier to its monthly job limit (-1 = unlimited).
var TierLimits = map[APIKeyTier]int64{
	TierFree:     1_000,
	TierPro:      100_000,
	TierBusiness: -1,
}

// APIKey is a hashed API key with usage tracking.
type APIKey struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	KeyPrefix string     `json:"key_prefix"` // first 8 chars, safe to display
	Tier      APIKeyTier `json:"tier"`
	JobsUsed  int64      `json:"jobs_used"`
	JobsLimit int64      `json:"jobs_limit"` // -1 = unlimited
	ResetAt   time.Time  `json:"reset_at"`
	Enabled   bool       `json:"enabled"`
	CreatedAt time.Time  `json:"created_at"`
}

// LimitReached returns true when the key has hit its monthly job limit.
func (k *APIKey) LimitReached() bool {
	if k.JobsLimit == -1 {
		return false // unlimited
	}
	return k.JobsUsed >= k.JobsLimit
}

// UsagePercent returns usage as a 0-100 percentage (capped at 100).
func (k *APIKey) UsagePercent() float64 {
	if k.JobsLimit <= 0 {
		return 0
	}
	pct := float64(k.JobsUsed) / float64(k.JobsLimit) * 100
	if pct > 100 {
		return 100
	}
	return pct
}
