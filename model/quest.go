package model

import "time"

type QuestDifficulty string

const (
	QuestBasic       QuestDifficulty = "basic"
	QuestAdvanced    QuestDifficulty = "advanced"
	QuestExpert      QuestDifficulty = "expert"
	QuestMaster      QuestDifficulty = "master"
	QuestGrandMaster QuestDifficulty = "grand_master"
	QuestEasy        QuestDifficulty = "easy"
	QuestMedium      QuestDifficulty = "medium"
	QuestHard        QuestDifficulty = "hard"
)

// QuestTemplateKey identifies one of the preset quest definitions in
// service/quest_templates.go. Progress for every key is computed live from
// real transactions on every read — nothing about a quest's progress is
// stored, only which template it is and whether it has already been
// completed/paid out.
type QuestTemplateKey string

type QuestPeriod string

const (
	QuestPeriodDaily  QuestPeriod = "daily"
	QuestPeriodWeekly QuestPeriod = "weekly"
)

type QuestRuleType string

const (
	QuestRuleLogTransactionCount      QuestRuleType = "log_transaction_count"
	QuestRuleSpendCap                 QuestRuleType = "spend_cap"
	QuestRuleNoImpulse                QuestRuleType = "no_impulse"
	QuestRuleNoSpendDays              QuestRuleType = "no_spend_days"
	QuestRuleCategorySpendCap         QuestRuleType = "category_spend_cap"
	QuestRuleCoffeeSpendCap           QuestRuleType = "coffee_spend_cap"
	QuestRuleBeatLastMonthSavingsRate QuestRuleType = "beat_last_month_savings_rate"
)

// QuestPreset is an editable DB-backed quest template. User-specific quest
// state still lives in UserQuestAssignment; this table only defines the
// reusable preset pool that can be randomly assigned.
type QuestPreset struct {
	ID          uint             `json:"id"           gorm:"primaryKey;autoIncrement"`
	Key         QuestTemplateKey `json:"key"          gorm:"size:96;not null;uniqueIndex"`
	Name        string           `json:"name"         gorm:"size:160;not null"`
	Logo        string           `json:"logo"         gorm:"size:32"`
	Period      QuestPeriod      `json:"period"       gorm:"size:16;not null;index"`
	Difficulty  QuestDifficulty  `json:"difficulty"   gorm:"size:32;not null"`
	XP          int              `json:"xp"           gorm:"not null"`
	Accent      string           `json:"accent"       gorm:"size:32"`
	RuleType    QuestRuleType    `json:"rule_type"    gorm:"size:64;not null"`
	Target      float64          `json:"target"`
	Unit        string           `json:"unit"         gorm:"size:32"`
	Category    string           `json:"category"     gorm:"size:96"`
	SubCategory string           `json:"sub_category" gorm:"size:96"`
	BehaviorTag string           `json:"behavior_tag" gorm:"size:64"`
	IsActive    bool             `json:"is_active"    gorm:"not null;default:true;index"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

// UserQuestAssignment is one quest assigned to a user as part of a batch.
// A user always holds one fixed-size *batch* of assignments (sharing the
// same BatchID); a fresh batch can only be requested once every assignment
// in the current batch is completed.
type UserQuestAssignment struct {
	ID          uint             `json:"id"           gorm:"primaryKey;autoIncrement"`
	UserID      string           `json:"user_id"      gorm:"size:128;not null;default:default;index"`
	BatchID     string           `json:"batch_id"     gorm:"size:64;index"`
	Period      QuestPeriod      `json:"period"       gorm:"size:16;not null;default:weekly;index"`
	TemplateKey QuestTemplateKey `json:"template_key"`
	AssignedAt  time.Time        `json:"assigned_at"`
	ExpiresAt   time.Time        `json:"expires_at"   gorm:"index"`
	CompletedAt *time.Time       `json:"completed_at"`
	XPAwarded   bool             `json:"xp_awarded"`
}

// UserGameProfile accumulates XP earned from completed quests. Level is
// derived from TotalXP at read time, never stored directly, so the level
// curve can change later without a migration.
//
// ID is an autoincrement surrogate key (not UserID) so that repository.Save()
// correctly distinguishes "new profile" (ID zero -> INSERT) from "existing
// profile" (ID set -> UPDATE) — matching the UserPlan upsert convention.
type UserGameProfile struct {
	ID        uint      `json:"id"         gorm:"primaryKey;autoIncrement"`
	UserID    string    `json:"user_id"    gorm:"size:128;not null;default:default;uniqueIndex"`
	TotalXP   int       `json:"total_xp"`
	UpdatedAt time.Time `json:"updated_at"`
}
