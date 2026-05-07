package controller

import (
	"encoding/json"
	"finance-chat/agent"
	"finance-chat/model"
	"finance-chat/service"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// LineMessage represents a single Line message object
type LineMessage struct {
	Type string `json:"type" example:"text"`
	Text string `json:"text" example:"spent 100 baht on coffee"`
}

// LineSource holds the sender's userId
type LineSource struct {
	UserID string `json:"userId"`
}

// LineEvent represents a single Line event
type LineEvent struct {
	Type       string      `json:"type"       example:"message"`
	ReplyToken string      `json:"replyToken" example:"nHuyWiB7yP5Zw52FIkcQobQuGDXCTA"`
	Source     LineSource  `json:"source"`
	Message    LineMessage `json:"message"`
}

// LinePayload is the top-level Line webhook request body
type LinePayload struct {
	Events []LineEvent `json:"events"`
}

type WebhookController struct {
	txSvc   service.TransactionService
	lineSvc agent.LineService

	// lastTx tracks the most recent transaction per Line userId (in-memory)
	mu     sync.Mutex
	lastTx map[string]*model.Transaction
}

func NewWebhookController(txSvc service.TransactionService, lineSvc agent.LineService) *WebhookController {
	return &WebhookController{
		txSvc:   txSvc,
		lineSvc: lineSvc,
		lastTx:  make(map[string]*model.Transaction),
	}
}

// LineWebhook godoc
// @Summary      Line Messaging API webhook
// @Description  Receives events from Line and replies with AI-parsed transaction info. Always returns 200.
// @Description  To correct the last transaction, send: edit brand=Starbucks sub=Coffee tag=treat
// @Tags         webhook
// @Accept       json
// @Produce      json
// @Param        body  body      LinePayload  false  "Line webhook payload"
// @Success      200
// @Router       /webhook [post]
func (w *WebhookController) LineWebhook(ctx *gin.Context) {
	body, _ := io.ReadAll(ctx.Request.Body)

	if !w.lineSvc.VerifySignature(body, ctx.GetHeader("X-Line-Signature")) {
		log.Println("[webhook] warning: invalid or missing X-Line-Signature")
	}

	var payload LinePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("[webhook] could not parse payload: %v", err)
		ctx.Status(http.StatusOK)
		return
	}

	for _, event := range payload.Events {
		if event.Type == "message" && event.Message.Type == "text" && event.Message.Text != "" {
			go w.reply(event.ReplyToken, event.Source.UserID, event.Message.Text)
		}
	}

	ctx.Status(http.StatusOK)
}

func (w *WebhookController) reply(replyToken, userID, text string) {
	lower := strings.ToLower(strings.TrimSpace(text))

	if lower == "edit" || lower == "แก้ไข" ||
		strings.HasPrefix(lower, "edit ") || strings.HasPrefix(lower, "แก้ไข ") {
		w.handleCorrection(replyToken, userID, text)
		return
	}

	result, err := w.txSvc.Chat(text)
	if err != nil {
		_ = w.lineSvc.ReplyMessage(replyToken, "Sorry, couldn't process that.")
		return
	}

	// Extract the transaction from the pipeline result
	tx := result.Transaction

	// Remember last transaction for this user
	w.mu.Lock()
	w.lastTx[userID] = tx
	w.mu.Unlock()

	_ = w.lineSvc.ReplyMessage(replyToken, formatReply(tx))
}

// handleCorrection — no ID needed, always edits the last transaction.
// Format: edit brand=Starbucks sub=Coffee category=Food & Drink tag=treat
func (w *WebhookController) handleCorrection(replyToken, userID, text string) {
	w.mu.Lock()
	tx := w.lastTx[userID]
	w.mu.Unlock()

	if tx == nil {
		_ = w.lineSvc.ReplyMessage(replyToken, "No recent transaction to edit. Send a transaction first.")
		return
	}

	// Parse key=value pairs from the message
	// Strip the leading "edit" / "แก้ไข" word first
	parts := strings.Fields(text)
	if len(parts) < 2 {
		_ = w.lineSvc.ReplyMessage(replyToken,
			"What would you like to fix?\nExample: edit brand=Starbucks sub=Coffee tag=treat")
		return
	}

	var category, sub, brand, tag string
	for _, f := range parts[1:] {
		if v, ok := strings.CutPrefix(f, "category="); ok {
			category = v
		} else if v, ok := strings.CutPrefix(f, "sub="); ok {
			sub = v
		} else if v, ok := strings.CutPrefix(f, "brand="); ok {
			brand = v
		} else if v, ok := strings.CutPrefix(f, "tag="); ok {
			tag = v
		}
	}

	if err := w.txSvc.Correct(tx.ID, category, sub, brand, tag); err != nil {
		_ = w.lineSvc.ReplyMessage(replyToken, fmt.Sprintf("Correction failed: %v", err))
		return
	}

	_ = w.lineSvc.ReplyMessage(replyToken, "✅ Got it, I'll remember that for next time.")
}

func formatReply(tx *model.Transaction) string {
	emoji := "💸"
	if tx.Type == model.Income {
		emoji = "💰"
	}
	brand := tx.Brand
	if brand == "" {
		brand = "-"
	}
	return fmt.Sprintf(
		"%s %s\nAmount: %.2f\nCategory: %s\nSub: %s\nBrand: %s\nBehavior: %s\nNote: %s\n\nWrong? Reply: edit brand=X sub=X tag=X",
		emoji, tx.Type, tx.Amount,
		tx.Category, tx.SubCategory, brand, tx.BehaviorTag, tx.Description,
	)
}
