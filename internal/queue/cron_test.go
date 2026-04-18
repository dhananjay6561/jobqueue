package queue

import (
	"testing"
	"time"
)

func TestParseCron_NextAfter(t *testing.T) {
	loc := time.UTC
	base := time.Date(2026, 4, 18, 10, 0, 0, 0, loc) // Saturday 10:00

	tests := []struct {
		expr    string
		wantMin time.Time
	}{
		// Every minute
		{"* * * * *", base.Add(time.Minute)},
		// Every hour at :30
		{"30 * * * *", time.Date(2026, 4, 18, 10, 30, 0, 0, loc)},
		// Daily at 09:00 — next is tomorrow since base is 10:00
		{"0 9 * * *", time.Date(2026, 4, 19, 9, 0, 0, 0, loc)},
		// Every 15 minutes
		{"*/15 * * * *", time.Date(2026, 4, 18, 10, 15, 0, 0, loc)},
	}

	for _, tt := range tests {
		expr, err := ParseCron(tt.expr)
		if err != nil {
			t.Fatalf("ParseCron(%q): %v", tt.expr, err)
		}
		got, err := expr.NextAfter(base)
		if err != nil {
			t.Fatalf("NextAfter(%q): %v", tt.expr, err)
		}
		if !got.Equal(tt.wantMin) {
			t.Errorf("NextAfter(%q): got %v, want %v", tt.expr, got, tt.wantMin)
		}
	}
}

func TestParseCron_Invalid(t *testing.T) {
	cases := []string{"* * * *", "60 * * * *", "bad", ""}
	for _, c := range cases {
		if _, err := ParseCron(c); err == nil {
			t.Errorf("ParseCron(%q) expected error, got nil", c)
		}
	}
}
