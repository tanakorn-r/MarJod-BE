package repository

import (
	"errors"
	"finance-chat/model"
	"time"

	"gorm.io/gorm"
)

type UserPlanRepository interface {
	FindByUserID(userID string) (*model.UserPlan, error)
	Upsert(p *model.UserPlan) error
}

type userPlanRepository struct {
	db *gorm.DB
}

func NewUserPlanRepository(db *gorm.DB) UserPlanRepository {
	return &userPlanRepository{db: db}
}

// FindByUserID returns the user's plan. If no row exists, it returns a default
// free plan with an expiry far in the future so EffectivePlan() returns PlanFree.
func (r *userPlanRepository) FindByUserID(userID string) (*model.UserPlan, error) {
	var plan model.UserPlan
	err := r.db.Where("user_id = ?", userID).First(&plan).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &model.UserPlan{
				UserID:     userID,
				Plan:       model.PlanFree,
				ExpiryDate: time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC),
			}, nil
		}
		return nil, err
	}
	return &plan, nil
}

// Upsert inserts or updates the user's plan record. GORM's Save handles both
// cases; the uniqueIndex on user_id ensures only one row per user.
func (r *userPlanRepository) Upsert(p *model.UserPlan) error {
	return r.db.Save(p).Error
}
