package model

import "time"

type TransactionType string

const (
	Income  TransactionType = "income"
	Expense TransactionType = "expense"
)

type Transaction struct {
	ID           uint            `json:"id"            gorm:"primaryKey;autoIncrement"`
	RawMessage   string          `json:"raw_message"`
	Type         TransactionType `json:"type"`
	Amount       float64         `json:"amount"`
	Category     string          `json:"category"`     // e.g. Food & Drink, Transport, Health
	SubCategory  string          `json:"sub_category"` // e.g. Coffee, Taxi, Gym
	Brand        string          `json:"brand"`        // e.g. Starbucks, Grab, McDonald's
	Description  string          `json:"description"`
	BehaviorTag  string          `json:"behavior_tag"`  // e.g. "impulse", "necessity", "social", "treat", "recurring"
	CreatedAt    time.Time       `json:"created_at"`
}
