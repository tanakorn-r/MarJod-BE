package database

import (
	"finance-chat/model"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMigratePreservesLegacyTransactionsAndBackfillsUserID(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open legacy database: %v", err)
	}

	legacySchema := `CREATE TABLE transactions (
		id integer PRIMARY KEY AUTOINCREMENT,
		raw_message text,
		type text,
		amount real,
		category text,
		sub_category text,
		brand text,
		description text,
		behavior_tag text,
		created_at datetime
	)`
	if err := db.Exec(legacySchema).Error; err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}
	if err := db.Exec(`INSERT INTO transactions
		(raw_message, type, amount, category, description, created_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		"legacy coffee", "expense", 120, "Food", "Coffee").Error; err != nil {
		t.Fatalf("insert legacy transaction: %v", err)
	}

	Migrate(db)

	var transactions []model.Transaction
	if err := db.Find(&transactions).Error; err != nil {
		t.Fatalf("read migrated transactions: %v", err)
	}
	if len(transactions) != 1 {
		t.Fatalf("migration changed row count: want 1, got %d", len(transactions))
	}
	got := transactions[0]
	if got.RawMessage != "legacy coffee" || got.Amount != 120 {
		t.Fatalf("legacy data changed during migration: %+v", got)
	}
	if got.UserID != model.DefaultUserID {
		t.Fatalf("legacy user_id: want %q, got %q", model.DefaultUserID, got.UserID)
	}
}

func TestMigrateSeedsQuestPresetsWithoutOverwritingExistingRows(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "quest-presets.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}

	Migrate(db)

	var count int64
	if err := db.Model(&model.QuestPreset{}).Count(&count).Error; err != nil {
		t.Fatalf("count quest presets: %v", err)
	}
	if want := int64(len(model.DefaultQuestPresets())); count != want {
		t.Fatalf("quest preset count: want %d, got %d", want, count)
	}

	var preset model.QuestPreset
	if err := db.Where("key = ?", model.QuestTemplateKey("daily_log_transaction")).First(&preset).Error; err != nil {
		t.Fatalf("find seeded preset: %v", err)
	}
	preset.Name = "Custom admin title"
	if err := db.Save(&preset).Error; err != nil {
		t.Fatalf("customize preset: %v", err)
	}

	Migrate(db)

	var after model.QuestPreset
	if err := db.Where("key = ?", model.QuestTemplateKey("daily_log_transaction")).First(&after).Error; err != nil {
		t.Fatalf("find preset after second migrate: %v", err)
	}
	if after.Name != "Custom admin title" {
		t.Fatalf("migration overwrote customized preset name: %q", after.Name)
	}
}
