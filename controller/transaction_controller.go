package controller

import (
	"encoding/json"
	"finance-chat/service"
	"fmt"
	"io"
	"net/http"
	"strconv"

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

// ErrorResponse represents an error payload
type ErrorResponse struct {
	Error string `json:"error" example:"something went wrong"`
}

// Chat godoc
// @Summary      Parse a natural language message (blocking)
// @Description  Waits for the full LLM response, saves the transaction, and returns it.
// @Tags         chat
// @Accept       json
// @Produce      json
// @Param        body  body      ChatRequest        true  "Message to parse"
// @Success      201   {object}  model.Transaction
// @Failure      400   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /api/chat [post]
func (c *TransactionController) Chat(ctx *gin.Context) {
	var req ChatRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	tx, err := c.svc.Chat(req.Message)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, tx)
}

// ChatStream godoc
// @Summary      Parse a natural language message (SSE streaming)
// @Description  Streams LLM tokens as Server-Sent Events. Each token is sent as `event: token`.
// @Description  When the model finishes, the saved transaction is sent as `event: done` with JSON body.
// @Description  On error, `event: error` is sent and the stream closes.
// @Tags         chat
// @Accept       json
// @Produce      text/event-stream
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

	tokens, done, errc := c.svc.ChatStream(req.Message)

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

		case tx, ok := <-done:
			if !ok {
				return false
			}
			b, _ := json.Marshal(tx)
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

	if err := c.svc.Correct(uint(id), req.Category, req.SubCategory, req.Brand, req.BehaviorTag); err != nil {
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "correction saved"})
}

// List godoc
// @Summary      List all transactions
// @Description  Returns all saved income and expense transactions, newest first.
// @Tags         transactions
// @Produce      json
// @Success      200  {array}   model.Transaction
// @Failure      500  {object}  ErrorResponse
// @Router       /api/transactions [get]
func (c *TransactionController) List(ctx *gin.Context) {
	list, err := c.svc.List()
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

	if err := c.svc.Delete(uint(id)); err != nil {
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
// @Success      200  {object}  service.Summary
// @Failure      500  {object}  ErrorResponse
// @Router       /api/summary [get]
func (c *TransactionController) Summary(ctx *gin.Context) {
	sum, err := c.svc.Summary()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, sum)
}
