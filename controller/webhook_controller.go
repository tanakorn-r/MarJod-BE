package controller

import (
	"encoding/json"
	"finance-chat/agent"
	"finance-chat/model"
	"finance-chat/repository"
	"finance-chat/service"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

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
	txRepo  repository.TransactionRepository
}

func NewWebhookController(txSvc service.TransactionService, lineSvc agent.LineService, txRepo repository.TransactionRepository) *WebhookController {
	return &WebhookController{
		txSvc:   txSvc,
		lineSvc: lineSvc,
		txRepo:  txRepo,
	}
}

// LineWebhook godoc
// @Summary      Line Messaging API webhook
// @Description  Receives events from Line and replies with AI-parsed transaction info. Always returns 200.
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
		if event.Type != "message" ||
			event.Message.Type != "text" ||
			event.Message.Text == "" {
			continue
		}

		userID := event.Source.UserID
		if strings.TrimSpace(userID) == "" {
			log.Println("[webhook] refusing to save transaction without LINE source.userId")
			ctx.JSON(http.StatusOK, gin.H{"error": "LINE event has no source.userId"})
			return
		}

		// Save + classify transaction
		result, err := w.txSvc.Chat(userID, event.Message.Text)
		if err != nil {
			ctx.JSON(http.StatusOK, gin.H{
				"error": err.Error(),
			})
			return
		}

		// Today's totals for this user
		todayIncome, todayExpense := w.getTodayTotals(userID)

		reply := formatTransactionReply(
			result,
			todayExpense,
			todayIncome,
		)

		// Send back to LINE
		if event.ReplyToken != "" {
			_ = w.lineSvc.ReplyMessage(
				event.ReplyToken,
				reply,
			)
		}

		// Debug response
		ctx.JSON(http.StatusOK, gin.H{
			"user_id":       userID,
			"input":         event.Message.Text,
			"today_income":  todayIncome,
			"today_expense": todayExpense,
			"reply":         reply,
		})

		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "no text event",
	})
}
func helpMessage() string {
	return `👋 สวัสดี ผมช่วยจดรายรับรายจ่ายให้ได้

ตัวอย่างที่พิมพ์ได้:

☕ กาแฟ Starbucks 180
🚕 Grab ไปออฟฟิศ 85
🍜 ข้าวกลางวัน 60
💰 เงินเดือน 35000

📊 ดูสรุปวันนี้
พิมพ์: สรุป

✏️ แก้ไขรายการล่าสุด
พิมพ์:
edit brand=Starbucks
edit sub=Coffee
edit tag=treat

ลองส่งข้อความมาได้เลย 😊`
}

func (w *WebhookController) reply(replyToken, userID, text string) {
	lower := strings.ToLower(strings.TrimSpace(text))

	// ── Command routing ───────────────────────────────────────
	switch {
	case lower == "สรุป" ||
		lower == "summary" ||
		lower == "today" ||
		lower == "วันนี้":

		w.handleDailySummary(replyToken, userID)
		return

	case lower == "edit" ||
		lower == "แก้ไข" ||
		strings.HasPrefix(lower, "edit ") ||
		strings.HasPrefix(lower, "แก้ไข "):

		w.handleCorrection(replyToken, userID, text)
		return

	case lower == "help" ||
		lower == "ช่วยด้วย" ||
		lower == "?":

		_ = w.lineSvc.ReplyMessage(replyToken, helpMessage())
		return
	}

	// ── Parse transaction ─────────────────────────────────────
	result, err := w.txSvc.Chat(userID, text)
	if err != nil {
		_ = w.lineSvc.ReplyMessage(
			replyToken,
			"❌ ขอโทษนะ ประมวลผลไม่ได้\nSorry, couldn't process that.",
		)
		return
	}

	// ── Get today's summary ───────────────────────────────────

	todayIncome, todayExpense := w.getTodayTotals(userID)

	reply := formatTransactionReply(
		result,
		todayExpense,
		todayIncome,
	)

	_ = w.lineSvc.ReplyMessage(replyToken, reply)
}
func (w *WebhookController) getTodayTotals(userID string) (income, expense float64) {
	txs, err := w.txRepo.FindTodayByUserID(userID)
	if err != nil {
		return 0, 0
	}

	for _, t := range txs {
		if t.Type == model.Income {
			income += t.Amount
		} else {
			expense += t.Amount
		}
	}

	return
}

