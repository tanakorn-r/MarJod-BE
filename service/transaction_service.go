package service

import (
	"finance-chat/agent"
	"finance-chat/model"
	"finance-chat/repository"
	"fmt"
	"sort"
	"sync"
	"time"
)

type TransactionService interface {
	Chat(message string) (*agent.PipelineResult, error)
	ChatStream(message string) (<-chan string, <-chan *agent.PipelineResult, <-chan error)
	Correct(transactionID uint, category, subCategory, brand, behaviorTag string) error
	List() ([]model.Transaction, error)
	GetByID(id uint) (*model.Transaction, error)
	Delete(id uint) error
	Summary() (*Summary, error)
	GetAnalytics() (*AnalyticsDashboard, error)
	ListCorrections() ([]model.UserCorrection, error)
	DeleteCorrection(id uint) error
}

type Summary struct {
	TotalIncome  float64 `json:"total_income"`
	TotalExpense float64 `json:"total_expense"`
	Balance      float64 `json:"balance"`
}

// Analytics structures
type Alert struct {
	Type    string `json:"type"`    // "warning", "info", "success"
	Message string `json:"message"`
}

type MonthlySummary struct {
	Month        string  `json:"month"`         // e.g. "April 2025"
	TotalIncome  float64 `json:"total_income"`
	TotalExpense float64 `json:"total_expense"`
	Balance      float64 `json:"balance"`
	Alert        *Alert  `json:"alert,omitempty"`
}

type HighlightedTransaction struct {
	ID          uint    `json:"id"`
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
	Category    string  `json:"category"`
	Highlight   string  `json:"highlight"` // "most_spent", "unusual", "recurring"
}

type DailySpending struct {
	Date   string  `json:"date"`   // e.g. "2025-04-01"
	Income float64 `json:"income"`
	Expense float64 `json:"expense"`
}

type CategoryBreakdown struct {
	Rank        int     `json:"rank"`
	Category    string  `json:"category"`
	SubCategory string  `json:"sub_category"`
	Amount      float64 `json:"amount"`
	Percentage  float64 `json:"percentage"`
}

type ComparisonMetric struct {
	Label      string  `json:"label"`       // "vs last month", "vs last week"
	Amount     float64 `json:"amount"`
	Percentage float64 `json:"percentage"`  // positive or negative
}

type IrregularPurchase struct {
	Description string  `json:"description"`
	Brand       string  `json:"brand"`
	Amount      float64 `json:"amount"`
	Date        string  `json:"date"`
	Reason      string  `json:"reason"` // why it's irregular
}

type DayOfWeekSpending struct {
	Day    string  `json:"day"`    // "Mon", "Tue", etc.
	Amount float64 `json:"amount"`
}

type BehaviorInsight struct {
	Icon        string `json:"icon"`        // emoji or icon name
	Title       string `json:"title"`
	Description string `json:"description"`
	Metric      string `json:"metric"`      // e.g. "3 times this week"
}

type AnalyticsDashboard struct {
	MonthlySummary        MonthlySummary            `json:"monthly_summary"`
	RecentHighlights      []HighlightedTransaction  `json:"recent_highlights"`
	DailySpending         []DailySpending           `json:"daily_spending"`
	CategoryBreakdown     []CategoryBreakdown       `json:"category_breakdown"`
	SavingsComparison     ComparisonMetric          `json:"savings_comparison"`
	ExpenseComparison     ComparisonMetric          `json:"expense_comparison"`
	IrregularPurchases    []IrregularPurchase       `json:"irregular_purchases"`
	SpendingByDayOfWeek   []DayOfWeekSpending       `json:"spending_by_day_of_week"`
	BehaviorInsights      []BehaviorInsight         `json:"behavior_insights"`
}

type transactionService struct {
	repo        repository.TransactionRepository
	correction  repository.CorrectionRepository
	profileRepo repository.BehaviorProfileRepository
	planRepo    repository.UserPlanRepository
	agentDeps   agent.AgentDeps
}

var (
	txServiceInstance TransactionService
	txServiceOnce     sync.Once
)

