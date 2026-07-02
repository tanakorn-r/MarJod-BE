package controller

import (
	"encoding/json"
	"finance-chat/middleware"
	"finance-chat/model"
	"finance-chat/service"
	"finance-chat/timeutil"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type TransactionController struct {
	svc service.TransactionService
}

func NewTransactionController(svc service.TransactionService) *TransactionController {
	return &TransactionController{svc: svc}
}

// ChatRequest represents the chat input
type ChatRequest struct {
	Message string `json:"message" binding:"required" example:"spent 250 baht on lunch"`
}

// requestUserID returns the LINE userId verified by middleware.RequireLineAuth,
// which runs ahead of every /api handler and aborts unauthenticated requests
// before they get here. "default" preserves access to records created before
// per-user storage.
func requestUserID(ctx *gin.Context) string {
	if v, ok := ctx.Get(middleware.UserIDContextKey); ok {
		if userID, ok := v.(string); ok && userID != "" {
			return model.UserIDOrDefault(userID)
		}
	}
	return model.DefaultUserID
}

// ErrorResponse represents an error payload
type ErrorResponse struct {
	Error string `json:"error" example:"something went wrong"`
}

// Chat godoc
// @Summary      Parse a natural language message (blocking)
// @Description  Waits for the full LLM response, runs the agent pipeline, and returns the result with transaction, behavior DNA, alerts, and recommendations.
// @Tags         chat
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      ChatRequest        true  "Message to parse"
// @Success      201   {object}  agent.PipelineResult
// @Failure      400   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /api/chat [post]
func (c *TransactionController) Chat(ctx *gin.Context) {
	var req ChatRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	result, err := c.svc.Chat(requestUserID(ctx), req.Message)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, result)
}

// ChatStream godoc
// @Summary      Parse a natural language message (SSE streaming)
// @Description  Streams LLM tokens as Server-Sent Events. Each token is sent as `event: token`.
// @Description  When the model finishes, the pipeline result (transaction + behavior DNA + alerts + recommendations) is sent as `event: done` with JSON body.
// @Description  On error, `event: error` is sent and the stream closes.
// @Tags         chat
// @Accept       json
// @Produce      text/event-stream
// @Security     BearerAuth
// @Param        body  body  ChatRequest  true  "Message to parse"
// @Success      200
// @Failure      400  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /api/chat/stream [post]
func (c *TransactionController) ChatStream(ctx *gin.Context) {
	var req ChatRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	tokens, done, errc := c.svc.ChatStream(requestUserID(ctx), req.Message)

	ctx.Header("Content-Type", "text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")
	ctx.Header("X-Accel-Buffering", "no") // disable nginx buffering if behind proxy

	ctx.Stream(func(w io.Writer) bool {
		select {
		case token, ok := <-tokens:
			if !ok {
				// tokens exhausted — nil it so we stop selecting on it
				tokens = nil
				return true // keep looping to catch done/errc
			}
			fmt.Fprintf(w, "event: token\ndata: %s\n\n", token)
			return true

		case result, ok := <-done:
			if !ok {
				return false
			}
			b, _ := json.Marshal(result)
			fmt.Fprintf(w, "event: done\ndata: %s\n\n", b)
			return false

		case err, ok := <-errc:
			if !ok {
				return false
			}
			fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
			return false

		case <-ctx.Request.Context().Done():
			return false
		}
	})
}

// CorrectionRequest represents a user correction payload
type CorrectionRequest struct {
	Category    string `json:"category"     example:"Food & Drink"`
	SubCategory string `json:"sub_category" example:"Coffee"`
	Brand       string `json:"brand"        example:"Starbucks"`
	BehaviorTag string `json:"behavior_tag" example:"treat"`
}

// Correct godoc
// @Summary      Correct a transaction's classification
// @Description  User can fix category, sub_category, brand or behavior_tag. Corrections are saved and fed back to the AI on future requests.
// @Tags         transactions
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      int                true  "Transaction ID"
// @Param        body  body      CorrectionRequest  true  "Fields to correct (only non-empty fields are applied)"
// @Success      200   {object}  model.Transaction
// @Failure      400   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /api/transactions/{id}/correct [patch]
func (c *TransactionController) Correct(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
		return
	}

	var req CorrectionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	if err := c.svc.Correct(requestUserID(ctx), uint(id), req.Category, req.SubCategory, req.Brand, req.BehaviorTag); err != nil {
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "correction saved"})
}

