package repository

import (
	"finance-chat/model"

	"gorm.io/gorm"
)

type BehaviorProfileRepository interface {
	Save(p *model.BehaviorProfile) error
	FindLatestByUserID(userID string) (*model.BehaviorProfile, error)
}

type behaviorProfileRepository struct {
	db *gorm.DB
}

func NewBehaviorProfileRepository(db *gorm.DB) BehaviorProfileRepository {
	return &behaviorProfileRepository{db: db}
}

func (r *behaviorProfileRepository) Save(p *model.BehaviorProfile) error {
	p.UserID = model.UserIDOrDefault(p.UserID)
	return r.db.Create(p).Error
}

func (r *behaviorProfileRepository) FindLatestByUserID(userID string) (*model.BehaviorProfile, error) {
	var profile model.BehaviorProfile
	err := r.db.Where("user_id = ?", model.UserIDOrDefault(userID)).Order("computed_date desc").First(&profile).Error
	if err != nil {
		return nil, err
	}
	return &profile, nil
}