func NewTransactionService(
	repo repository.TransactionRepository,
	correction repository.CorrectionRepository,
	profileRepo repository.BehaviorProfileRepository,
	planRepo repository.UserPlanRepository,
	agentDeps agent.AgentDeps,
) TransactionService {
	txServiceOnce.Do(func() {
		txServiceInstance = &transactionService{
			repo:        repo,
			correction:  correction,
			profileRepo: profileRepo,
			planRepo:    planRepo,
			agentDeps:   agentDeps,
		}
	})
	return txServiceInstance
}

// NewTransactionServiceDirect creates a new instance without the singleton —
// intended for use in tests only.
func NewTransactionServiceDirect(
	repo repository.TransactionRepository,
	correction repository.CorrectionRepository,
	profileRepo repository.BehaviorProfileRepository,
	planRepo repository.UserPlanRepository,
	agentDeps agent.AgentDeps,
) TransactionService {
	return &transactionService{
		repo:        repo,
		correction:  correction,
		profileRepo: profileRepo,
		planRepo:    planRepo,
		agentDeps:   agentDeps,
	}
}

func (s *transactionService) Chat(message string) (*agent.PipelineResult, error) {
	// Look up user's plan
	plan, err := s.planRepo.FindByUserID("default")
	if err != nil {
		return nil, fmt.Errorf("failed to load user plan: %w", err)
	}

	effectivePlan := plan.EffectivePlan()

	// Build pipeline for the user's plan
	pipeline := agent.BuildPipeline(effectivePlan, s.agentDeps)

	// Build agent context
	ctx := &agent.AgentContext{
		RawMessage:  message,
		UserPlan:    effectivePlan,
		LLM:         s.agentDeps.LLM,
		LineService: s.agentDeps.LineService,
		LineUserID:  "", // No LINE user ID for direct API calls
	}

	// Run the pipeline
	result, err := pipeline.Run(ctx)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *transactionService) ChatStream(message string) (<-chan string, <-chan *agent.PipelineResult, <-chan error) {
	tokens := make(chan string, 32)
	done := make(chan *agent.PipelineResult, 1)
	errc := make(chan error, 1)

	// For now, ChatStream doesn't support streaming through the pipeline
	// We'll run the full pipeline and return the result
	// TODO: Implement true streaming through the agent pipeline

	go func() {
		defer close(tokens)
		defer close(done)
		defer close(errc)

		result, err := s.Chat(message)
		if err != nil {
			errc <- err
			return
		}

		done <- result
	}()

	return tokens, done, errc
}

func (s *transactionService) Correct(transactionID uint, category, subCategory, brand, behaviorTag string) error {
	tx, err := s.repo.FindByID(transactionID)
	if err != nil {
		return fmt.Errorf("transaction not found: %w", err)
	}

	// Apply corrections — only update non-empty fields
	if category != "" {
		tx.Category = category
	}
	if subCategory != "" {
		tx.SubCategory = subCategory
	}
	if brand != "" {
		tx.Brand = brand
	}
	if behaviorTag != "" {
		tx.BehaviorTag = behaviorTag
	}

	if err := s.repo.Update(tx); err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	// Persist as a correction example for future prompts
	c := &model.UserCorrection{
		RawMessage:  tx.RawMessage,
		Category:    tx.Category,
		SubCategory: tx.SubCategory,
		Brand:       tx.Brand,
		BehaviorTag: tx.BehaviorTag,
	}
	return s.correction.Save(c)
}

func (s *transactionService) List() ([]model.Transaction, error) {
	return s.repo.FindAll()
}

func (s *transactionService) Delete(id uint) error {
	return s.repo.Delete(id)
}

func (s *transactionService) Summary() (*Summary, error) {
	list, err := s.repo.FindAll()
	if err != nil {
		return nil, err
	}

	sum := &Summary{}
	for _, t := range list {
		if t.Type == model.Income {
			sum.TotalIncome += t.Amount
		} else {
			sum.TotalExpense += t.Amount
		}
	}
	sum.Balance = sum.TotalIncome - sum.TotalExpense
	return sum, nil
}

