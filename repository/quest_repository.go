package repository

import (
	"errors"
	"finance-chat/model"
	"finance-chat/timeutil"
	"time"

	"gorm.io/gorm"
)

type QuestRepository interface {
	ListQuestPresets(includeInactive bool) ([]model.QuestPreset, error)
	GetQuestPreset(id uint) (*model.QuestPreset, error)
	CreateQuestPreset(preset *model.QuestPreset) error
	UpdateQuestPreset(preset *model.QuestPreset) error
	DeleteQuestPreset(id uint) error
	SeedQuestPresets(presets []model.QuestPreset) error
	// FindLatestBatch returns every assignment sharing the most recent
	// BatchID for userID, or nil if the user has never been assigned a batch.
	FindLatestBatch(userID string) ([]model.UserQuestAssignment, error)
	FindLatestBatchByPeriod(userID string, period model.QuestPeriod, now time.Time) ([]model.UserQuestAssignment, error)
	HasBatchForPeriodWindow(userID string, period model.QuestPeriod, start time.Time, end time.Time) (bool, error)
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

func (r *questRepository) ListQuestPresets(includeInactive bool) ([]model.QuestPreset, error) {
	var presets []model.QuestPreset
	query := r.db.Order("period asc, difficulty asc, xp asc, id asc")
	if !includeInactive {
		query = query.Where("is_active = ?", true)
	}
	err := query.Find(&presets).Error
	return presets, err
}

func (r *questRepository) GetQuestPreset(id uint) (*model.QuestPreset, error) {
	var preset model.QuestPreset
	if err := r.db.First(&preset, id).Error; err != nil {
		return nil, err
	}
	return &preset, nil
}

func (r *questRepository) CreateQuestPreset(preset *model.QuestPreset) error {
	return r.db.Create(preset).Error
}

func (r *questRepository) UpdateQuestPreset(preset *model.QuestPreset) error {
	return r.db.Save(preset).Error
}

func (r *questRepository) DeleteQuestPreset(id uint) error {
	result := r.db.Model(&model.QuestPreset{}).Where("id = ?", id).Update("is_active", false)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *questRepository) SeedQuestPresets(presets []model.QuestPreset) error {
	for i := range presets {
		preset := presets[i]
		var existing model.QuestPreset
		err := r.db.Where("key = ?", preset.Key).First(&existing).Error
		if err == nil {
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := r.db.Create(&preset).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *questRepository) FindLatestBatch(userID string) ([]model.UserQuestAssignment, error) {
	return r.FindLatestBatchByPeriod(userID, model.QuestPeriodWeekly, timeutil.Now())
}

func (r *questRepository) FindLatestBatchByPeriod(userID string, period model.QuestPeriod, now time.Time) ([]model.UserQuestAssignment, error) {
	userID = model.UserIDOrDefault(userID)
	if period == "" {
		period = model.QuestPeriodWeekly
	}

	var latest model.UserQuestAssignment
	err := r.db.
		Where("user_id = ? AND period = ? AND assigned_at <> ? AND expires_at > ?", userID, period, time.Time{}, now).
		Order("assigned_at desc").
		First(&latest).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	var rows []model.UserQuestAssignment
	err = r.db.Where("user_id = ? AND period = ? AND batch_id = ? AND assigned_at <> ? AND expires_at > ?", userID, period, latest.BatchID, time.Time{}, now).
		Order("id asc").Find(&rows).Error
	return rows, err
}

func (r *questRepository) HasBatchForPeriodWindow(userID string, period model.QuestPeriod, start time.Time, end time.Time) (bool, error) {
	userID = model.UserIDOrDefault(userID)
	if period == "" {
		period = model.QuestPeriodWeekly
	}

	var count int64
	err := r.db.Model(&model.UserQuestAssignment{}).
		Where("user_id = ? AND period = ? AND assigned_at >= ? AND assigned_at < ?", userID, period, start, end).
		Count(&count).Error
	return count > 0, err
}

func (r *questRepository) CreateBatch(rows []model.UserQuestAssignment) error {
	return r.db.Create(&rows).Error
}

func (r *questRepository) UpdateAssignment(a *model.UserQuestAssignment) error {
	return r.db.Save(a).Error
}

func (r *questRepository) FindGameProfile(userID string) (*model.UserGameProfile, error) {
	userID = model.UserIDOrDefault(userID)

	p := model.UserGameProfile{UserID: userID}
	if err := r.db.Where("user_id = ?", userID).FirstOrCreate(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *questRepository) UpsertGameProfile(p *model.UserGameProfile) error {
	p.UserID = model.UserIDOrDefault(p.UserID)
	return r.db.Save(p).Error
}
