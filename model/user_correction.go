package model

import "time"

// UserCorrection stores manual fixes made by the user.
// These are fed back into the AI prompt as few-shot examples
// so the model learns the user's preferences over time.
type UserCorrection struct {
	ID          uint      `json:"id"           gorm:"primaryKey;autoIncrement"`
	UserID      string    `json:"user_id"      gorm:"size:128;not null;default:default;index"`
	RawMessage  string    `json:"raw_message"` // original message that was misclassified
	Category    string    `json:"category"`
	SubCategory string    `json:"sub_category"`
	Brand       string    `json:"brand"`
	BehaviorTag string    `json:"behavior_tag"`
	CreatedAt   time.Time `json:"created_at"`
}