func (s *transactionService) GetByID(id uint) (*model.Transaction, error) {
	return s.repo.FindByID(id)
}

func (s *transactionService) GetAnalytics() (*AnalyticsDashboard, error) {
	list, err := s.repo.FindAll()
	if err != nil {
		return nil, err
	}

	// Get latest behavior profile for insights
	profile, _ := s.profileRepo.FindLatestByUserID("default")
	
	dashboard := &AnalyticsDashboard{
		MonthlySummary: s.calculateMonthlySummary(list),
		RecentHighlights: s.getRecentHighlights(list),
		DailySpending: s.calculateDailySpending(list),
		CategoryBreakdown: s.calculateCategoryBreakdown(list),
		SavingsComparison: s.calculateSavingsComparison(list),
		ExpenseComparison: s.calculateExpenseComparison(list),
		IrregularPurchases: s.findIrregularPurchases(list),
		SpendingByDayOfWeek: s.calculateSpendingByDayOfWeek(list),
		BehaviorInsights: s.generateBehaviorInsights(list, profile),
	}

	return dashboard, nil
}

func (s *transactionService) calculateMonthlySummary(transactions []model.Transaction) MonthlySummary {
	now := time.Now()
	currentMonth := now.Format("January 2006")
	
	var totalIncome, totalExpense float64
	
	// Only count transactions from current month
	for _, t := range transactions {
		if t.CreatedAt.Month() == now.Month() && t.CreatedAt.Year() == now.Year() {
			if t.Type == model.Income {
				totalIncome += t.Amount
			} else {
				totalExpense += t.Amount
			}
		}
	}
	
	balance := totalIncome - totalExpense
	
	// Generate alert based on spending ratio
	var alert *Alert
	if totalIncome > 0 && totalExpense > totalIncome*0.9 {
		alert = &Alert{
			Type:    "warning",
			Message: "You've spent 90% of your income this month. Consider reducing expenses if you want to save more.",
		}
	} else if balance > totalIncome*0.3 && totalIncome > 0 {
		alert = &Alert{
			Type:    "success",
			Message: "Great job! You're saving over 30% of your income this month.",
		}
	}
	
	return MonthlySummary{
		Month:        currentMonth,
		TotalIncome:  totalIncome,
		TotalExpense: totalExpense,
		Balance:      balance,
		Alert:        alert,
	}
}

func (s *transactionService) getRecentHighlights(transactions []model.Transaction) []HighlightedTransaction {
	highlights := []HighlightedTransaction{}
	
	// Find top 4 most expensive transactions
	if len(transactions) > 0 {
		// Simple approach: take first 4 expenses
		count := 0
		for _, t := range transactions {
			if t.Type == model.Expense && count < 4 {
				highlight := "most_spent"
				if t.BehaviorTag == "impulse" {
					highlight = "unusual"
				} else if t.BehaviorTag == "recurring" {
					highlight = "recurring"
				}
				
				highlights = append(highlights, HighlightedTransaction{
					ID:          t.ID,
					Description: t.Description,
					Amount:      t.Amount,
					Category:    t.Category,
					Highlight:   highlight,
				})
				count++
			}
		}
	}
	
	return highlights
}

func (s *transactionService) calculateDailySpending(transactions []model.Transaction) []DailySpending {
	dailyMap := make(map[string]*DailySpending)
	
	for _, t := range transactions {
		dateStr := t.CreatedAt.Format("2006-01-02")
		if _, exists := dailyMap[dateStr]; !exists {
			dailyMap[dateStr] = &DailySpending{Date: dateStr}
		}
		
		if t.Type == model.Income {
			dailyMap[dateStr].Income += t.Amount
		} else {
			dailyMap[dateStr].Expense += t.Amount
		}
	}
	
	result := []DailySpending{}
	for _, ds := range dailyMap {
		result = append(result, *ds)
	}
	
	// Sort by date ascending
	sort.Slice(result, func(i, j int) bool {
		return result[i].Date < result[j].Date
	})
	
	return result
}

