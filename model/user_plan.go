package model

import (
	"finance-chat/timeutil"
	"time"
)

type PlanName string

const (
	PlanFree    PlanName = "free"
	PlanStarter PlanName = "starter"
	PlanPro     PlanName = "pro"
)

// UserPlan persists the user's active subscription.
type UserPlan struct {
	ID         uint      `json:"id"          gorm:"primaryKey;autoIncrement"`
	UserID     string    `json:"user_id"     gorm:"size:128;not null;default:default;uniqueIndex"`
	Plan       PlanName  `json:"plan"`
	StartDate  time.Time `json:"start_date"`
	ExpiryDate time.Time `json:"expiry_date"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// IsExpired returns true if the plan's expiry date is in the past.
func (p *UserPlan) IsExpired() bool {
	return timeutil.Now().After(p.ExpiryDate)
}

// EffectivePlan returns the plan name, falling back to free if expired.
func (p *UserPlan) EffectivePlan() PlanName {
	if p.IsExpired() {
		return PlanFree
	}
	return p.Plan
}
