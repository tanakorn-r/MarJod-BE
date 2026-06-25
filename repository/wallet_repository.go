package repository

import (
	"errors"

	"finance-chat/model"

	"gorm.io/gorm"
)

type WalletRepository struct {
	db *gorm.DB
}

type WalletRepositoryInterface interface {
	GetUserWalletsWithBalances(userID string) ([]model.Wallet, error)
	CreateNewWallet(wallet *model.Wallet) error
	ArchiveWallet(userID string, walletID uint) error
	GetWalletByID(userID string, walletID uint) (*model.Wallet, error)
	EnsureGeneralWallet(userID string) (*model.Wallet, error)
	SetCurrentWallet(userID string, walletID uint) error
	GetCurrentWallet(userID string) (*model.Wallet, error)
}

func NewWalletRepository(db *gorm.DB) *WalletRepository {
	if db == nil {
		panic("NewWalletRepository received nil db")
	}

	return &WalletRepository{db: db}
}

func (r *WalletRepository) ensureDB() error {
	if r == nil || r.db == nil {
		return gorm.ErrInvalidDB
	}

	return nil
}

// GetUserWalletsWithBalances collects custom rows, prepends General, and aggregates amounts
func (r *WalletRepository) GetUserWalletsWithBalances(userID string) ([]model.Wallet, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}

	normalizedUID := model.UserIDOrDefault(userID)

	var wallets []model.Wallet

	err := r.db.
		Where("user_id = ?", normalizedUID).
		Order("is_archived ASC").
		Order("created_at ASC").
		Find(&wallets).Error
	if err != nil {
		return nil, err
	}

	if len(wallets) == 0 {
		return []model.Wallet{}, nil
	}

	type AggregatedResult struct {
		WalletID uint
		Total    float64
	}

	var results []AggregatedResult

	err = r.db.Model(&model.Transaction{}).
		Select("wallet_id, COALESCE(SUM(amount), 0) AS total").
		Where("user_id = ? AND wallet_id IS NOT NULL AND type = ?", normalizedUID, model.Expense).
		Group("wallet_id").
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	spentMap := make(map[uint]float64, len(results))
	for _, res := range results {
		spentMap[res.WalletID] = res.Total
	}

	for i := range wallets {
		wallets[i].Spent = spentMap[wallets[i].ID]
	}

	return wallets, nil
}

// CreateNewWallet handles inserting custom occasion wallets
func (r *WalletRepository) CreateNewWallet(wallet *model.Wallet) error {
	if err := r.ensureDB(); err != nil {
		return err
	}

	if wallet == nil {
		return errors.New("wallet is nil")
	}

	return r.db.Create(wallet).Error
}

// ArchiveWallet marks custom targets as settled/archived instead of deleting rows
func (r *WalletRepository) ArchiveWallet(userID string, walletID uint) error {
	if err := r.ensureDB(); err != nil {
		return err
	}

	if walletID == model.GeneralWalletID {
		return errors.New("cannot archive general wallet")
	}

	return r.db.Model(&model.Wallet{}).
		Where("id = ? AND user_id = ?", walletID, model.UserIDOrDefault(userID)).
		Updates(map[string]interface{}{
			"is_archived": true,
		}).Error
}

// GetWalletByID retrieves a single wallet structure context by its ID
func (r *WalletRepository) GetWalletByID(userID string, walletID uint) (*model.Wallet, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}

	normalizedUID := model.UserIDOrDefault(userID)

	if walletID == model.GeneralWalletID {
		generalWallet := model.GetDefaultGeneralWallet(normalizedUID)

		var totalSpent float64
		err := r.db.Model(&model.Transaction{}).
			Select("COALESCE(SUM(amount), 0)").
			Where("user_id = ? AND wallet_id IS NULL AND type = ?", normalizedUID, model.Expense).
			Scan(&totalSpent).Error
		if err != nil {
			return nil, err
		}

		generalWallet.Spent = totalSpent
		return &generalWallet, nil
	}

	var wallet model.Wallet
	err := r.db.Where("id = ? AND user_id = ?", walletID, normalizedUID).
		First(&wallet).Error
	if err != nil {
		return nil, err
	}

	var totalSpent float64
	err = r.db.Model(&model.Transaction{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("user_id = ? AND wallet_id = ? AND type = ?", normalizedUID, walletID, model.Expense).
		Scan(&totalSpent).Error
	if err != nil {
		return nil, err
	}

	wallet.Spent = totalSpent
	return &wallet, nil
}

func (r *WalletRepository) EnsureGeneralWallet(userID string) (*model.Wallet, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}

	normalizedUID := model.UserIDOrDefault(userID)

	var wallet model.Wallet
	err := r.db.Where("user_id = ? AND name = ?", normalizedUID, "General").
		First(&wallet).Error

	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}

		wallet = model.Wallet{
			UserID:     normalizedUID,
			Name:       "General",
			Icon:       "👝",
			Target:     20000,
			IsArchived: false,
		}

		if err := r.db.Create(&wallet).Error; err != nil {
			return nil, err
		}
	}

	err = r.db.Model(&model.Transaction{}).
		Where("user_id = ? AND wallet_id IS NULL", normalizedUID).
		Update("wallet_id", wallet.ID).Error
	if err != nil {
		return nil, err
	}

	return &wallet, nil
}

func (r *WalletRepository) SetCurrentWallet(userID string, walletID uint) error {
	if err := r.ensureDB(); err != nil {
		return err
	}

	normalizedUID := model.UserIDOrDefault(userID)

	var wallet model.Wallet
	if err := r.db.
		Where("id = ? AND user_id = ? AND is_archived = ?", walletID, normalizedUID, false).
		First(&wallet).Error; err != nil {
		return err
	}

	selection := model.UserWalletSelection{
		UserID:   normalizedUID,
		WalletID: walletID,
	}

	return r.db.
		Where("user_id = ?", normalizedUID).
		Assign(model.UserWalletSelection{
			WalletID: walletID,
		}).
		FirstOrCreate(&selection).Error
}
func (r *WalletRepository) GetCurrentWallet(userID string) (*model.Wallet, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}

	normalizedUID := model.UserIDOrDefault(userID)

	var selection model.UserWalletSelection
	err := r.db.
		Where("user_id = ?", normalizedUID).
		First(&selection).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.EnsureGeneralWallet(normalizedUID)
		}

		return nil, err
	}

	var wallet model.Wallet
	err = r.db.
		Where("id = ? AND user_id = ? AND is_archived = ?", selection.WalletID, normalizedUID, false).
		First(&wallet).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.EnsureGeneralWallet(normalizedUID)
		}

		return nil, err
	}

	return &wallet, nil
}