// ListByCategory godoc
// @Summary      List transactions by category (paginated)
// @Description  Returns transactions filtered by category and optional month, sorted by most recent first.
// @Tags         transactions
// @Produce      json
// @Security     BearerAuth
// @Param        category  query     string  false  "Category name (e.g. Food & Beverage)"
// @Param        month     query     string  false  "Month filter YYYY-MM (e.g. 2026-06). Defaults to all months."
// @Param        page      query     int     false  "Page number (default 1)"
// @Param        limit     query     int     false  "Items per page (default 20, max 100)"
// @Success      200       {object}  service.CategoryPage
// @Failure      500       {object}  ErrorResponse
// @Router       /api/transactions/by-category [get]
func (c *TransactionController) ListByCategory(ctx *gin.Context) {
	category := ctx.Query("category")
	month := ctx.Query("month")

	page := 1
	if p, err := strconv.Atoi(ctx.Query("page")); err == nil && p > 0 {
		page = p
	}
	limit := 20
	if l, err := strconv.Atoi(ctx.Query("limit")); err == nil && l > 0 {
		limit = l
	}

	result, err := c.svc.ListByCategory(requestUserID(ctx), category, month, page, limit)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, result)
}

// List godoc
// @Summary      List all transactions
// @Description  Returns all saved income and expense transactions, newest first.
// @Tags         transactions
// @Produce      json
// @Security     BearerAuth
// @Param        wallet_id  query     int  false  "Wallet ID to filter by. Use 0 for General wallet. Omit for all wallets."
// @Success      200  {array}   model.Transaction
// @Failure      400  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /api/transactions [get]
func (c *TransactionController) List(ctx *gin.Context) {
	walletIDStr := ctx.Query("wallet_id")
	if walletIDStr == "" {
		list, err := c.svc.List(requestUserID(ctx))
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			return
		}
		ctx.JSON(http.StatusOK, list)
		return
	}

	id, err := strconv.ParseUint(walletIDStr, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid wallet_id format"})
		return
	}

	list, err := c.svc.List(requestUserID(ctx), uint(id))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, list)
}

// Delete godoc
// @Summary      Delete a transaction
// @Description  Remove a transaction by its ID.
// @Tags         transactions
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "Transaction ID"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /api/transactions/{id} [delete]
func (c *TransactionController) Delete(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
		return
	}

	if err := c.svc.Delete(requestUserID(ctx), uint(id)); err != nil {
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// Summary godoc
// @Summary      Get financial summary
// @Description  Returns total income, total expense, and current balance.
// @Tags         transactions
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  service.Summary
// @Failure      500  {object}  ErrorResponse
// @Router       /api/summary [get]
func (c *TransactionController) Summary(ctx *gin.Context) {
	sum, err := c.svc.Summary(requestUserID(ctx))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, sum)
}

// GetByID godoc
// @Summary      Get transaction details
// @Description  Returns a single transaction by ID with full details.
// @Tags         transactions
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "Transaction ID"
// @Success      200  {object}  model.Transaction
// @Failure      400  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /api/transactions/{id} [get]
func (c *TransactionController) GetByID(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
		return
	}

	tx, err := c.svc.GetByID(requestUserID(ctx), uint(id))
	if err != nil {
		ctx.JSON(http.StatusNotFound, ErrorResponse{Error: "transaction not found"})
		return
	}

	ctx.JSON(http.StatusOK, tx)
}

// Analytics godoc
// @Summary      Get analytics dashboard data
// @Description  Returns comprehensive analytics for the given month. Defaults to current month.
// @Tags         analytics
// @Produce      json
// @Security     BearerAuth
// @Param        month  query     string  false  "Month to analyse (YYYY-MM, e.g. 2026-06). Defaults to current month."
// @Success      200    {object}  service.AnalyticsDashboard
// @Failure      400    {object}  ErrorResponse
// @Failure      500    {object}  ErrorResponse
// @Router       /api/analytics [get]
func (c *TransactionController) Analytics(ctx *gin.Context) {
	month, err := parseMonthParam(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	// 1. Extract the wallet_id query parameter (e.g., ?wallet_id=12)
	var walletID uint
	walletIDStr := ctx.Query("wallet_id")
	if walletIDStr != "" {
		// Parse the string into a uint
		var id uint64
		id, err = strconv.ParseUint(walletIDStr, 10, 32)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid wallet_id format"})
			return
		}
		walletID = uint(id)
	}

	// 2. Pass walletID as the third argument
	analytics, err := c.svc.GetAnalytics(requestUserID(ctx), month, walletID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, analytics)
}

