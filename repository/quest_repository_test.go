package repository

import (
	"finance-chat/model"
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newQuestTestRepo(t *testing.T) (*gorm.DB, QuestRepository) {
	t.Helper()

	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.QuestPreset{}, &model.UserQuestAssignment{}, &model.UserGameProfile{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	return db, NewQuestRepository(db)
}

func TestQuestRepository_QuestPresetCRUDArchivesOnDelete(t *testing.T) {
	_, repo := newQuestTestRepo(t)

	preset := model.QuestPreset{
		Key:        "test_daily_log",
		Name:       "Test daily log",
		Logo:       "🧾",
		Period:     model.QuestPeriodDaily,
		Difficulty: model.QuestBasic,
		XP:         10,
		RuleType:   model.QuestRuleLogTransactionCount,
		Target:     1,
		Unit:       "count",
		IsActive:   true,
	}
	if err := repo.CreateQuestPreset(&preset); err != nil {
		t.Fatalf("create preset: %v", err)
	}

	active, err := repo.ListQuestPresets(false)
	if err != nil {
		t.Fatalf("list active presets: %v", err)
	}
	if len(active) != 1 || active[0].Key != preset.Key {
		t.Fatalf("unexpected active presets: %+v", active)
	}

	preset.XP = 25
	if err := repo.UpdateQuestPreset(&preset); err != nil {
		t.Fatalf("update preset: %v", err)
	}
	updated, err := repo.GetQuestPreset(preset.ID)
	if err != nil {
		t.Fatalf("get preset: %v", err)
	}
	if updated.XP != 25 {
		t.Fatalf("expected updated XP 25, got %d", updated.XP)
	}

	if err := repo.DeleteQuestPreset(preset.ID); err != nil {
		t.Fatalf("delete preset: %v", err)
	}
	active, err = repo.ListQuestPresets(false)
	if err != nil {
		t.Fatalf("list active after delete: %v", err)
	}
	if len(active) != 0 {
		t.Fatalf("expected no active presets after archive, got %+v", active)
	}
	all, err := repo.ListQuestPresets(true)
	if err != nil {
		t.Fatalf("list all after delete: %v", err)
	}
	if len(all) != 1 || all[0].IsActive {
		t.Fatalf("expected archived preset in all list, got %+v", all)
	}
}

func TestQuestRepository_FindGameProfileCreatesMissingProfileOnce(t *testing.T) {
	db, repo := newQuestTestRepo(t)

	profile, err := repo.FindGameProfile("line-user-1")
	if err != nil {
		t.Fatalf("find or create game profile: %v", err)
	}
	if profile.ID == 0 {
		t.Fatalf("expected persisted game profile ID, got zero")
	}
	if profile.UserID != "line-user-1" {
		t.Fatalf("expected normalized user profile, got %q", profile.UserID)
	}
	if profile.TotalXP != 0 {
		t.Fatalf("expected new profile XP 0, got %d", profile.TotalXP)
	}

	again, err := repo.FindGameProfile("line-user-1")
	if err != nil {
		t.Fatalf("find existing game profile: %v", err)
	}
	if again.ID != profile.ID {
		t.Fatalf("expected existing profile ID %d, got %d", profile.ID, again.ID)
	}

	var count int64
	if err := db.Model(&model.UserGameProfile{}).Where("user_id = ?", "line-user-1").Count(&count).Error; err != nil {
		t.Fatalf("count game profiles: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one persisted game profile, got %d", count)
	}
}

func TestQuestRepository_FindLatestBatchByPeriodIgnoresExpiredDailyBatch(t *testing.T) {
	_, repo := newQuestTestRepo(t)
	now := time.Date(2026, time.June, 27, 10, 0, 0, 0, time.UTC)

	err := repo.CreateBatch([]model.UserQuestAssignment{
		{
			UserID:      "line-user-1",
			BatchID:     "yesterday",
			Period:      model.QuestPeriodDaily,
			TemplateKey: "daily_log_transaction",
			AssignedAt:  now.AddDate(0, 0, -1),
			ExpiresAt:   now.Add(-time.Hour),
		},
	})
	if err != nil {
		t.Fatalf("create expired batch: %v", err)
	}

	rows, err := repo.FindLatestBatchByPeriod("line-user-1", model.QuestPeriodDaily, now)
	if err != nil {
		t.Fatalf("find latest daily batch: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected expired daily batch to be ignored, got %d rows", len(rows))
	}
}

func TestQuestRepository_FindLatestBatchByPeriodIgnoresBatchWithoutAvailableDates(t *testing.T) {
	_, repo := newQuestTestRepo(t)
	now := time.Date(2026, time.June, 27, 10, 0, 0, 0, time.UTC)

	err := repo.CreateBatch([]model.UserQuestAssignment{
		{
			UserID:      "line-user-1",
			BatchID:     "missing-expiry",
			Period:      model.QuestPeriodDaily,
			TemplateKey: "daily_log_transaction",
			AssignedAt:  now,
		},
		{
			UserID:      "line-user-1",
			BatchID:     "missing-assigned-at",
			Period:      model.QuestPeriodDaily,
			TemplateKey: "daily_no_impulse",
			ExpiresAt:   now.Add(time.Hour),
		},
	})
	if err != nil {
		t.Fatalf("create invalid date batches: %v", err)
	}

	rows, err := repo.FindLatestBatchByPeriod("line-user-1", model.QuestPeriodDaily, now)
	if err != nil {
		t.Fatalf("find latest daily batch: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected date-unavailable batches to be ignored, got %d rows", len(rows))
	}
}

func TestQuestRepository_FindLatestBatchByPeriodSeparatesDailyAndWeekly(t *testing.T) {
	_, repo := newQuestTestRepo(t)
	now := time.Date(2026, time.June, 27, 10, 0, 0, 0, time.UTC)

	err := repo.CreateBatch([]model.UserQuestAssignment{
		{
			UserID:      "line-user-1",
			BatchID:     "daily",
			Period:      model.QuestPeriodDaily,
			TemplateKey: "daily_log_transaction",
			AssignedAt:  now,
			ExpiresAt:   now.Add(12 * time.Hour),
		},
		{
			UserID:      "line-user-1",
			BatchID:     "weekly",
			Period:      model.QuestPeriodWeekly,
			TemplateKey: "log_5",
			AssignedAt:  now,
			ExpiresAt:   now.AddDate(0, 0, 3),
		},
	})
	if err != nil {
		t.Fatalf("create batches: %v", err)
	}

	dailyRows, err := repo.FindLatestBatchByPeriod("line-user-1", model.QuestPeriodDaily, now)
	if err != nil {
		t.Fatalf("find daily batch: %v", err)
	}
	weeklyRows, err := repo.FindLatestBatchByPeriod("line-user-1", model.QuestPeriodWeekly, now)
	if err != nil {
		t.Fatalf("find weekly batch: %v", err)
	}
	if len(dailyRows) != 1 || dailyRows[0].BatchID != "daily" {
		t.Fatalf("unexpected daily rows: %+v", dailyRows)
	}
	if len(weeklyRows) != 1 || weeklyRows[0].BatchID != "weekly" {
		t.Fatalf("unexpected weekly rows: %+v", weeklyRows)
	}
}

func TestQuestRepository_HasBatchForPeriodWindowUsesAssignedAtWindow(t *testing.T) {
	_, repo := newQuestTestRepo(t)
	now := time.Date(2026, time.June, 27, 10, 0, 0, 0, time.UTC)
	start := now.Add(-2 * time.Hour)
	end := now.Add(2 * time.Hour)

	err := repo.CreateBatch([]model.UserQuestAssignment{
		{
			UserID:      "line-user-1",
			BatchID:     "current-window",
			Period:      model.QuestPeriodDaily,
			TemplateKey: "daily_log_transaction",
			AssignedAt:  now,
			ExpiresAt:   now.Add(time.Hour),
		},
		{
			UserID:      "line-user-1",
			BatchID:     "previous-window",
			Period:      model.QuestPeriodDaily,
			TemplateKey: "daily_no_impulse",
			AssignedAt:  now.AddDate(0, 0, -1),
			ExpiresAt:   now.AddDate(0, 0, -1).Add(time.Hour),
		},
	})
	if err != nil {
		t.Fatalf("create batches: %v", err)
	}

	hasBatch, err := repo.HasBatchForPeriodWindow("line-user-1", model.QuestPeriodDaily, start, end)
	if err != nil {
		t.Fatalf("has current window batch: %v", err)
	}
	if !hasBatch {
		t.Fatalf("expected current window batch")
	}

	hasBatch, err = repo.HasBatchForPeriodWindow("line-user-1", model.QuestPeriodWeekly, start, end)
	if err != nil {
		t.Fatalf("has weekly window batch: %v", err)
	}
	if hasBatch {
		t.Fatalf("did not expect weekly window batch")
	}
}