// handleDailySummary replies with today's spending summary
func (w *WebhookController) handleDailySummary(replyToken, userID string) {
	txs, err := w.txRepo.FindTodayByUserID(userID)
	if err != nil || len(txs) == 0 {
		_ = w.lineSvc.ReplyMessage(replyToken,
			"📭 ยังไม่มีรายการวันนี้\nNo transactions recorded today yet.")
		return
	}

	var totalIncome, totalExpense float64
	var expenseLines strings.Builder

	for _, t := range txs {
		if t.Type == model.Income {
			totalIncome += t.Amount
		} else {
			totalExpense += t.Amount
			icon := categoryIcon(t.Category)
			expenseLines.WriteString(fmt.Sprintf("  %s %s  ฿%.0f\n", icon, t.Description, t.Amount))
		}
	}

	balance := totalIncome - totalExpense
	balanceIcon := "💚"
	if balance < 0 {
		balanceIcon = "🔴"
	}

	var sb strings.Builder
	sb.WriteString("📅 สรุปวันนี้ · Today's Summary\n")
	if expenseLines.Len() > 0 {
		sb.WriteString("\n💸 รายจ่าย · Expenses:\n")
		sb.WriteString(expenseLines.String())
	}
	if totalIncome > 0 {
		sb.WriteString(fmt.Sprintf("\n💰 รายรับ · Income:  ฿%.0f\n", totalIncome))
	}
	sb.WriteString(fmt.Sprintf("💸 รายจ่ายรวม  ฿%.0f\n", totalExpense))
	if totalIncome > 0 {
		sb.WriteString(fmt.Sprintf("💰 รายรับรวม   ฿%.0f\n", totalIncome))
	}
	sb.WriteString(fmt.Sprintf("%s คงเหลือ       ฿%.0f\n", balanceIcon, balance))

	_ = w.lineSvc.ReplyMessage(replyToken, sb.String())
}

// handleCorrection edits the most recent transaction
func (w *WebhookController) handleCorrection(replyToken, userID, text string) {
	tx, err := w.txRepo.FindLatestByUserID(userID)
	if err != nil || tx == nil {
		_ = w.lineSvc.ReplyMessage(replyToken,
			"❓ ไม่พบรายการล่าสุด\nNo recent transaction to edit. Send a transaction first.")
		return
	}

	parts := strings.Fields(text)
	if len(parts) < 2 {
		_ = w.lineSvc.ReplyMessage(replyToken,
			"✏️ แก้ไขอย่างไร?\nExample: edit brand=Starbucks sub=Coffee tag=treat")
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

	if err := w.txSvc.Correct(userID, tx.ID, category, sub, brand, tag); err != nil {
		_ = w.lineSvc.ReplyMessage(replyToken, fmt.Sprintf("❌ แก้ไขไม่ได้: %v", err))
		return
	}

	_ = w.lineSvc.ReplyMessage(replyToken, "✅ แก้ไขแล้ว! จำไว้สำหรับครั้งหน้า\nGot it, I'll remember that!")
}

// ─────────────────────────────────────────────────────────────
// Formatters
// ─────────────────────────────────────────────────────────────

func formatTransactionReply(
	result *agent.PipelineResult,
	todayExpense float64,
	todayIncome float64,
) string {

	tx := result.Transaction
	if tx == nil {
		return "❌ ไม่สามารถบันทึกรายการได้"
	}

	var sb strings.Builder

	// Header
	if tx.Type == model.Income {
		sb.WriteString("💰 รับทราบรายรับ\n\n")
	} else {
		sb.WriteString("📒 จดให้แล้ว\n\n")
	}

	// Display name
	name := tx.Brand

	switch {
	case tx.Brand != "" &&
		tx.Brand != "Unknown" &&
		tx.Brand != "General":
		name = tx.Brand

	case tx.Description != "":
		name = tx.Description

	case tx.SubCategory != "":
		name = tx.SubCategory

	default:
		name = tx.Category
	}

	sb.WriteString(fmt.Sprintf("%s %s\n",
		categoryIcon(tx.Category),
		name,
	))

	sb.WriteString(fmt.Sprintf("฿%.0f\n", tx.Amount))

	// Daily summary
	if tx.Type == model.Income {
		sb.WriteString(
			fmt.Sprintf("\n💰 รายรับวันนี้ ฿%.0f\n", todayIncome),
		)

		sb.WriteString(
			fmt.Sprintf("💸 รายจ่ายวันนี้ ฿%.0f\n", todayExpense),
		)

		sb.WriteString(
			fmt.Sprintf("✨ คงเหลือ ฿%.0f\n",
				todayIncome-todayExpense,
			),
		)
	} else {
		sb.WriteString(
			fmt.Sprintf("\n💸 วันนี้ใช้ไปแล้ว ฿%.0f\n",
				todayExpense,
			),
		)
	}

	// Alert
	if len(result.Alerts) > 0 {
		sb.WriteString("\n⚠️ ")
		sb.WriteString(result.Alerts[0].Message)
		sb.WriteString("\n")
	}

	// Recommendation
	if len(result.Recommendations) > 0 {
		sb.WriteString("\n💡 ")
		sb.WriteString(result.Recommendations[0].Title)
		sb.WriteString("\n")
	}

	return sb.String()
}

// ─────────────────────────────────────────────────────────────
// Icon helpers
// ─────────────────────────────────────────────────────────────

func categoryIcon(category string) string {
	icons := map[string]string{
		"Food & Beverage": "🍜",
		"Transport":       "🚗",
		"Shopping":        "🛍",
		"Bill":            "📄",
		"Health":          "💊",
		"Entertainment":   "🎮",
		"Salary & Income": "💼",
		"Other":           "📦",
	}
	if icon, ok := icons[category]; ok {
		return icon
	}
	return "📦"
}

func behaviorTagIcon(tag string) string {
	icons := map[string]string{
		"impulse":   "⚡",
		"necessity": "✅",
		"social":    "👥",
		"treat":     "🎁",
		"recurring": "🔄",
	}
	if icon, ok := icons[tag]; ok {
		return icon
	}
	return "🏷"
}
