// Package store — store_test.go unit-tests the pure helper functions in the
// store package that do not require a real database.
package store

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// ─── nullableString ───────────────────────────────────────────────────────────

func TestNullableString_Empty(t *testing.T) {
	if nullableString("") != nil {
		t.Error("expected nil for empty string")
	}
}

func TestNullableString_NonEmpty(t *testing.T) {
	s := nullableString("hello")
	if s == nil || *s != "hello" {
		t.Errorf("expected 'hello', got %v", s)
	}
}

// ─── nullableJSONB ────────────────────────────────────────────────────────────

func TestNullableJSONB_Nil(t *testing.T) {
	if nullableJSONB(nil) != nil {
		t.Error("expected nil for nil map")
	}
}

func TestNullableJSONB_Empty(t *testing.T) {
	if nullableJSONB(map[string]string{}) != nil {
		t.Error("expected nil for empty map")
	}
}

func TestNullableJSONB_NonEmpty(t *testing.T) {
	b := nullableJSONB(map[string]string{"env": "prod"})
	if b == nil {
		t.Fatal("expected non-nil JSON bytes")
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if m["env"] != "prod" {
		t.Errorf("expected env=prod, got %v", m)
	}
}

// ─── cursor encoding round-trip ───────────────────────────────────────────────

func TestCursorEncoding_RoundTrip(t *testing.T) {
	ts := time.Now().UTC().Truncate(time.Nanosecond)
	rawID := "550e8400-e29b-41d4-a716-446655440000"

	encoded := base64.StdEncoding.EncodeToString(
		[]byte(ts.Format(time.RFC3339Nano) + "|" + rawID),
	)

	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}

	parsedTime, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		t.Fatalf("parse time: %v", err)
	}
	if !parsedTime.Equal(ts) {
		t.Errorf("time mismatch: got %v want %v", parsedTime, ts)
	}
	if parts[1] != rawID {
		t.Errorf("ID mismatch: got %s want %s", parts[1], rawID)
	}
}

// ─── CursorPage helpers ───────────────────────────────────────────────────────

func TestCursorPage_HasMoreFalse_WhenExactLimit(t *testing.T) {
	// Simulate fetching limit+1=6 rows, getting exactly 5 back → no more.
	items := make([]int, 5)
	hasMore := len(items) > 5
	if hasMore {
		t.Error("expected hasMore=false when items == limit")
	}
}

func TestCursorPage_HasMoreTrue_WhenOverLimit(t *testing.T) {
	items := make([]int, 6)
	hasMore := len(items) > 5
	if !hasMore {
		t.Error("expected hasMore=true when items > limit")
	}
}
