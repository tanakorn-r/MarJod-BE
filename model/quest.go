package model

import "time"

type QuestDifficulty string

const (
	QuestEasy   QuestDifficulty = "easy"
	QuestMedium QuestDifficulty = "medium"
	QuestHard   QuestDifficulty = "hard"
)

// QuestTemplateKey identifies one of the preset quest definitions in
// service/quest_templates.go. Progress for every key is computed live from
// real transactions on every read — nothing about a quest's progress is
// stored, only which template it is and whether it has already been
// completed/paid out.
type QuestTemplateKey string

// UserQuestAssignment is one quest assigned to a user as part of a batch.
// A user always holds one fixed-size *batch* of assignments (sharing the
// same BatchID); a fresh batch can only be requested once every assignment
// in the current batch is completed.
type UserQuestAssignment struct {
	ID          uint             `json:"id"           gorm:"primaryKey;autoIncrement"`
	UserID      string           `json:"user_id"      gorm:"size:128;not null;default:default;index"`
	BatchID     string           `json:"batch_id"     gorm:"size:64;index"`
	TemplateKey QuestTemplateKey `json:"template_key"`
	AssignedAt  time.Time        `json:"assigned_at"`
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
