package queue

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// CronSchedule is a recurring job template that is enqueued on a cron expression.
type CronSchedule struct {
	ID             uuid.UUID       `json:"id"`
	Name           string          `json:"name"`
	JobType        string          `json:"job_type"`
	Payload        json.RawMessage `json:"payload"`
	QueueName      string          `json:"queue_name"`
	Priority       int             `json:"priority"`
	MaxAttempts    int             `json:"max_attempts"`
	CronExpression string          `json:"cron_expression"`
	Enabled        bool            `json:"enabled"`
	LastRunAt      *time.Time      `json:"last_run_at,omitempty"`
	NextRunAt      time.Time       `json:"next_run_at"`
	CreatedAt      time.Time       `json:"created_at"`
}

// ── Minimal cron parser ───────────────────────────────────────────────────────
// Supports standard 5-field cron: "minute hour dom month dow"
// Each field may be: * | number | */step | a-b | a,b,c

// cronField holds the resolved set of valid values for one cron field.
type cronField struct {
	values [60]bool // large enough for any field range
}

func (f *cronField) matches(v int) bool {
	if v < 0 || v >= len(f.values) {
		return false
	}
	return f.values[v]
}

// parseCronField parses one field of a cron expression.
// min/max define the valid range for this field (e.g. 0-59 for minutes).
func parseCronField(expr string, min, max int) (cronField, error) {
	var f cronField
	for _, part := range strings.Split(expr, ",") {
		part = strings.TrimSpace(part)
		if part == "*" {
			for i := min; i <= max; i++ {
				f.values[i] = true
			}
			continue
		}
		// */step
		if strings.HasPrefix(part, "*/") {
			step, err := strconv.Atoi(part[2:])
			if err != nil || step <= 0 {
				return f, fmt.Errorf("invalid step %q", part)
			}
			for i := min; i <= max; i += step {
				f.values[i] = true
			}
			continue
		}
		// range a-b
		if idx := strings.Index(part, "-"); idx >= 0 {
			lo, err1 := strconv.Atoi(part[:idx])
			hi, err2 := strconv.Atoi(part[idx+1:])
			if err1 != nil || err2 != nil || lo > hi {
				return f, fmt.Errorf("invalid range %q", part)
			}
			for i := lo; i <= hi; i++ {
				if i >= min && i <= max {
					f.values[i] = true
				}
			}
			continue
		}
		// exact value
		n, err := strconv.Atoi(part)
		if err != nil || n < min || n > max {
			return f, fmt.Errorf("invalid value %q (range %d-%d)", part, min, max)
		}
		f.values[n] = true
	}
	return f, nil
}

// CronExpr is a parsed 5-field cron expression.
type CronExpr struct {
	minute  cronField
	hour    cronField
	dom     cronField // day of month
	month   cronField
	dow     cronField // day of week (0=Sun)
}

// ParseCron parses a 5-field cron string and returns a CronExpr.
func ParseCron(expr string) (CronExpr, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return CronExpr{}, fmt.Errorf("cron: expected 5 fields, got %d in %q", len(fields), expr)
	}
	var c CronExpr
	var err error
	if c.minute, err = parseCronField(fields[0], 0, 59); err != nil {
		return c, fmt.Errorf("cron minute: %w", err)
	}
	if c.hour, err = parseCronField(fields[1], 0, 23); err != nil {
		return c, fmt.Errorf("cron hour: %w", err)
	}
	if c.dom, err = parseCronField(fields[2], 1, 31); err != nil {
		return c, fmt.Errorf("cron dom: %w", err)
	}
	if c.month, err = parseCronField(fields[3], 1, 12); err != nil {
		return c, fmt.Errorf("cron month: %w", err)
	}
	if c.dow, err = parseCronField(fields[4], 0, 6); err != nil {
		return c, fmt.Errorf("cron dow: %w", err)
	}
	return c, nil
}

// NextAfter returns the next time after t that matches the cron expression.
// It searches up to 4 years ahead before giving up.
func (c CronExpr) NextAfter(t time.Time) (time.Time, error) {
	// Truncate to the next whole minute.
	next := t.Truncate(time.Minute).Add(time.Minute)
	limit := t.Add(4 * 365 * 24 * time.Hour)

	for next.Before(limit) {
		// Advance month if needed.
		if !c.month.matches(int(next.Month())) {
			// Jump to the 1st of the next month.
			next = time.Date(next.Year(), next.Month()+1, 1, 0, 0, 0, 0, next.Location())
			continue
		}
		// Advance day if needed (dom OR dow match, standard cron semantics).
		domOK := c.dom.matches(next.Day())
		dowOK := c.dow.matches(int(next.Weekday()))
		if !domOK && !dowOK {
			next = time.Date(next.Year(), next.Month(), next.Day()+1, 0, 0, 0, 0, next.Location())
			continue
		}
		if !c.hour.matches(next.Hour()) {
			next = time.Date(next.Year(), next.Month(), next.Day(), next.Hour()+1, 0, 0, 0, next.Location())
			continue
		}
		if !c.minute.matches(next.Minute()) {
			next = next.Add(time.Minute)
			continue
		}
		return next, nil
	}
	return time.Time{}, fmt.Errorf("cron: no next time found within 4 years for %q", "expr")
}
