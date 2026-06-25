package repository

import (
	"errors"
	"finance-chat/model"

	"gorm.io/gorm"
)

type QuestRepository interface {
	// FindLatestBatch returns every assignment sharing the most recent
	// BatchID for userID, or nil if the user has never been assigned a batch.
	FindLatestBatch(userID string) ([]model.UserQuestAssignment, error)
	CreateBatch(rows []model.UserQuestAssignment) error
	UpdateAssignment(a *model.UserQuestAssignment) error
	FindGameProfile(userID string) (*model.UserGameProfile, error)
	UpsertGameProfile(p *model.UserGameProfile) error
}

type questRepository struct {
	db *gorm.DB
}

func NewQuestRepository(db *gorm.DB) QuestRepository {
	return &questRepository{db: db}
}

func (r *questRepository) FindLatestBatch(userID string) ([]model.UserQuestAssignment, error) {
	userID = model.UserIDOrDefault(userID)

	var latest model.UserQuestAssignment
	err := r.db.Where("user_id = ?", userID).Order("assigned_at desc").First(&latest).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	var rows []model.UserQuestAssignment
	err = r.db.Where("user_id = ? AND batch_id = ?", userID, latest.BatchID).
		Order("id asc").Find(&rows).Error
	return rows, err
}

func (r *questRepository) CreateBatch(rows []model.UserQuestAssignment) error {
	return r.db.Create(&rows).Error
}

func (r *questRepository) UpdateAssignment(a *model.UserQuestAssignment) error {
	return r.db.Save(a).Error
}

func (r *questRepository) FindGameProfile(userID string) (*model.UserGameProfile, error) {
	userID = model.UserIDOrDefault(userID)

	var p model.UserGameProfile
	err := r.db.Where("user_id = ?", userID).First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &model.UserGameProfile{UserID: userID}, nil
		}
		return nil, err
	}
	return &p, nil
}

func (r *questRepository) UpsertGameProfile(p *model.UserGameProfile) error {
	p.UserID = model.UserIDOrDefault(p.UserID)
	return r.db.Save(p).Error
}
