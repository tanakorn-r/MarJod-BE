package controller

import (
	"errors"
	"finance-chat/model"
	"finance-chat/service"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type WalletController struct {
	svc           *service.WalletService // Linked to your business logic layer
	svcTransction service.TransactionService
}

func NewWalletController(svc *service.WalletService, txSvc service.TransactionService) *WalletController {
	return &WalletController{svc: svc, svcTransction: txSvc}
}

type SetCurrentWalletRequest struct {
	WalletID uint `json:"wallet_id" binding:"required"`
}

type CreateWalletRequest struct {
	Name   string  `json:"name" binding:"required"`
	Icon   string  `json:"icon"`
	Target float64 `json:"target"`
}

type UpdateWalletRequest struct {
	Name   *string  `json:"name"`
	Icon   *string  `json:"icon"`
	Target *float64 `json:"target"`
}

// ListWallets godoc
// @Summary      List wallets
// @Description  Returns all wallets for the user, including the implicit General wallet.
// @Tags         wallets
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array}   model.Wallet
// @Failure      500  {object}  ErrorResponse
// @Router       /api/wallets [get]
func (ctrl *WalletController) ListWallets(c *gin.Context) {
	userID := requestUserID(c)

	wallets, err := ctrl.svc.GetWalletsForUser(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve wallets: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, wallets)
}

// CreateWallet godoc
// @Summary      Create a wallet
// @Description  Creates a new occasion wallet for the user.
// @Tags         wallets
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      CreateWalletRequest  true  "Wallet to create"
// @Success      201   {object}  model.Wallet
// @Failure      400   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /api/wallets [post]
func (ctrl *WalletController) CreateWallet(c *gin.Context) {
	var payload CreateWalletRequest

	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	newWallet := model.Wallet{
		UserID: requestUserID(c),
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

// UpdateWallet godoc
// @Summary      Update a wallet
// @Description  Updates wallet details such as monthly target, name, or icon. Passing wallet ID 0 updates the user's General wallet after ensuring it exists.
// @Tags         wallets
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      int                  true  "Wallet ID"
// @Param        body  body      UpdateWalletRequest  true  "Wallet fields to update"
// @Success      200   {object}  model.Wallet
// @Failure      400   {object}  ErrorResponse
// @Failure      404   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /api/wallets/{id} [patch]
func (ctrl *WalletController) UpdateWallet(c *gin.Context) {
	walletID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid wallet ID format"})
		return
	}

	var payload UpdateWalletRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	wallet, err := ctrl.svc.UpdateWallet(requestUserID(c), uint(walletID), payload.Name, payload.Icon, payload.Target)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "wallet not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, wallet)
}

// RemoveOrArchiveWallet godoc
// @Summary      Remove or archive a wallet
// @Description  Archives the wallet (or removes it, depending on its state) for the user.
// @Tags         wallets
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "Wallet ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  ErrorResponse
// @Router       /api/wallets/{id} [delete]
func (ctrl *WalletController) RemoveOrArchiveWallet(c *gin.Context) {
	walletIDStr := c.Param("id")
	walletID, err := strconv.ParseUint(walletIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid wallet ID format"})
		return
	}

	userID := requestUserID(c)

	if err := ctrl.svc.RemoveOrArchiveWallet(userID, uint(walletID)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Wallet successfully archived",
		"wallet_id": walletID,
	})
}

// SetCurrentWallet godoc
// @Summary      Set the current wallet
// @Description  Sets which wallet new transactions should be attributed to.
// @Tags         wallets
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      SetCurrentWalletRequest  true  "Wallet to switch to"
// @Success      200   {object}  map[string]interface{}
// @Failure      400   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /api/wallets/current [patch]
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

// GetCurrentWallet godoc
// @Summary      Get the current wallet
// @Description  Returns the wallet currently selected for new transactions.
// @Tags         wallets
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  map[string]interface{}
// @Failure      500  {object}  ErrorResponse
// @Router       /api/wallets/current [get]
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
