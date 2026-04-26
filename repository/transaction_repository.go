package repository

import (
	"finance-chat/model"

	"gorm.io/gorm"
)

type TransactionRepository interface {
	Create(t *model.Transaction) error
	FindAll() ([]model.Transaction, error)
	FindByID(id uint) (*model.Transaction, error)
	Update(t *model.Transaction) error
	Delete(id uint) error
}

type transactionRepository struct {
	db *gorm.DB
}

func NewTransactionRepository(db *gorm.DB) TransactionRepository {
	return &transactionRepository{db: db}
}

func (r *transactionRepository) Create(t *model.Transaction) error {
	return r.db.Create(t).Error
}

func (r *transactionRepository) FindAll() ([]model.Transaction, error) {
	var list []model.Transaction
	err := r.db.Order("created_at desc").Find(&list).Error
	return list, err
}

func (r *transactionRepository) FindByID(id uint) (*model.Transaction, error) {
	var t model.Transaction
	err := r.db.First(&t, id).Error
	return &t, err
}

func (r *transactionRepository) Update(t *model.Transaction) error {
	return r.db.Save(t).Error
}

func (r *transactionRepository) Delete(id uint) error {
	return r.db.Delete(&model.Transaction{}, id).Error
}