func (s *transactionService) calculateCategoryBreakdown(transactions []model.Transaction) []CategoryBreakdown {
	categoryMap := make(map[string]float64)
	var totalExpense float64
	
	for _, t := range transactions {
		if t.Type == model.Expense {
			key := t.Category
			categoryMap[key] += t.Amount
			totalExpense += t.Amount
		}
	}
	
	breakdown := []CategoryBreakdown{}
	for category, amount := range categoryMap {
		percentage := 0.0
		if totalExpense > 0 {
			percentage = (amount / totalExpense) * 100
		}
		
		breakdown = append(breakdown, CategoryBreakdown{
			Rank:        0, // Will be assigned after sorting
			Category:    category,
			SubCategory: "", // Could aggregate sub-categories
			Amount:      amount,
			Percentage:  percentage,
		})
	}
	
	// Sort by amount descending
	sort.Slice(breakdown, func(i, j int) bool {
		return breakdown[i].Amount > breakdown[j].Amount
	})
	
	// Assign ranks after sorting
	for i := range breakdown {
		breakdown[i].Rank = i + 1
	}
	
	return breakdown
}

func (s *transactionService) calculateSavingsComparison(transactions []model.Transaction) ComparisonMetric {
	now := time.Now()
	currentMonth := now.Month()
	currentYear := now.Year()
	
	lastMonth := currentMonth - 1
	lastMonthYear := currentYear
	if lastMonth == 0 {
		lastMonth = 12
		lastMonthYear--
	}
	
	var currentSavings, lastMonthSavings float64
	var currentIncome, currentExpense, lastIncome, lastExpense float64
	
	for _, t := range transactions {
		if t.CreatedAt.Month() == currentMonth && t.CreatedAt.Year() == currentYear {
			if t.Type == model.Income {
				currentIncome += t.Amount
			} else {
				currentExpense += t.Amount
			}
		} else if t.CreatedAt.Month() == lastMonth && t.CreatedAt.Year() == lastMonthYear {
			if t.Type == model.Income {
				lastIncome += t.Amount
			} else {
				lastExpense += t.Amount
			}
		}
	}
	
	currentSavings = currentIncome - currentExpense
	lastMonthSavings = lastIncome - lastExpense
	
	diff := currentSavings - lastMonthSavings
	percentage := 0.0
	if lastMonthSavings != 0 {
		percentage = (diff / lastMonthSavings) * 100
	}
	
	return ComparisonMetric{
		Label:      "vs last month",
		Amount:     diff,
		Percentage: percentage,
	}
}

func (s *transactionService) calculateExpenseComparison(transactions []model.Transaction) ComparisonMetric {
	now := time.Now()
	currentMonth := now.Month()
	currentYear := now.Year()
	
	lastMonth := currentMonth - 1
	lastMonthYear := currentYear
	if lastMonth == 0 {
		lastMonth = 12
		lastMonthYear--
	}
	
	var currentExpense, lastMonthExpense float64
	
	for _, t := range transactions {
		if t.Type == model.Expense {
			if t.CreatedAt.Month() == currentMonth && t.CreatedAt.Year() == currentYear {
				currentExpense += t.Amount
			} else if t.CreatedAt.Month() == lastMonth && t.CreatedAt.Year() == lastMonthYear {
				lastMonthExpense += t.Amount
			}
		}
	}
	
	diff := currentExpense - lastMonthExpense
	percentage := 0.0
	if lastMonthExpense != 0 {
		percentage = (diff / lastMonthExpense) * 100
	}
	
	return ComparisonMetric{
		Label:      "vs last month",
		Amount:     diff,
		Percentage: percentage,
	}
}

func (s *transactionService) findIrregularPurchases(transactions []model.Transaction) []IrregularPurchase {
	irregular := []IrregularPurchase{}
	
	// Mock data - AI would analyze patterns
	for _, t := range transactions {
		if t.BehaviorTag == "impulse" || t.Amount > 1000 {
			irregular = append(irregular, IrregularPurchase{
				Description: t.Description,
				Brand:       t.Brand,
				Amount:      t.Amount,
				Date:        t.CreatedAt.Format("Jan 02"),
				Reason:      "Higher than usual spending",
			})
			
			if len(irregular) >= 4 {
				break
			}
		}
	}
	
	return irregular
}

