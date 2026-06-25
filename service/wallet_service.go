package service

import (
	"errors"
	"finance-chat/model"
	"finance-chat/repository"
)

type WalletService struct {
	repo *repository.WalletRepository
}

// NewWalletService instantiates a new business logic service container
func NewWalletService(repo *repository.WalletRepository) *WalletService {
	return &WalletService{repo: repo}
}

// GetWalletsForUser orchestrates user wallets retrieval, passing calculated metrics to UI
func (s *WalletService) GetWalletsForUser(userID string) ([]model.Wallet, error) {
	// Business Layer Guardrail
	if userID == "" {
		userID = model.DefaultUserID
	}

	print(userID)
	return s.repo.GetUserWalletsWithBalances(userID)
}

// CreateNewOccasionWallet enforces operational rules before saving a custom occasion target
func (s *WalletService) CreateNewOccasionWallet(wallet *model.Wallet) error {
	if wallet.Name == "" {
		return errors.New("wallet name cannot be empty")
	}
	if wallet.Target <= 0 {
		return errors.New("wallet budget target must be greater than zero")
	}

	// Default icon fallback if the user didn't specify one
	if wallet.Icon == "" {
		wallet.Icon = "💼"
	}

	return s.repo.CreateNewWallet(wallet)
}

// RemoveOrArchiveWallet checks system guards before delegating archive states to db storage
func (s *WalletService) RemoveOrArchiveWallet(userID string, walletID uint) error {
	// Rule: Prevent structural deletion or modification of virtual General fallback system
	if walletID == model.GeneralWalletID {
		return errors.New("the core General wallet container cannot be modified or removed")
	}

	if userID == "" {
		userID = model.DefaultUserID
	}

	return s.repo.ArchiveWallet(userID, walletID)
}
