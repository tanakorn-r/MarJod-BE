package service

import (
	"errors"
	"finance-chat/model"
	"finance-chat/repository"
	"finance-chat/timeutil"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// QuestProgress is the live-computed state of one quest, evaluated fresh
// from real transactions every time — nothing about progress is stored.
type QuestProgress struct {
	Current float64
	Target  float64
	Unit    string
	Done    bool
}

// QuestTemplate is one preset quest definition. Evaluate computes progress
// for a given user's transaction history at a point in time. Title can
// embed live values (e.g. last month's real savings rate), so it's computed
// alongside progress rather than stored as a static string.
type QuestTemplate struct {
	Key        model.QuestTemplateKey
	Name       string
	Logo       string
	Period     model.QuestPeriod
	Difficulty model.QuestDifficulty
	XP         int
	Accent     string
	Evaluate   func(all []model.Transaction, now time.Time) (QuestProgress, string)
}

func templateByKey(templates map[model.QuestTemplateKey]QuestTemplate, key model.QuestTemplateKey) (QuestTemplate, bool) {
	t, ok := templates[key]
	return t, ok
}

// startOfWeek returns midnight on the Monday of now's Thailand-time week.
func startOfWeek(now time.Time) time.Time {
	return timeutil.StartOfWeek(now)
}

func startOfDay(now time.Time) time.Time {
	return timeutil.StartOfDay(now)
}

func endOfDay(now time.Time) time.Time {
	return timeutil.StartOfNextDay(now)
}

func endOfWeek(now time.Time) time.Time {
	return timeutil.StartOfNextWeek(now)
}

func questExpiry(period model.QuestPeriod, now time.Time) time.Time {
	if period == model.QuestPeriodDaily {
		return endOfDay(now)
	}
	return endOfWeek(now)
}

func questTemplatesFromPresets(presets []model.QuestPreset) []QuestTemplate {
	templates := make([]QuestTemplate, 0, len(presets))
	for _, preset := range presets {
		evaluate, ok := evaluatorForPreset(preset)
		if !ok {
			continue
		}
		name := strings.TrimSpace(preset.Name)
		templates = append(templates, QuestTemplate{
			Key:        preset.Key,
			Name:       name,
			Logo:       preset.Logo,
			Period:     preset.Period,
			Difficulty: preset.Difficulty,
			XP:         preset.XP,
			Accent:     preset.Accent,
			Evaluate: func(all []model.Transaction, now time.Time) (QuestProgress, string) {
				progress, fallbackTitle := evaluate(all, now)
				if name != "" {
					return progress, name
				}
				return progress, fallbackTitle
			},
		})
	}
	return templates
}

func evaluatorForPreset(preset model.QuestPreset) (func([]model.Transaction, time.Time) (QuestProgress, string), bool) {
	target := preset.Target
	if target <= 0 && preset.RuleType != model.QuestRuleNoImpulse && preset.RuleType != model.QuestRuleBeatLastMonthSavingsRate {
		target = 1
	}

	switch preset.RuleType {
	case model.QuestRuleLogTransactionCount:
		return evalLogCountForPeriod(preset.Period, int(target)), true
	case model.QuestRuleSpendCap:
		return evalSpendCapForPeriod(preset.Period, target), true
	case model.QuestRuleNoImpulse:
		return evalNoImpulseForPeriod(preset.Period), true
	case model.QuestRuleNoSpendDays:
		return evalNoSpendDays(int(target)), true
	case model.QuestRuleCategorySpendCap:
		return evalCategoryCap(preset.Category, preset.SubCategory, target), true
	case model.QuestRuleCoffeeSpendCap:
		return evalCoffeeCap(target, preset.Difficulty), true
	case model.QuestRuleBeatLastMonthSavingsRate:
		return evalBeatLastMonthSavingsRate, true
	default:
		return nil, false
	}
}

func periodStart(period model.QuestPeriod, now time.Time) time.Time {
	if period == model.QuestPeriodDaily {
		return startOfDay(now)
	}
	return startOfWeek(now)
}

func evalLogCountForPeriod(period model.QuestPeriod, target int) func([]model.Transaction, time.Time) (QuestProgress, string) {
	return func(all []model.Transaction, now time.Time) (QuestProgress, string) {
		start := periodStart(period, now)
		count := 0
		for _, t := range all {
			if t.CreatedAt.Before(start) || t.CreatedAt.After(now) {
				continue
			}
			count++
		}
		if period == model.QuestPeriodDaily {
			return QuestProgress{Current: float64(count), Target: float64(target), Unit: "count", Done: count >= target}, "Log a transaction today"
		}
		title := fmt.Sprintf("Log %d transactions this week", target)
		return QuestProgress{Current: float64(count), Target: float64(target), Unit: "count", Done: count >= target}, title
	}
}

func evalSpendCapForPeriod(period model.QuestPeriod, cap float64) func([]model.Transaction, time.Time) (QuestProgress, string) {
	return func(all []model.Transaction, now time.Time) (QuestProgress, string) {
		start := periodStart(period, now)
		var sum float64
		count := 0
		for _, t := range all {
			if t.Type != model.Expense || t.CreatedAt.Before(start) || t.CreatedAt.After(now) {
				continue
			}
			count++
			sum += t.Amount
		}
		periodLabel := "today"
		if period == model.QuestPeriodWeekly {
			periodLabel = "this week"
		}
		return QuestProgress{Current: sum, Target: cap, Unit: "thb", Done: count > 0 && sum <= cap}, fmt.Sprintf("Stay under ฿%s %s", formatThb(cap), periodLabel)
	}
}

func evalNoImpulseForPeriod(period model.QuestPeriod) func([]model.Transaction, time.Time) (QuestProgress, string) {
	return func(all []model.Transaction, now time.Time) (QuestProgress, string) {
		start := periodStart(period, now)
		impulseCount := 0
		transactionCount := 0
		for _, t := range all {
			if t.CreatedAt.Before(start) || t.CreatedAt.After(now) {
				continue
			}
			transactionCount++
			if t.Type != model.Expense || t.BehaviorTag != "impulse" {
				continue
			}
			impulseCount++
		}
		title := "Avoid impulse buys today"
		if period == model.QuestPeriodWeekly {
			title = "Zero impulse buys this week"
		}
		return QuestProgress{Current: float64(impulseCount), Target: 0, Unit: "count", Done: transactionCount > 0 && impulseCount == 0}, title
	}
}

func evalLogToday(all []model.Transaction, now time.Time) (QuestProgress, string) {
	dayStart := startOfDay(now)
	count := 0
	for _, t := range all {
		if t.CreatedAt.Before(dayStart) || t.CreatedAt.After(now) {
			continue
		}
		count++
	}
	return QuestProgress{Current: float64(count), Target: 1, Unit: "count", Done: count >= 1}, "Log a transaction today"
}

func evalDailySpendCap(cap float64) func([]model.Transaction, time.Time) (QuestProgress, string) {
	return func(all []model.Transaction, now time.Time) (QuestProgress, string) {
		dayStart := startOfDay(now)
		var sum float64
		count := 0
		for _, t := range all {
			if t.Type != model.Expense || t.CreatedAt.Before(dayStart) || t.CreatedAt.After(now) {
				continue
			}
			count++
			sum += t.Amount
		}
		return QuestProgress{Current: sum, Target: cap, Unit: "thb", Done: count > 0 && sum <= cap}, fmt.Sprintf("Stay under ฿%s today", formatThb(cap))
	}
}

func evalNoImpulseToday(all []model.Transaction, now time.Time) (QuestProgress, string) {
	dayStart := startOfDay(now)
	impulseCount := 0
	transactionCount := 0
	for _, t := range all {
		if t.CreatedAt.Before(dayStart) || t.CreatedAt.After(now) {
			continue
		}
		transactionCount++
		if t.Type != model.Expense || t.BehaviorTag != "impulse" {
			continue
		}
		impulseCount++
	}
	return QuestProgress{Current: float64(impulseCount), Target: 0, Unit: "count", Done: transactionCount > 0 && impulseCount == 0}, "Avoid impulse buys today"
}

func evalNoSpendDays(target int) func([]model.Transaction, time.Time) (QuestProgress, string) {
	return func(all []model.Transaction, now time.Time) (QuestProgress, string) {
		weekStart := startOfWeek(now)
		spendByDay := map[string]float64{}
		for _, t := range all {
			if t.Type != model.Expense || t.CreatedAt.Before(weekStart) || t.CreatedAt.After(now) {
				continue
			}
			spendByDay[timeutil.DateKey(t.CreatedAt)] += t.Amount
		}
		noSpend := 0
		for d := weekStart; !d.After(now); d = d.AddDate(0, 0, 1) {
			if spendByDay[d.Format("2006-01-02")] == 0 {
				noSpend++
			}
		}
		title := fmt.Sprintf("%s no-spend days", numberWord(target))
		return QuestProgress{Current: float64(noSpend), Target: float64(target), Unit: "days", Done: noSpend >= target}, title
	}
}

func evalCategoryCap(category string, label string, cap float64) func([]model.Transaction, time.Time) (QuestProgress, string) {
	return func(all []model.Transaction, now time.Time) (QuestProgress, string) {
		weekStart := startOfWeek(now)
		var sum float64
		count := 0
		for _, t := range all {
			if t.Type != model.Expense || t.Category != category {
				continue
			}
			if t.CreatedAt.Before(weekStart) || t.CreatedAt.After(now) {
				continue
			}
			count++
			sum += t.Amount
		}
		done := count > 0 && sum <= cap
		title := fmt.Sprintf("Keep %s under ฿%s", label, formatThb(cap))
		return QuestProgress{Current: sum, Target: cap, Unit: "thb", Done: done}, title
	}
}

func evalCoffeeCap(cap float64, difficulty model.QuestDifficulty) func([]model.Transaction, time.Time) (QuestProgress, string) {
	return func(all []model.Transaction, now time.Time) (QuestProgress, string) {
		weekStart := startOfWeek(now)
		var sum float64
		count := 0
		for _, t := range all {
			if t.Type != model.Expense || !isCoffeeTransaction(t) {
				continue
			}
			if t.CreatedAt.Before(weekStart) || t.CreatedAt.After(now) {
				continue
			}
			count++
			sum += t.Amount
		}
		done := count > 0 && sum <= cap
		title := fmt.Sprintf("%s coffee quest: spend under ฿%s this week", questDifficultyLabel(difficulty), formatThb(cap))
		return QuestProgress{Current: sum, Target: cap, Unit: "thb", Done: done}, title
	}
}

func isCoffeeTransaction(t model.Transaction) bool {
	text := strings.ToLower(strings.Join([]string{
		t.Category,
		t.SubCategory,
		t.Brand,
		t.Description,
		t.RawMessage,
	}, " "))
	keywords := []string{
		"coffee",
		"cafe",
		"café",
		"starbucks",
		"amazon",
		"latte",
		"espresso",
		"americano",
		"กาแฟ",
	}
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func questDifficultyLabel(difficulty model.QuestDifficulty) string {
	switch difficulty {
	case model.QuestBasic:
		return "Basic"
	case model.QuestAdvanced:
		return "Advanced"
	case model.QuestExpert:
		return "Expert"
	case model.QuestMaster:
		return "Master"
	case model.QuestGrandMaster:
		return "Grand Master"
	default:
		return string(difficulty)
	}
}

func numberWord(n int) string {
	words := map[int]string{1: "One", 2: "Two", 3: "Three", 4: "Four", 5: "Five", 6: "Six", 7: "Seven"}
	if w, ok := words[n]; ok {
		return w
	}
	return fmt.Sprintf("%d", n)
}

func formatThb(amount float64) string {
	return fmt.Sprintf("%.0f", amount)
}

func savingsRateForMonth(all []model.Transaction, ref time.Time) float64 {
	ref = timeutil.InThailand(ref)
	var income, expense float64
	for _, t := range all {
		createdAt := timeutil.InThailand(t.CreatedAt)
		if createdAt.Year() != ref.Year() || createdAt.Month() != ref.Month() {
			continue
		}
		if t.Type == model.Income {
			income += t.Amount
		} else {
			expense += t.Amount
		}
	}
	if income <= 0 {
		return 0
	}
	return (income - expense) / income * 100
}

func evalBeatLastMonthSavingsRate(all []model.Transaction, now time.Time) (QuestProgress, string) {
	thisRate := savingsRateForMonth(all, now)
	lastMonth := now.AddDate(0, -1, 0)
	lastRate := savingsRateForMonth(all, lastMonth)
	title := fmt.Sprintf("Beat %s's %.1f%% save rate", lastMonth.Format("January"), lastRate)
	return QuestProgress{Current: thisRate, Target: lastRate, Unit: "percent", Done: lastRate > 0 && thisRate >= lastRate}, title
}

func evalNoImpulseThisWeek(all []model.Transaction, now time.Time) (QuestProgress, string) {
	weekStart := startOfWeek(now)
	impulseCount := 0
	transactionCount := 0
	for _, t := range all {
		if t.CreatedAt.Before(weekStart) || t.CreatedAt.After(now) {
			continue
		}
		transactionCount++
		if t.Type != model.Expense || t.BehaviorTag != "impulse" {
			continue
		}
		impulseCount++
	}
	return QuestProgress{Current: float64(impulseCount), Target: 0, Unit: "count", Done: transactionCount > 0 && impulseCount == 0}, "Zero impulse buys this week"
}

func evalLogCount(n int) func([]model.Transaction, time.Time) (QuestProgress, string) {
	return func(all []model.Transaction, now time.Time) (QuestProgress, string) {
		weekStart := startOfWeek(now)
		count := 0
		for _, t := range all {
			if t.CreatedAt.Before(weekStart) || t.CreatedAt.After(now) {
				continue
			}
			count++
		}
		title := fmt.Sprintf("Log %d transactions this week", n)
		return QuestProgress{Current: float64(count), Target: float64(n), Unit: "count", Done: count >= n}, title
	}
}

// calcStreaks scans every distinct day the user logged a transaction and
// returns the current consecutive-day run (ending today or yesterday) and
// the longest run ever recorded.
func calcStreaks(all []model.Transaction, now time.Time) (current int, best int) {
	now = timeutil.InThailand(now)
	days := make(map[string]bool, len(all))
	for _, t := range all {
		days[timeutil.DateKey(t.CreatedAt)] = true
	}

	d := now
	if !days[d.Format("2006-01-02")] {
		d = d.AddDate(0, 0, -1)
	}
	for days[d.Format("2006-01-02")] {
		current++
		d = d.AddDate(0, 0, -1)
	}

	visited := make(map[string]bool, len(days))
	for k := range days {
		if visited[k] {
			continue
		}
		t, err := time.ParseInLocation("2006-01-02", k, timeutil.ThailandLocation())
		if err != nil {
			continue
		}
		for days[t.AddDate(0, 0, -1).Format("2006-01-02")] {
			t = t.AddDate(0, 0, -1)
		}
		runLen := 0
		for cur := t; days[cur.Format("2006-01-02")]; cur = cur.AddDate(0, 0, 1) {
			visited[cur.Format("2006-01-02")] = true
			runLen++
		}
		if runLen > best {
			best = runLen
		}
	}
	return current, best
}

type levelDefinition struct {
	Level    int
	Name     string
	XP       int
	Icon     string
	Feature  string
	Cosmetic string
}

var levelLadder = []levelDefinition{
	{Level: 1, Name: "Newcomer", XP: 0, Icon: "🌱", Feature: "Auto-categorized transactions", Cosmetic: "Clay theme"},
	{Level: 2, Name: "Budgeter", XP: 150, Icon: "📊", Feature: "Monthly target & safe-to-spend", Cosmetic: "Budgeter badge"},
	{Level: 3, Name: "Planner", XP: 300, Icon: "🧭", Feature: "Spending DNA + 6-month history", Cosmetic: "Forest theme"},
	{Level: 4, Name: "Saver", XP: 450, Icon: "⭐", Feature: "2 occasion wallets + weekly quests", Cosmetic: "Gold \"Saver\" title"},
	{Level: 5, Name: "Strategist", XP: 600, Icon: "♟️", Feature: "AI what-if simulations + 12-mo history", Cosmetic: "Midnight theme"},
	{Level: 6, Name: "Sage", XP: 900, Icon: "🦉", Feature: "Yearly forecast + partner cashback", Cosmetic: "Animated Sage crest"},
}

func levelForXP(xp int) (level int, name string, xpIntoLevel int, xpForNext int, nextName string) {
	current := levelLadder[0]
	next := levelLadder[len(levelLadder)-1]
	for i, candidate := range levelLadder {
		if xp < candidate.XP {
			break
		}
		current = candidate
		if i+1 < len(levelLadder) {
			next = levelLadder[i+1]
		} else {
			next = candidate
		}
	}
	level = current.Level
	name = current.Name
	nextName = next.Name
	xpIntoLevel = xp - current.XP
	if next.Level == current.Level {
		xpForNext = 0
	} else {
		xpForNext = next.XP - current.XP
	}
	return level, name, xpIntoLevel, xpForNext, nextName
}

// ErrQuestsInProgress is returned by RerollQuests when the user's current
// batch still has an incomplete quest.
var ErrQuestsInProgress = errors.New("complete your current quests before requesting a new set")
var ErrNoActiveQuestPresets = errors.New("no active quest presets are available for this period")
var ErrInvalidQuestPreset = errors.New("invalid quest preset")
var ErrInvalidQuestPeriod = errors.New("quest period must be daily or weekly")
var ErrQuestSetAlreadyGenerated = errors.New("quest set already generated for this period")
var ErrQuestRerollUnavailable = errors.New("quest reroll is unavailable; only one weekly set is allowed per week")

type QuestView struct {
	Key         string     `json:"key"`
	Period      string     `json:"period"`
	Title       string     `json:"title"`
	Logo        string     `json:"logo"`
	Difficulty  string     `json:"difficulty"`
	Accent      string     `json:"accent"`
	XP          int        `json:"xp"`
	Current     float64    `json:"current"`
	Target      float64    `json:"target"`
	Unit        string     `json:"unit"`
	Done        bool       `json:"done"`
	AssignedAt  time.Time  `json:"assigned_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
	CompletedAt *time.Time `json:"completed_at"`
	XPAwarded   bool       `json:"xp_awarded"`
}

type GameProfileView struct {
	Level          int    `json:"level"`
	LevelName      string `json:"level_name"`
	NextLevelName  string `json:"next_level_name"`
	TotalXP        int    `json:"total_xp"`
	XPIntoLevel    int    `json:"xp_into_level"`
	XPForNextLevel int    `json:"xp_for_next_level"`
}

type StreakView struct {
	CurrentDays int `json:"current_days"`
	BestDays    int `json:"best_days"`
}

type QuestStats struct {
	CurrentBatchTotal       int `json:"current_batch_total"`
	CurrentBatchCompleted   int `json:"current_batch_completed"`
	CurrentBatchRemaining   int `json:"current_batch_remaining"`
	CurrentBatchXPAvailable int `json:"current_batch_xp_available"`
	CurrentBatchXPEarned    int `json:"current_batch_xp_earned"`
	DailyTotal              int `json:"daily_total"`
	DailyCompleted          int `json:"daily_completed"`
	WeeklyTotal             int `json:"weekly_total"`
	WeeklyCompleted         int `json:"weekly_completed"`
	TotalXP                 int `json:"total_xp"`
}

type LevelRewardView struct {
	Level      int    `json:"level"`
	Name       string `json:"name"`
	XPRequired int    `json:"xp_required"`
	Icon       string `json:"icon"`
	Feature    string `json:"feature"`
	Cosmetic   string `json:"cosmetic"`
	State      string `json:"state"`
	Unlocked   bool   `json:"unlocked"`
}

type QuestBoard struct {
	Quests            []QuestView       `json:"quests"`
	DailyQuests       []QuestView       `json:"daily_quests"`
	WeeklyQuests      []QuestView       `json:"weekly_quests"`
	Profile           GameProfileView   `json:"profile"`
	Streak            StreakView        `json:"streak"`
	Stats             QuestStats        `json:"stats"`
	Levels            []LevelRewardView `json:"levels"`
	Rewards           []LevelRewardView `json:"rewards"`
	CanReroll         bool              `json:"can_reroll"`
	CanGenerateDaily  bool              `json:"can_generate_daily"`
	CanGenerateWeekly bool              `json:"can_generate_weekly"`
}

type QuestService interface {
	GetQuests(userID string) (*QuestBoard, error)
	EvaluateActiveQuests(userID string) error
	GenerateQuests(userID string, period model.QuestPeriod) (*QuestBoard, error)
	RerollQuests(userID string) (*QuestBoard, error)
	ListQuestPresets(includeInactive bool) ([]model.QuestPreset, error)
	CreateQuestPreset(preset *model.QuestPreset) (*model.QuestPreset, error)
	UpdateQuestPreset(id uint, patch QuestPresetPatch) (*model.QuestPreset, error)
	DeleteQuestPreset(id uint) error
}

type questService struct {
	repo   repository.QuestRepository
	txRepo repository.TransactionRepository
	mu     sync.Mutex
}

func NewQuestService(repo repository.QuestRepository, txRepo repository.TransactionRepository) QuestService {
	return &questService{repo: repo, txRepo: txRepo}
}

type QuestPresetPatch struct {
	Key         *model.QuestTemplateKey
	Name        *string
	Logo        *string
	Period      *model.QuestPeriod
	Difficulty  *model.QuestDifficulty
	XP          *int
	Accent      *string
	RuleType    *model.QuestRuleType
	Target      *float64
	Unit        *string
	Category    *string
	SubCategory *string
	BehaviorTag *string
	IsActive    *bool
}

func (s *questService) activeTemplatePools() ([]QuestTemplate, []QuestTemplate, map[model.QuestTemplateKey]QuestTemplate, error) {
	if err := s.ensureDefaultQuestPresets(); err != nil {
		return nil, nil, nil, err
	}
	activePresets, err := s.repo.ListQuestPresets(false)
	if err != nil {
		return nil, nil, nil, err
	}
	allPresets, err := s.repo.ListQuestPresets(true)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(allPresets) == 0 {
		allPresets = activePresets
	}

	allTemplates := questTemplatesFromPresets(allPresets)
	allByKey := make(map[model.QuestTemplateKey]QuestTemplate, len(allTemplates))
	for _, tmpl := range allTemplates {
		allByKey[tmpl.Key] = tmpl
	}

	var daily []QuestTemplate
	var weekly []QuestTemplate
	for _, tmpl := range questTemplatesFromPresets(activePresets) {
		if tmpl.Period == model.QuestPeriodDaily {
			daily = append(daily, tmpl)
			continue
		}
		if tmpl.Period == model.QuestPeriodWeekly {
			weekly = append(weekly, tmpl)
		}
	}
	return daily, weekly, allByKey, nil
}

func (s *questService) ensureDefaultQuestPresets() error {
	presets, err := s.repo.ListQuestPresets(true)
	if err != nil {
		return err
	}
	if len(presets) > 0 {
		return nil
	}
	return s.repo.SeedQuestPresets(model.DefaultQuestPresets())
}

func (s *questService) GetQuests(userID string) (*QuestBoard, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.getQuests(userID)
}

func (s *questService) EvaluateActiveQuests(userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.getQuests(userID)
	return err
}

func (s *questService) getQuests(userID string) (*QuestBoard, error) {
	userID = model.UserIDOrDefault(userID)
	now := timeutil.Now()

	transactions, err := s.txRepo.FindAllByUserID(userID)
	if err != nil {
		return nil, err
	}
	profile, err := s.repo.FindGameProfile(userID)
	if err != nil {
		return nil, err
	}

	_, _, templatesByKey, err := s.activeTemplatePools()
	if err != nil {
		return nil, err
	}

	dailyViews, dailyStats, _, dailyXP, err := s.viewsForPeriod(userID, model.QuestPeriodDaily, templatesByKey, transactions, now, profile)
	if err != nil {
		return nil, err
	}
	weeklyViews, weeklyStats, _, weeklyXP, err := s.viewsForPeriod(userID, model.QuestPeriodWeekly, templatesByKey, transactions, now, profile)
	if err != nil {
		return nil, err
	}
	profile.TotalXP += dailyXP + weeklyXP

	if dailyXP+weeklyXP > 0 {
		if err := s.repo.UpsertGameProfile(profile); err != nil {
			return nil, err
		}
	}

	level, levelName, xpInto, xpNext, nextLevelName := levelForXP(profile.TotalXP)
	current, best := calcStreaks(transactions, now)
	stats := mergeQuestStats(dailyStats, weeklyStats, profile.TotalXP)
	dailyGenerated, err := s.hasGeneratedForCurrentWindow(userID, model.QuestPeriodDaily, now)
	if err != nil {
		return nil, err
	}
	weeklyGenerated, err := s.hasGeneratedForCurrentWindow(userID, model.QuestPeriodWeekly, now)
	if err != nil {
		return nil, err
	}

	return &QuestBoard{
		Quests:            weeklyViews,
		DailyQuests:       dailyViews,
		WeeklyQuests:      weeklyViews,
		Profile:           GameProfileView{Level: level, LevelName: levelName, NextLevelName: nextLevelName, TotalXP: profile.TotalXP, XPIntoLevel: xpInto, XPForNextLevel: xpNext},
		Streak:            StreakView{CurrentDays: current, BestDays: best},
		Stats:             stats,
		Levels:            levelRewardsForXP(profile.TotalXP),
		Rewards:           levelRewardsForXP(profile.TotalXP),
		CanReroll:         false,
		CanGenerateDaily:  !dailyGenerated,
		CanGenerateWeekly: !weeklyGenerated,
	}, nil
}

func (s *questService) viewsForPeriod(
	userID string,
	period model.QuestPeriod,
	templatesByKey map[model.QuestTemplateKey]QuestTemplate,
	transactions []model.Transaction,
	now time.Time,
	profile *model.UserGameProfile,
) ([]QuestView, QuestStats, bool, int, error) {
	rows, err := s.repo.FindLatestBatchByPeriod(userID, period, now)
	if err != nil {
		return nil, QuestStats{}, false, 0, err
	}
	if len(rows) == 0 {
		return []QuestView{}, QuestStats{}, false, 0, nil
	}

	views := make([]QuestView, 0, len(rows))
	allDone := true
	xpEarned := 0
	stats := QuestStats{}

	for i := range rows {
		row := &rows[i]
		normalizedAssignment := false
		if row.Period == "" {
			row.Period = period
			normalizedAssignment = true
		}
		if row.ExpiresAt.IsZero() {
			row.ExpiresAt = questExpiry(period, row.AssignedAt)
			normalizedAssignment = true
		}
		tmpl, ok := templateByKey(templatesByKey, row.TemplateKey)
		if !ok {
			continue
		}

		progress, title := tmpl.Evaluate(transactionsSince(transactions, row.AssignedAt), now)

		if progress.Done && row.CompletedAt == nil {
			completedAt := now
			row.CompletedAt = &completedAt
		}
		if progress.Done && !row.XPAwarded {
			row.XPAwarded = true
			xpEarned += tmpl.XP
		}
		if row.CompletedAt != nil || row.XPAwarded || normalizedAssignment {
			if err := s.repo.UpdateAssignment(row); err != nil {
				return nil, QuestStats{}, false, 0, err
			}
		}
		if row.CompletedAt == nil {
			allDone = false
		}
		stats.CurrentBatchTotal++
		stats.CurrentBatchXPAvailable += tmpl.XP
		if row.CompletedAt != nil {
			stats.CurrentBatchCompleted++
			stats.CurrentBatchXPEarned += tmpl.XP
		}

		views = append(views, QuestView{
			Key: string(tmpl.Key), Period: string(period), Title: title, Logo: tmpl.Logo, Difficulty: string(tmpl.Difficulty), Accent: tmpl.Accent, XP: tmpl.XP,
			Current: progress.Current, Target: progress.Target, Unit: progress.Unit, Done: row.CompletedAt != nil,
			AssignedAt: row.AssignedAt, ExpiresAt: row.ExpiresAt, CompletedAt: row.CompletedAt, XPAwarded: row.XPAwarded,
		})
	}
	stats.CurrentBatchRemaining = stats.CurrentBatchTotal - stats.CurrentBatchCompleted
	stats.TotalXP = profile.TotalXP + xpEarned

	return views, stats, allDone, xpEarned, nil
}

func transactionsSince(transactions []model.Transaction, since time.Time) []model.Transaction {
	if since.IsZero() {
		return transactions
	}
	filtered := make([]model.Transaction, 0, len(transactions))
	for _, tx := range transactions {
		if tx.CreatedAt.Before(since) {
			continue
		}
		filtered = append(filtered, tx)
	}
	return filtered
}

func mergeQuestStats(daily, weekly QuestStats, totalXP int) QuestStats {
	return QuestStats{
		CurrentBatchTotal:       weekly.CurrentBatchTotal,
		CurrentBatchCompleted:   weekly.CurrentBatchCompleted,
		CurrentBatchRemaining:   weekly.CurrentBatchRemaining,
		CurrentBatchXPAvailable: weekly.CurrentBatchXPAvailable,
		CurrentBatchXPEarned:    weekly.CurrentBatchXPEarned,
		DailyTotal:              daily.CurrentBatchTotal,
		DailyCompleted:          daily.CurrentBatchCompleted,
		WeeklyTotal:             weekly.CurrentBatchTotal,
		WeeklyCompleted:         weekly.CurrentBatchCompleted,
		TotalXP:                 totalXP,
	}
}

func levelRewardsForXP(totalXP int) []LevelRewardView {
	level, _, _, _, _ := levelForXP(totalXP)
	rewards := make([]LevelRewardView, 0, len(levelLadder))
	for _, item := range levelLadder {
		state := "locked"
		if item.Level < level {
			state = "unlocked"
		}
		if item.Level == level {
			state = "current"
		}
		rewards = append(rewards, LevelRewardView{
			Level:      item.Level,
			Name:       item.Name,
			XPRequired: item.XP,
			Icon:       item.Icon,
			Feature:    item.Feature,
			Cosmetic:   item.Cosmetic,
			State:      state,
			Unlocked:   totalXP >= item.XP,
		})
	}
	return rewards
}

func (s *questService) RerollQuests(userID string) (*QuestBoard, error) {
	return nil, ErrQuestRerollUnavailable
}

func (s *questService) GenerateQuests(userID string, period model.QuestPeriod) (*QuestBoard, error) {
	userID = model.UserIDOrDefault(userID)
	now := timeutil.Now()
	period = normalizeQuestPeriod(period)
	if period == "" {
		return nil, ErrInvalidQuestPeriod
	}

	generated, err := s.hasGeneratedForCurrentWindow(userID, period, now)
	if err != nil {
		return nil, err
	}
	if generated {
		return s.GetQuests(userID)
	}

	existing, err := s.repo.FindLatestBatchByPeriod(userID, period, now)
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 {
		return s.GetQuests(userID)
	}

	dailyPool, weeklyPool, _, err := s.activeTemplatePools()
	if err != nil {
		return nil, err
	}
	pool := weeklyPool
	if period == model.QuestPeriodDaily {
		pool = dailyPool
	}
	if err := s.createRandomBatchForPeriod(userID, period, pool, 3, now); err != nil {
		return nil, err
	}

	return s.GetQuests(userID)
}

func (s *questService) hasGeneratedForCurrentWindow(userID string, period model.QuestPeriod, now time.Time) (bool, error) {
	start, end := questPeriodWindow(period, now)
	return s.repo.HasBatchForPeriodWindow(userID, period, start, end)
}

func questPeriodWindow(period model.QuestPeriod, now time.Time) (time.Time, time.Time) {
	if period == model.QuestPeriodDaily {
		return timeutil.StartOfDay(now), timeutil.StartOfNextDay(now)
	}
	return timeutil.StartOfWeek(now), timeutil.StartOfNextWeek(now)
}

func normalizeQuestPeriod(period model.QuestPeriod) model.QuestPeriod {
	switch period {
	case model.QuestPeriodDaily, model.QuestPeriodWeekly:
		return period
	default:
		return ""
	}
}

func (s *questService) ListQuestPresets(includeInactive bool) ([]model.QuestPreset, error) {
	if err := s.ensureDefaultQuestPresets(); err != nil {
		return nil, err
	}
	return s.repo.ListQuestPresets(includeInactive)
}

func (s *questService) CreateQuestPreset(preset *model.QuestPreset) (*model.QuestPreset, error) {
	if err := validateQuestPreset(preset); err != nil {
		return nil, err
	}
	if err := s.repo.CreateQuestPreset(preset); err != nil {
		return nil, err
	}
	return preset, nil
}

func (s *questService) UpdateQuestPreset(id uint, patch QuestPresetPatch) (*model.QuestPreset, error) {
	preset, err := s.repo.GetQuestPreset(id)
	if err != nil {
		return nil, err
	}
	if patch.Key != nil {
		preset.Key = *patch.Key
	}
	if patch.Name != nil {
		preset.Name = *patch.Name
	}
	if patch.Logo != nil {
		preset.Logo = *patch.Logo
	}
	if patch.Period != nil {
		preset.Period = *patch.Period
	}
	if patch.Difficulty != nil {
		preset.Difficulty = *patch.Difficulty
	}
	if patch.XP != nil {
		preset.XP = *patch.XP
	}
	if patch.Accent != nil {
		preset.Accent = *patch.Accent
	}
	if patch.RuleType != nil {
		preset.RuleType = *patch.RuleType
	}
	if patch.Target != nil {
		preset.Target = *patch.Target
	}
	if patch.Unit != nil {
		preset.Unit = *patch.Unit
	}
	if patch.Category != nil {
		preset.Category = *patch.Category
	}
	if patch.SubCategory != nil {
		preset.SubCategory = *patch.SubCategory
	}
	if patch.BehaviorTag != nil {
		preset.BehaviorTag = *patch.BehaviorTag
	}
	if patch.IsActive != nil {
		preset.IsActive = *patch.IsActive
	}
	if err := validateQuestPreset(preset); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateQuestPreset(preset); err != nil {
		return nil, err
	}
	return preset, nil
}

func (s *questService) DeleteQuestPreset(id uint) error {
	return s.repo.DeleteQuestPreset(id)
}

func (s *questService) createRandomBatch(userID string) error {
	_, weeklyPool, _, err := s.activeTemplatePools()
	if err != nil {
		return err
	}
	return s.createRandomBatchForPeriod(userID, model.QuestPeriodWeekly, weeklyPool, 3, timeutil.Now())
}

func (s *questService) createRandomBatchForPeriod(userID string, period model.QuestPeriod, source []QuestTemplate, pickCount int, now time.Time) error {
	now = timeutil.InThailand(now)
	pool := append([]QuestTemplate{}, source...)
	if len(pool) == 0 {
		return ErrNoActiveQuestPresets
	}
	rand.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	if pickCount > len(pool) {
		pickCount = len(pool)
	}

	batchID := fmt.Sprintf("%d-%d", now.UnixNano(), rand.Int63())
	expiresAt := questExpiry(period, now)
	rows := make([]model.UserQuestAssignment, 0, pickCount)
	for _, tmpl := range pool[:pickCount] {
		rows = append(rows, model.UserQuestAssignment{
			UserID: userID, BatchID: batchID, Period: period, TemplateKey: tmpl.Key, AssignedAt: now, ExpiresAt: expiresAt,
		})
	}
	return s.repo.CreateBatch(rows)
}

func validateQuestPreset(preset *model.QuestPreset) error {
	preset.Key = model.QuestTemplateKey(strings.TrimSpace(string(preset.Key)))
	preset.Name = strings.TrimSpace(preset.Name)
	preset.Logo = strings.TrimSpace(preset.Logo)
	preset.Accent = strings.TrimSpace(preset.Accent)
	preset.Unit = strings.TrimSpace(preset.Unit)
	preset.Category = strings.TrimSpace(preset.Category)
	preset.SubCategory = strings.TrimSpace(preset.SubCategory)
	preset.BehaviorTag = strings.TrimSpace(preset.BehaviorTag)

	if preset.Key == "" || preset.Name == "" || preset.XP <= 0 {
		return ErrInvalidQuestPreset
	}
	if preset.Period != model.QuestPeriodDaily && preset.Period != model.QuestPeriodWeekly {
		return ErrInvalidQuestPreset
	}
	switch preset.Difficulty {
	case model.QuestBasic, model.QuestAdvanced, model.QuestExpert, model.QuestMaster, model.QuestGrandMaster, model.QuestEasy, model.QuestMedium, model.QuestHard:
	default:
		return ErrInvalidQuestPreset
	}
	if _, ok := evaluatorForPreset(*preset); !ok {
		return ErrInvalidQuestPreset
	}
	if preset.Unit == "" {
		preset.Unit = "count"
	}
	return nil
}