func (s *transactionService) calculateSpendingByDayOfWeek(transactions []model.Transaction) []DayOfWeekSpending {
	days := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	dayMap := make(map[string]float64)
	
	for _, t := range transactions {
		if t.Type == model.Expense {
			dayName := t.CreatedAt.Weekday().String()[:3]
			dayMap[dayName] += t.Amount
		}
	}
	
	result := []DayOfWeekSpending{}
	for _, day := range days {
		result = append(result, DayOfWeekSpending{
			Day:    day,
			Amount: dayMap[day],
		})
	}
	
	return result
}

func (s *transactionService) generateBehaviorInsights(transactions []model.Transaction, profile *model.BehaviorProfile) []BehaviorInsight {
	insights := []BehaviorInsight{}
	
	// If no profile data, return generic insights
	if profile == nil || profile.InsufficientData {
		insights = append(insights, BehaviorInsight{
			Icon:        "📊",
			Title:       "Building Your Profile",
			Description: "Keep logging transactions to unlock personalized insights about your spending patterns.",
			Metric:      fmt.Sprintf("%d transactions logged", len(transactions)),
		})
		return insights
	}
	
	// Insight 1: Dominant spending category
	if profile.DominantCategory != "" {
		insights = append(insights, BehaviorInsight{
			Icon:        "🎯",
			Title:       "Top Spending Category",
			Description: fmt.Sprintf("Most of your spending goes to %s. This is your dominant category.", profile.DominantCategory),
			Metric:      profile.DominantCategory,
		})
	}
	
	// Insight 2: Impulse buying pattern
	if profile.ImpulseFrequency > 0 {
		icon := "⚠️"
		title := "Impulse Buying Alert"
		description := fmt.Sprintf("You've made %d impulse purchases recently. Consider waiting 24 hours before buying.", profile.ImpulseFrequency)
		metric := fmt.Sprintf("%d impulse buys", profile.ImpulseFrequency)
		
		if profile.ImpulseFrequency >= 5 {
			icon = "🚨"
			title = "High Impulse Buying"
			description = "Your impulse purchases are frequent. Try the 24-hour rule: wait a day before buying non-essentials."
		}
		
		insights = append(insights, BehaviorInsight{
			Icon:        icon,
			Title:       title,
			Description: description,
			Metric:      metric,
		})
	}
	
	// Insight 3: Luxury drift detection
	if profile.LuxuryDriftDetected {
		insights = append(insights, BehaviorInsight{
			Icon:        "💎",
			Title:       "Luxury Drift Detected",
			Description: fmt.Sprintf("Your luxury spending is trending up (index: %.1f). Premium purchases are becoming more frequent.", profile.LuxuryDriftIndex),
			Metric:      fmt.Sprintf("Drift index: %.1f", profile.LuxuryDriftIndex),
		})
	}
	
	// Insight 4: Top brands (decode from JSON string)
	if profile.TopBrands != "" {
		// TopBrands is stored as JSON string, decode it
		// For simplicity, just display the raw string for now
		insights = append(insights, BehaviorInsight{
			Icon:        "🏷️",
			Title:       "Favorite Brands",
			Description: fmt.Sprintf("You frequently shop at certain brands. Consider looking for alternatives to save money."),
			Metric:      profile.TopBrands,
		})
	}
	
	// Insight 5: Recurring expenses
	recurringCount := 0
	for _, t := range transactions {
		if t.BehaviorTag == "recurring" {
			recurringCount++
		}
	}
	if recurringCount > 0 {
		insights = append(insights, BehaviorInsight{
			Icon:        "🔄",
			Title:       "Recurring Expenses",
			Description: "You have regular subscriptions and recurring payments. Review them periodically to avoid paying for unused services.",
			Metric:      fmt.Sprintf("%d recurring payments", recurringCount),
		})
	}
	
	return insights
}

func (s *transactionService) ListCorrections() ([]model.UserCorrection, error) {
	return s.correction.FindAll()
}

func (s *transactionService) DeleteCorrection(id uint) error {
	return s.correction.Delete(id)
}
