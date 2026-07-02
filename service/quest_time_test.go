package service

import (
	"finance-chat/model"
	"testing"
	"time"
)

func TestQuestExpiryUsesThailandDayBoundary(t *testing.T) {
	now := time.Date(2026, time.June, 28, 23, 30, 0, 0, time.FixedZone("UTC", 0))

	expiry := questExpiry(model.QuestPeriodDaily, now)
	if expiry.Location().String() != "Asia/Bangkok" {
		t.Fatalf("expiry location: want Asia/Bangkok, got %s", expiry.Location())
	}
	if got := expiry.Format(time.RFC3339); got != "2026-06-29T00:00:00+07:00" {
		t.Fatalf("daily expiry: want 2026-06-29T00:00:00+07:00, got %s", got)
	}
}

func TestQuestExpiryUsesThailandWeekBoundary(t *testing.T) {
	now := time.Date(2026, time.June, 24, 10, 0, 0, 0, time.FixedZone("UTC", 0))

	expiry := questExpiry(model.QuestPeriodWeekly, now)
	if expiry.Location().String() != "Asia/Bangkok" {
		t.Fatalf("expiry location: want Asia/Bangkok, got %s", expiry.Location())
	}
	if got := expiry.Format(time.RFC3339); got != "2026-06-29T00:00:00+07:00" {
		t.Fatalf("weekly expiry: want 2026-06-29T00:00:00+07:00, got %s", got)
	}
}
