package repository

import (
	"finance-chat/model"

	"gorm.io/gorm"
)

type CorrectionRepository interface {
	Save(c *model.UserCorrection) error
	FindRecent(limit int) ([]model.UserCorrection, error)
	FindAll() ([]model.UserCorrection, error)
	Delete(id uint) error
}

type correctionRepository struct {
	db *gorm.DB
}

func NewCorrectionRepository(db *gorm.DB) CorrectionRepository {
	return &correctionRepository{db: db}
}

func (r *correctionRepository) Save(c *model.UserCorrection) error {
	return r.db.Create(c).Error
}

func (r *correctionRepository) FindRecent(limit int) ([]model.UserCorrection, error) {
	var list []model.UserCorrection
	err := r.db.Order("created_at desc").Limit(limit).Find(&list).Error
	return list, err
}

func (r *correctionRepository) FindAll() ([]model.UserCorrection, error) {
	var list []model.UserCorrection
	err := r.db.Order("created_at desc").Find(&list).Error
	return list, err
}

func (r *correctionRepository) Delete(id uint) error {
	return r.db.Delete(&model.UserCorrection{}, id).Error
}
