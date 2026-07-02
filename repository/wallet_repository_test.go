package repository

import (
	"finance-chat/model"
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newWalletTestRepo(t *testing.T) (*gorm.DB, *WalletRepository) {
	t.Helper()

	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Wallet{}, &model.Transaction{}, &model.UserWalletSelection{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	return db, NewWalletRepository(db)
}

func TestWalletRepository_UpdateGeneralWalletTargetIsReadByGeneralID(t *testing.T) {
	_, repo := newWalletTestRepo(t)
	target := 42000.0
	updated, err := repo.UpdateWallet("line-user-1", model.GeneralWalletID, map[string]interface{}{
		"target": target,
	})
	if err != nil {
		t.Fatalf("update general wallet: %v", err)
	}
	if updated.ID == model.GeneralWalletID {
		t.Fatalf("expected persisted general wallet id, got %d", updated.ID)
	}
	if updated.Target != target {
		t.Fatalf("updated target: want %.0f, got %.0f", target, updated.Target)
	}

	readBack, err := repo.GetWalletByID("line-user-1", model.GeneralWalletID)
	if err != nil {
		t.Fatalf("read general wallet: %v", err)
	}
	if readBack.Target != target {
		t.Fatalf("read target: want %.0f, got %.0f", target, readBack.Target)
	}
}

func TestWalletRepository_GetCurrentWalletCreatesGeneralAndSelectionWhenUserHasNoWallet(t *testing.T) {
	db, repo := newWalletTestRepo(t)

	wallet, err := repo.GetCurrentWallet("line-user-1")
	if err != nil {
		t.Fatalf("get current wallet: %v", err)
	}
	if wallet.ID == model.GeneralWalletID {
		t.Fatalf("expected persisted general wallet id, got %d", wallet.ID)
	}
	if wallet.Name != "General" {
		t.Fatalf("wallet name: want General, got %s", wallet.Name)
	}

	var selection model.UserWalletSelection
	if err := db.Where("user_id = ?", "line-user-1").First(&selection).Error; err != nil {
		t.Fatalf("selection was not persisted: %v", err)
	}
	if selection.WalletID != wallet.ID {
		t.Fatalf("selection wallet: want %d, got %d", wallet.ID, selection.WalletID)
	}
}

func TestWalletRepository_GetCurrentWalletSelectsGeneralWhenNoSelection(t *testing.T) {
	db, repo := newWalletTestRepo(t)
	userID := "line-user-1"
	createdAt := time.Now().Add(-time.Hour)
	first := model.Wallet{UserID: userID, Name: "Trip", Icon: "🧳", Target: 1000, CreatedAt: createdAt}
	second := model.Wallet{UserID: userID, Name: "Food", Icon: "🍜", Target: 2000, CreatedAt: createdAt.Add(time.Minute)}
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("create first wallet: %v", err)
	}
	if err := db.Create(&second).Error; err != nil {
		t.Fatalf("create second wallet: %v", err)
	}

	wallet, err := repo.GetCurrentWallet(userID)
	if err != nil {
		t.Fatalf("get current wallet: %v", err)
	}
	if wallet.ID == model.GeneralWalletID {
		t.Fatalf("expected persisted general wallet id, got %d", wallet.ID)
	}
	if wallet.Name != "General" {
		t.Fatalf("current wallet: want General, got %s", wallet.Name)
	}

	var selection model.UserWalletSelection
	if err := db.Where("user_id = ?", userID).First(&selection).Error; err != nil {
		t.Fatalf("selection was not persisted: %v", err)
	}
	if selection.WalletID != wallet.ID {
		t.Fatalf("selection wallet: want %d, got %d", wallet.ID, selection.WalletID)
	}
}

func TestWalletRepository_GetCurrentWalletUsesExistingSelection(t *testing.T) {
	db, repo := newWalletTestRepo(t)
	userID := "line-user-1"
	first := model.Wallet{UserID: userID, Name: "Trip", Icon: "🧳", Target: 1000}
	second := model.Wallet{UserID: userID, Name: "Food", Icon: "🍜", Target: 2000}
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("create first wallet: %v", err)
	}
	if err := db.Create(&second).Error; err != nil {
		t.Fatalf("create second wallet: %v", err)
	}
	if err := repo.SetCurrentWallet(userID, second.ID); err != nil {
		t.Fatalf("set current wallet: %v", err)
	}

	wallet, err := repo.GetCurrentWallet(userID)
	if err != nil {
		t.Fatalf("get current wallet: %v", err)
	}
	if wallet.ID != second.ID {
		t.Fatalf("current wallet: want selected wallet %d, got %d", second.ID, wallet.ID)
	}
}
