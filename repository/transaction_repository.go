package repository

import (
	"finance-chat/model"
	"finance-chat/timeutil"

	"gorm.io/gorm"
)

type TransactionRepository interface {
	Create(t *model.Transaction) error
	FindAllByUserID(userID string) ([]model.Transaction, error)
	FindByID(userID string, id uint) (*model.Transaction, error)
	FindLatestByUserID(userID string) (*model.Transaction, error)
	FindTodayByUserID(userID string) ([]model.Transaction, error)
	FindByCategory(userID, category, month string, page, limit int) ([]model.Transaction, int64, error)
	Update(t *model.Transaction) error
	Delete(userID string, id uint) error
}

type transactionRepository struct {
	db *gorm.DB
}

func NewTransactionRepository(db *gorm.DB) TransactionRepository {
	return &transactionRepository{db: db}
}

func (r *transactionRepository) Create(t *model.Transaction) error {
	t.UserID = model.UserIDOrDefault(t.UserID)
	if t.CreatedAt.IsZero() {
		t.CreatedAt = timeutil.Now()
	} else {
		t.CreatedAt = timeutil.InThailand(t.CreatedAt)
	}
	if t.WalletID != nil && *t.WalletID == model.GeneralWalletID {
		zero := uint(0)
		t.WalletID = &zero
	}
	return r.db.Create(t).Error
}

func (r *transactionRepository) FindAllByUserID(userID string) ([]model.Transaction, error) {
	var list []model.Transaction
	err := r.db.Where("user_id = ?", model.UserIDOrDefault(userID)).
		Order("created_at desc").Find(&list).Error
	return list, err
}

func (r *transactionRepository) FindByID(userID string, id uint) (*model.Transaction, error) {
	var t model.Transaction
	err := r.db.Where("user_id = ?", model.UserIDOrDefault(userID)).First(&t, id).Error
	return &t, err
}

func (r *transactionRepository) FindLatestByUserID(userID string) (*model.Transaction, error) {
	var t model.Transaction
	err := r.db.Where("user_id = ?", model.UserIDOrDefault(userID)).
		Order("created_at desc").First(&t).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *transactionRepository) FindTodayByUserID(userID string) ([]model.Transaction, error) {
	var list []model.Transaction
	now := timeutil.Now()
	start := timeutil.StartOfDay(now)
	end := timeutil.StartOfNextDay(now)
	err := r.db.Where("user_id = ? AND created_at >= ? AND created_at < ?", model.UserIDOrDefault(userID), start, end).
		Order("created_at desc").Find(&list).Error
	return list, err
}

func (r *transactionRepository) FindByCategory(userID, category, month string, page, limit int) ([]model.Transaction, int64, error) {
	var list []model.Transaction
	var total int64

	query := r.db.Model(&model.Transaction{}).Where("user_id = ?", model.UserIDOrDefault(userID))

	if category != "" {
		query = query.Where("category = ?", category)
	}

	// Filter by month if provided (format: "2006-01")
	if month != "" {
		t, err := timeutil.ParseMonth(month)
		if err == nil {
			start := timeutil.StartOfMonth(t)
			end := timeutil.StartOfNextMonth(t)
			query = query.Where("created_at >= ? AND created_at < ?", start, end)
		}
	}

	// Count total matching rows
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Paginate, sorted by most recent first
	offset := (page - 1) * limit
	err := query.Order("created_at desc").Offset(offset).Limit(limit).Find(&list).Error
	return list, total, err
}

func (r *transactionRepository) Update(t *model.Transaction) error {
	return r.db.Save(t).Error
}

func (r *transactionRepository) Delete(userID string, id uint) error {
	return r.db.Where("user_id = ?", model.UserIDOrDefault(userID)).Delete(&model.Transaction{}, id).Error
}
