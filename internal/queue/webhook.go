package queue

import (
	"time"

	"github.com/google/uuid"
)

// Webhook is a registered HTTP endpoint that receives job event notifications.
type Webhook struct {
	ID        uuid.UUID `json:"id"`
	URL       string    `json:"url"`
	Secret    string    `json:"secret,omitempty"`
	Events    []string  `json:"events"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// WebhookPayload is the body POSTed to a registered endpoint on each event.
type WebhookPayload struct {
	Event     string `json:"event"`
	JobID     string `json:"job_id"`
	JobType   string `json:"job_type"`
	Timestamp int64  `json:"timestamp"`
	Error     string `json:"error,omitempty"`
}
