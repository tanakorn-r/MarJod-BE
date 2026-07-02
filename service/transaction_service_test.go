package service_test

import (
	"finance-chat/agent"
	"finance-chat/model"
	"finance-chat/service"
	"fmt"
	"strings"
	"testing"
	"time"
)

// ── Mocks ─────────────────────────────────────────────────────────────────────

type mockLLM struct {
	response   string
	err        error
	lastPrompt string
}

func (m *mockLLM) Complete(_ string) (string, error) {
	return m.response, m.err
}

func (m *mockLLM) CompleteWithTokenLimit(prompt string, _ int) (string, error) {
	m.lastPrompt = prompt
	return m.response, m.err
}

func (m *mockLLM) CompleteStream(_ string) (<-chan string, error) {
	ch := make(chan string, 1)
	ch <- m.response
	close(ch)
	return ch, m.err
}

type mockLineService struct{}

func (m *mockLineService) VerifySignature(_ []byte, _ string) bool { return true }
func (m *mockLineService) ReplyMessage(_, _ string) error          { return nil }
func (m *mockLineService) VerifyIDToken(_ string) (string, error)  { return "U_mock", nil }

type mockTxRepo struct {
	created  []*model.Transaction
	findByID map[uint]*model.Transaction
	updated  []*model.Transaction
}

func (r *mockTxRepo) Create(t *model.Transaction) error {
	t.ID = uint(len(r.created) + 1)
	t.UserID = model.UserIDOrDefault(t.UserID)
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}
	r.created = append(r.created, t)
	return nil
}

func (r *mockTxRepo) FindAllByUserID(userID string) ([]model.Transaction, error) {
	var list []model.Transaction
	for _, t := range r.created {
		if model.UserIDOrDefault(t.UserID) == model.UserIDOrDefault(userID) {
			list = append(list, *t)
		}
	}
	return list, nil
}

func (r *mockTxRepo) FindByID(userID string, id uint) (*model.Transaction, error) {
	if t, ok := r.findByID[id]; ok && model.UserIDOrDefault(t.UserID) == model.UserIDOrDefault(userID) {
		return t, nil
	}
	return nil, fmt.Errorf("not found")
}

func (r *mockTxRepo) Update(t *model.Transaction) error {
	r.updated = append(r.updated, t)
	return nil
}

