package controller

import (
	"finance-chat/model"
	"finance-chat/service"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type WalletController struct {
	svc           *service.WalletService // Linked to your business logic layer
	svcTransction service.TransactionService
}

// NewWalletController now correctly accepts a *service.WalletService instance
func NewWalletController(svc *service.WalletService) *WalletController {
	return &WalletController{svc: svc}
}

type SetCurrentWalletRequest struct {
	WalletID uint `json:"wallet_id" binding:"required"`
}

// ListWallets -> GET /api/wallets
func (ctrl *WalletController) ListWallets(c *gin.Context) {
	userID := c.DefaultQuery("user_id", model.DefaultUserID)

	wallets, err := ctrl.svc.GetWalletsForUser(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve wallets: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, wallets)
}

// CreateWallet -> POST /api/wallets
func (ctrl *WalletController) CreateWallet(c *gin.Context) {
	var payload struct {
		Name   string  `json:"name" binding:"required"`
		Icon   string  `json:"icon"`
		Target float64 `json:"target"`
	}

	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	newWallet := model.Wallet{
		UserID: c.DefaultQuery("user_id", model.DefaultUserID),
		Name:   payload.Name,
		Icon:   payload.Icon,
		Target: payload.Target,
	}

	if err := ctrl.svc.CreateNewOccasionWallet(&newWallet); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create wallet: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, newWallet)
}

// RemoveOrArchiveWallet -> DELETE /api/wallets/:id
func (ctrl *WalletController) RemoveOrArchiveWallet(c *gin.Context) {
	walletIDStr := c.Param("id")
	walletID, err := strconv.ParseUint(walletIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid wallet ID format"})
		return
	}

	userID := c.DefaultQuery("user_id", model.DefaultUserID)

	if err := ctrl.svc.RemoveOrArchiveWallet(userID, uint(walletID)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Wallet successfully archived",
		"wallet_id": walletID,
	})
}

func (c *WalletController) SetCurrentWallet(ctx *gin.Context) {
	userID := requestUserID(ctx)

	var req SetCurrentWalletRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	if err := c.svcTransction.SetCurrentWallet(userID, req.WalletID); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	wallet, err := c.svcTransction.GetCurrentWallet(userID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "current wallet updated",
		"wallet":  wallet,
	})
}

func (c *WalletController) GetCurrentWallet(ctx *gin.Context) {
	userID := requestUserID(ctx)

	wallet, err := c.svcTransction.GetCurrentWallet(userID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"wallet": wallet,
	})
}
