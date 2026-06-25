package model

import (
	"strings"
	"time"
)

const (
	// GeneralWalletID represents the virtual ID for the implicit default wallet
	GeneralWalletID uint = 0
)

type Wallet struct {
	ID         uint       `json:"id"          gorm:"primaryKey;autoIncrement"`
	UserID     string     `json:"user_id"     gorm:"size:128;not null;default:default;index"`
	Name       string     `json:"name"        gorm:"size:100;not null"`
	Icon       string     `json:"icon"        gorm:"size:20"`
	Target     float64    `json:"target"      gorm:"type:decimal(15,2);not null;default:0"`
	Spent      float64    `json:"spent"       gorm:"-"`
	IsArchived bool       `json:"is_archived" gorm:"not null;default:false;index"`
	StartDate  *time.Time `json:"start_date"`
	EndDate    *time.Time `json:"end_date"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func (w *Wallet) BeforeCreate() error {
	w.UserID = strings.TrimSpace(w.UserID)
	if w.UserID == "" {
		w.UserID = DefaultUserID
	}
	return nil
}

// GetDefaultGeneralWallet generates a dynamic layout matching your screenshot
func GetDefaultGeneralWallet(userID string) Wallet {
	return Wallet{
		ID:         GeneralWalletID,
		UserID:     UserIDOrDefault(userID),
		Name:       "General",
		Icon:       "👝",
		Target:     50000,
		IsArchived: false,
	}
}