func (r *mockTxRepo) FindLatestByUserID(userID string) (*model.Transaction, error) {
	for i := len(r.created) - 1; i >= 0; i-- {
		if model.UserIDOrDefault(r.created[i].UserID) == model.UserIDOrDefault(userID) {
			return r.created[i], nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (r *mockTxRepo) FindTodayByUserID(userID string) ([]model.Transaction, error) {
	return r.FindAllByUserID(userID)
}

func (r *mockTxRepo) FindByCategory(userID, _, _ string, _, _ int) ([]model.Transaction, int64, error) {
	list, _ := r.FindAllByUserID(userID)
	return list, int64(len(list)), nil
}

func (r *mockTxRepo) Delete(_ string, _ uint) error { return nil }

type mockCorrectionRepo struct {
	saved []*model.UserCorrection
}

func (r *mockCorrectionRepo) Save(c *model.UserCorrection) error {
	r.saved = append(r.saved, c)
	return nil
}

func (r *mockCorrectionRepo) FindRecent(userID string, _ int) ([]model.UserCorrection, error) {
	var list []model.UserCorrection
	for _, c := range r.saved {
		if model.UserIDOrDefault(c.UserID) == model.UserIDOrDefault(userID) {
			list = append(list, *c)
		}
	}
	return list, nil
}

func (r *mockCorrectionRepo) FindAll(userID string) ([]model.UserCorrection, error) {
	return r.FindRecent(userID, len(r.saved))
}

func (r *mockCorrectionRepo) Delete(_ string, _ uint) error {
	return nil
}

type mockProfileRepo struct{}

func (r *mockProfileRepo) Save(_ *model.BehaviorProfile) error { return nil }
func (r *mockProfileRepo) FindLatestByUserID(_ string) (*model.BehaviorProfile, error) {
	return nil, fmt.Errorf("not found")
}

type mockPlanRepo struct{}

func (r *mockPlanRepo) FindByUserID(_ string) (*model.UserPlan, error) {
	// Return a default free plan for tests
	return &model.UserPlan{
		UserID:     "default",
		Plan:       model.PlanFree,
		StartDate:  time.Now(),
		ExpiryDate: time.Now().Add(30 * 24 * time.Hour),
	}, nil
}

func (r *mockPlanRepo) Upsert(_ *model.UserPlan) error { return nil }

// newService creates a fresh service instance bypassing the singleton for tests.
func newService(llm agent.LLMClient, txRepo *mockTxRepo, corrRepo *mockCorrectionRepo) service.TransactionService {
	agentDeps := agent.AgentDeps{
		LLM:         llm,
		LineService: &mockLineService{},
		TxRepo:      txRepo,
		ProfileRepo: &mockProfileRepo{},
		PlanRepo:    &mockPlanRepo{},
	}
	return service.NewTransactionServiceDirect(txRepo, corrRepo, &mockProfileRepo{}, &mockPlanRepo{}, agentDeps)
}

// ── Chat tests ────────────────────────────────────────────────────────────────

func TestChat_ExpenseSavedCorrectly(t *testing.T) {
	llm := &mockLLM{response: `{
		"raw_message": "starbucks 180 baht",
		"type": "expense",
		"amount": 180,
		"category": "Food & Beverage",
		"sub_category": "Coffee",
		"brand": "Starbucks",
		"description": "Latte at Starbucks",
		"behavior_tag": "treat",
		"logic_gate": "Verified: Starbucks → Food & Beverage.",
		"confidence": 90
	}`}
	txRepo := &mockTxRepo{}
	svc := newService(llm, txRepo, &mockCorrectionRepo{})

	result, err := svc.Chat("line-user-1", "starbucks 180 baht")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tx := result.Transaction
	if tx.Type != model.Expense {
		t.Errorf("type: want expense, got %s", tx.Type)
	}
	if tx.Amount != 180 {
		t.Errorf("amount: want 180, got %v", tx.Amount)
	}
	if tx.Brand != "Starbucks" {
		t.Errorf("brand: want Starbucks, got %s", tx.Brand)
	}
	if tx.Category != "Food & Beverage" {
		t.Errorf("category: want Food & Beverage, got %s", tx.Category)
	}
	if tx.BehaviorTag != "treat" {
		t.Errorf("behavior_tag: want treat, got %s", tx.BehaviorTag)
	}
	if tx.UserID != "line-user-1" {
		t.Errorf("user_id: want line-user-1, got %s", tx.UserID)
	}
	if len(txRepo.created) != 1 {
		t.Errorf("expected 1 saved transaction, got %d", len(txRepo.created))
	}
}

func TestChat_IncomeSavedCorrectly(t *testing.T) {
	llm := &mockLLM{response: `{"raw_message":"เงินเดือน 50000","type":"income","amount":50000,"category":"Salary & Income","sub_category":"Salary","brand":"Company","description":"Monthly salary","behavior_tag":"recurring","logic_gate":"Verified: เงินเดือน → income.","confidence":99}`}
	txRepo := &mockTxRepo{}
	svc := newService(llm, txRepo, &mockCorrectionRepo{})

	result, err := svc.Chat("default", "เงินเดือน 50000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tx := result.Transaction
	if tx.Type != model.Income {
		t.Errorf("type: want income, got %s", tx.Type)
	}
	if tx.Amount != 50000 {
		t.Errorf("amount: want 50000, got %v", tx.Amount)
	}
}

func TestChat_UnknownType_ReturnsError(t *testing.T) {
	// With strict validation, an invalid type now returns an error from Parse()
	llm := &mockLLM{response: `{"raw_message":"something 100","type":"unknown","amount":100,"category":"Other","sub_category":"General","brand":"General","description":"test","behavior_tag":"necessity","logic_gate":"Flagged: unknown type.","confidence":50}`}
	txRepo := &mockTxRepo{}
	svc := newService(llm, txRepo, &mockCorrectionRepo{})

	_, err := svc.Chat("default", "something 100")
	if err == nil {
		t.Fatal("expected error for invalid type 'unknown', got nil")
	}
}

func TestChat_LLMError_ReturnsError(t *testing.T) {
	llm := &mockLLM{err: fmt.Errorf("connection refused")}
	svc := newService(llm, &mockTxRepo{}, &mockCorrectionRepo{})

	_, err := svc.Chat("default", "test")
	if err == nil {
		t.Fatal("expected error when LLM fails, got nil")
	}
}

func TestChat_BadLLMResponse_ReturnsError(t *testing.T) {
	llm := &mockLLM{response: "I cannot understand this."}
	svc := newService(llm, &mockTxRepo{}, &mockCorrectionRepo{})

	_, err := svc.Chat("default", "test")
	if err == nil {
		t.Fatal("expected error for non-JSON LLM response, got nil")
	}
}

// ── Correct tests ─────────────────────────────────────────────────────────────

func TestCorrect_UpdatesFieldsAndSavesCorrection(t *testing.T) {
	existing := &model.Transaction{
		ID:          1,
		RawMessage:  "starbucks 180",
		Type:        model.Expense,
		Amount:      180,
		Category:    "Food & Beverage",
		SubCategory: "Coffee",
		Brand:       "Unknown",
		BehaviorTag: "daily",
	}
	txRepo := &mockTxRepo{findByID: map[uint]*model.Transaction{1: existing}}
	corrRepo := &mockCorrectionRepo{}
	svc := newService(&mockLLM{}, txRepo, corrRepo)

	err := svc.Correct("default", 1, "", "", "Starbucks", "cafe")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if existing.Brand != "Starbucks" {
		t.Errorf("brand: want Starbucks, got %s", existing.Brand)
	}
	if existing.BehaviorTag != "cafe" {
		t.Errorf("behavior_tag: want cafe, got %s", existing.BehaviorTag)
	}
	// Category unchanged since we passed ""
	if existing.Category != "Food & Beverage" {
		t.Errorf("category should be unchanged, got %s", existing.Category)
	}
	if len(corrRepo.saved) != 1 {
		t.Errorf("expected 1 correction saved, got %d", len(corrRepo.saved))
	}
	if corrRepo.saved[0].Brand != "Starbucks" {
		t.Errorf("correction brand: want Starbucks, got %s", corrRepo.saved[0].Brand)
	}
}

func TestCorrect_NotFound_ReturnsError(t *testing.T) {
	txRepo := &mockTxRepo{findByID: map[uint]*model.Transaction{}}
	svc := newService(&mockLLM{}, txRepo, &mockCorrectionRepo{})

	err := svc.Correct("default", 99, "Food & Beverage", "Coffee", "Starbucks", "cafe")
	if err == nil {
		t.Fatal("expected error for non-existent transaction, got nil")
	}
}

// ── Summary tests ─────────────────────────────────────────────────────────────

func TestSummary_CalculatesCorrectly(t *testing.T) {
	txRepo := &mockTxRepo{
		created: []*model.Transaction{
			{Type: model.Income, Amount: 50000},
			{Type: model.Expense, Amount: 180},
			{Type: model.Expense, Amount: 320},
		},
	}
	svc := newService(&mockLLM{}, txRepo, &mockCorrectionRepo{})

	sum, err := svc.Summary("default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sum.TotalIncome != 50000 {
		t.Errorf("total_income: want 50000, got %v", sum.TotalIncome)
	}
	if sum.TotalExpense != 500 {
		t.Errorf("total_expense: want 500, got %v", sum.TotalExpense)
	}
	if sum.Balance != 49500 {
		t.Errorf("balance: want 49500, got %v", sum.Balance)
	}
}

func TestSummary_Empty_ReturnsZeros(t *testing.T) {
	svc := newService(&mockLLM{}, &mockTxRepo{}, &mockCorrectionRepo{})

	sum, err := svc.Summary("default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sum.Balance != 0 || sum.TotalIncome != 0 || sum.TotalExpense != 0 {
		t.Errorf("empty summary should be all zeros, got %+v", sum)
	}
}

// ── Analytics AI insight tests ──────────────────────────────────────────────

func TestAnalytics_DoesNotCallLLM(t *testing.T) {
	month := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	txRepo := &mockTxRepo{created: []*model.Transaction{
		{Type: model.Income, Amount: 50000, CreatedAt: month},
	}}
	llm := &mockLLM{err: fmt.Errorf("must not be called")}
	svc := newService(llm, txRepo, &mockCorrectionRepo{})

	if _, err := svc.GetAnalytics("default", month); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if llm.lastPrompt != "" {
		t.Error("GET analytics must not invoke the LLM")
	}
}

func TestAnalytics_IsolatedByUserID(t *testing.T) {
	month := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	txRepo := &mockTxRepo{created: []*model.Transaction{
		{UserID: "user-a", Type: model.Income, Amount: 10000, CreatedAt: month},
		{UserID: "user-a", Type: model.Expense, Amount: 2500, Category: "Food", CreatedAt: month},
		{UserID: "user-b", Type: model.Income, Amount: 900000, CreatedAt: month},
		{UserID: "user-b", Type: model.Expense, Amount: 800000, Category: "Travel", CreatedAt: month},
	}}
	svc := newService(&mockLLM{}, txRepo, &mockCorrectionRepo{})

	dashboard, err := svc.GetAnalytics("user-a", month)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dashboard.MonthlySummary.TotalIncome != 10000 || dashboard.MonthlySummary.TotalExpense != 2500 {
		t.Fatalf("analytics leaked another user's data: %+v", dashboard.MonthlySummary)
	}
}

func TestAnalyticsInsight_GeneratesStructuredPersonalFinanceInsight(t *testing.T) {
	month := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	txRepo := &mockTxRepo{created: []*model.Transaction{
		{RawMessage: "private salary note", Type: model.Income, Amount: 50000, CreatedAt: month},
		{RawMessage: "private coffee note", Type: model.Expense, Amount: 5000, Category: "Food", BehaviorTag: "treat", CreatedAt: month},
	}}
	llm := &mockLLM{response: `{
		"status":"ready",
		"health":"healthy",
		"headline":"Strong monthly surplus",
		"summary":"Spending is well below income. Treat spending is the main discretionary pattern.",
		"key_findings":["A large share of income remains unspent"],
		"recommendations":[{"priority":"medium","title":"Set a savings target","action":"Move part of the surplus to savings","rationale":"It turns the surplus into progress"}]
	}`}
	svc := newService(llm, txRepo, &mockCorrectionRepo{})

	insight, err := svc.GetAnalyticsInsight("default", month)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if insight.Status != "ready" || insight.Health != "healthy" {
		t.Fatalf("unexpected AI insight: %+v", insight)
	}
	if insight.Headline != "Strong monthly surplus" {
		t.Errorf("unexpected headline: %s", insight.Headline)
	}
	if strings.Contains(llm.lastPrompt, "private salary note") || strings.Contains(llm.lastPrompt, "private coffee note") {
		t.Error("analytics prompt must not include raw transaction messages")
	}
}

func TestAnalyticsInsight_LLMFailureReturnsUnavailable(t *testing.T) {
	month := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	txRepo := &mockTxRepo{created: []*model.Transaction{
		{Type: model.Income, Amount: 10000, CreatedAt: month},
		{Type: model.Expense, Amount: 2500, Category: "Food", CreatedAt: month},
	}}
	svc := newService(&mockLLM{err: fmt.Errorf("openai unavailable")}, txRepo, &mockCorrectionRepo{})

	insight, err := svc.GetAnalyticsInsight("default", month)
	if err != nil {
		t.Fatalf("insight should gracefully handle an LLM failure: %v", err)
	}
	if insight.Status != "unavailable" {
		t.Errorf("AI status: want unavailable, got %s", insight.Status)
	}
}