// AnalyticsTrend godoc
// @Summary      Get multi-month expense trend
// @Description  Returns just the total expense for each of the last `months` calendar months ending at `month`. Lightweight alternative to calling GET /api/analytics once per month.
// @Tags         analytics
// @Produce      json
// @Security     BearerAuth
// @Param        month   query     string  false  "Last month in the trend (YYYY-MM, e.g. 2026-06). Defaults to current month."
// @Param        months  query     int     false  "Number of trailing months to include (1-24). Defaults to 6."
// @Success      200     {array}   service.MonthlyExpensePoint
// @Failure      400     {object}  ErrorResponse
// @Failure      500     {object}  ErrorResponse
// @Router       /api/analytics/trend [get]
func (c *TransactionController) AnalyticsTrend(ctx *gin.Context) {
	month, err := parseMonthParam(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	months := 6
	if raw := ctx.Query("months"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 24 {
			ctx.JSON(http.StatusBadRequest, ErrorResponse{Error: "months must be an integer between 1 and 24"})
			return
		}
		months = n
	}

	trend, err := c.svc.GetExpenseTrend(requestUserID(ctx), month, months)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, trend)
}

// SpendingDNA godoc
// @Summary      Get the user's behavioral spending DNA
// @Description  Computes a real, data-derived behavioral snapshot — archetype, consistency/impulse-control/volatility labels, save rate, dominant category, impulse frequency, luxury drift, and top brands — entirely from real transactions. Not month-scoped; uses trailing 30/60-day windows like the existing behavior profiler.
// @Tags         analytics
// @Produce      json
// @Security     BearerAuth
// @Param        wallet_id  query     int  false  "Wallet ID to scope DNA to. Use 0 for General wallet."
// @Success      200  {object}  service.SpendingDNA
// @Failure      400  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /api/analytics/dna [get]
func (c *TransactionController) SpendingDNA(ctx *gin.Context) {
	walletIDStr := ctx.Query("wallet_id")
	if walletIDStr == "" {
		dna, err := c.svc.GetSpendingDNA(requestUserID(ctx))
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			return
		}
		ctx.JSON(http.StatusOK, dna)
		return
	}

	id, err := strconv.ParseUint(walletIDStr, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid wallet_id format"})
		return
	}

	dna, err := c.svc.GetSpendingDNA(requestUserID(ctx), uint(id))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, dna)
}

// AnalyticsInsight godoc
// @Summary      Generate an AI personal-finance insight
// @Description  Explicitly invokes OpenAI using the calculated analytics for the selected month. This endpoint consumes AI tokens; GET /api/analytics does not.
// @Tags         analytics
// @Produce      json
// @Security     BearerAuth
// @Param        month  query     string  false  "Month to analyse (YYYY-MM, e.g. 2026-06). Defaults to current month."
// @Success      200    {object}  service.PersonalFinanceInsight
// @Failure      400    {object}  ErrorResponse
// @Failure      500    {object}  ErrorResponse
// @Router       /api/analytics/insight [post]
func (c *TransactionController) AnalyticsInsight(ctx *gin.Context) {
	month, err := parseMonthParam(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	// 1. Extract the wallet_id query parameter (e.g., ?wallet_id=12)
	var walletID uint
	walletIDStr := ctx.Query("wallet_id")
	if walletIDStr != "" {
		var id uint64
		id, err = strconv.ParseUint(walletIDStr, 10, 32)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid wallet_id format"})
			return
		}
		walletID = uint(id)
	}

	// 2. Pass walletID down as the third argument
	insight, err := c.svc.GetAnalyticsInsight(requestUserID(ctx), month, walletID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, insight)
}

// parseMonthParam reads ?month=YYYY-MM from the query string.
// Returns Thailand current time when the param is absent, 400 when it's malformed.
func parseMonthParam(ctx *gin.Context) (time.Time, error) {
	raw := ctx.Query("month")
	if raw == "" {
		return timeutil.Now(), nil
	}
	t, err := timeutil.ParseMonth(raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid month format %q — use YYYY-MM (e.g. 2026-06)", raw)
	}
	return t, nil
}

// ListCorrections godoc
// @Summary      List all user corrections
// @Description  Returns all manual corrections made by the user. These corrections are used to train the AI.
// @Tags         corrections
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array}   model.UserCorrection
// @Failure      500  {object}  ErrorResponse
// @Router       /api/corrections [get]
func (c *TransactionController) ListCorrections(ctx *gin.Context) {
	corrections, err := c.svc.ListCorrections(requestUserID(ctx))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, corrections)
}

// DeleteCorrection godoc
// @Summary      Delete a user correction
// @Description  Remove a correction by its ID. This will stop the AI from learning from this example.
// @Tags         corrections
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "Correction ID"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /api/corrections/{id} [delete]
func (c *TransactionController) DeleteCorrection(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
		return
	}

	if err := c.svc.DeleteCorrection(requestUserID(ctx), uint(id)); err != nil {
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "correction deleted"})
}
