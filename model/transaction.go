package model

import (
	"strings"
	"time"
)

type TransactionType string

const (
	Income        TransactionType = "income"
	Expense       TransactionType = "expense"
	DefaultUserID                 = "default"
)

// UserIDOrDefault preserves compatibility for non-LINE API callers and for
// rows created before per-user ownership was introduced.
func UserIDOrDefault(userID string) string {
	if userID = strings.TrimSpace(userID); userID != "" {
		return userID
	}
	return DefaultUserID
}

type Transaction struct {
	ID          uint            `json:"id"            gorm:"primaryKey;autoIncrement"`
	UserID      string          `json:"user_id"       gorm:"size:128;not null;default:default;index"`
	RawMessage  string          `json:"raw_message"`
	Type        TransactionType `json:"type"`
	Amount      float64         `json:"amount"`
	Category    string          `json:"category"`     // e.g. Food & Drink, Transport, Health
	SubCategory string          `json:"sub_category"` // e.g. Coffee, Taxi, Gym
	Brand       string          `json:"brand"`        // e.g. Starbucks, Grab, McDonald's
	Description string          `json:"description"`
	BehaviorTag string          `json:"behavior_tag"` // e.g. "impulse", "necessity", "social", "treat", "recurring"
	CreatedAt   time.Time       `json:"created_at"`
	WalletID    *uint           `json:"wallet_id" gorm:"index"`
}
