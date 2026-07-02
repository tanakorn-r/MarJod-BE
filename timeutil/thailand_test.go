package timeutil

import (
	"testing"
	"time"
)

func TestStartOfDayUsesThailandDate(t *testing.T) {
	utcLate := time.Date(2026, time.June, 27, 18, 30, 0, 0, time.UTC)

	start := StartOfDay(utcLate)
	if got := start.Format(time.RFC3339); got != "2026-06-28T00:00:00+07:00" {
		t.Fatalf("Thailand start of day: want 2026-06-28T00:00:00+07:00, got %s", got)
	}
}

func TestParseMonthUsesThailandLocation(t *testing.T) {
	month, err := ParseMonth("2026-06")
	if err != nil {
		t.Fatalf("parse month: %v", err)
	}
	if got := month.Format(time.RFC3339); got != "2026-06-01T00:00:00+07:00" {
		t.Fatalf("Thailand month: want 2026-06-01T00:00:00+07:00, got %s", got)
	}
}
