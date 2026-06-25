package repository

import (
	"finance-chat/model"

	"gorm.io/gorm"
)

type CorrectionRepository interface {
	Save(c *model.UserCorrection) error
	FindRecent(userID string, limit int) ([]model.UserCorrection, error)
	FindAll(userID string) ([]model.UserCorrection, error)
	Delete(userID string, id uint) error
}

type correctionRepository struct {
	db *gorm.DB
}

func NewCorrectionRepository(db *gorm.DB) CorrectionRepository {
	return &correctionRepository{db: db}
}

func (r *correctionRepository) Save(c *model.UserCorrection) error {
	c.UserID = model.UserIDOrDefault(c.UserID)
	return r.db.Create(c).Error
}

func (r *correctionRepository) FindRecent(userID string, limit int) ([]model.UserCorrection, error) {
	var list []model.UserCorrection
	err := r.db.Where("user_id = ?", model.UserIDOrDefault(userID)).
		Order("created_at desc").Limit(limit).Find(&list).Error
	return list, err
}

func (r *correctionRepository) FindAll(userID string) ([]model.UserCorrection, error) {
	var list []model.UserCorrection
	err := r.db.Where("user_id = ?", model.UserIDOrDefault(userID)).
		Order("created_at desc").Find(&list).Error
	return list, err
}

func (r *correctionRepository) Delete(userID string, id uint) error {
	return r.db.Where("user_id = ?", model.UserIDOrDefault(userID)).Delete(&model.UserCorrection{}, id).Error
}
