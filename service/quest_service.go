package service

import (
	"errors"
	"finance-chat/model"
	"finance-chat/repository"
	"fmt"
	"math/rand"
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
	Difficulty model.QuestDifficulty
	XP         int
	Accent     string
	Evaluate   func(all []model.Transaction, now time.Time) (QuestProgress, string)
}

var questTemplatePool = []QuestTemplate{
	{Key: "no_spend_2", Difficulty: model.QuestEasy, XP: 60, Accent: "#3D9B6E", Evaluate: evalNoSpendDays(2)},
	{Key: "food_cap_400", Difficulty: model.QuestMedium, XP: 40, Accent: "#D9463B", Evaluate: evalCategoryCap("Food & Beverage", "coffee", 400)},
	{Key: "beat_last_month_savings", Difficulty: model.QuestHard, XP: 100, Accent: "#E6A23C", Evaluate: evalBeatLastMonthSavingsRate},
	{Key: "no_impulse_week", Difficulty: model.QuestMedium, XP: 50, Accent: "#B5829B", Evaluate: evalNoImpulseThisWeek},
	{Key: "log_5", Difficulty: model.QuestEasy, XP: 30, Accent: "#8C9CB0", Evaluate: evalLogCount(5)},
}

func templateByKey(key model.QuestTemplateKey) (QuestTemplate, bool) {
	for _, t := range questTemplatePool {
		if t.Key == key {
			return t, true
		}
	}
	return QuestTemplate{}, false
}

// startOfWeek returns midnight on the Monday of now's week.
func startOfWeek(now time.Time) time.Time {
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday -> 7, so Monday is always day 1
	}
	d := now.AddDate(0, 0, -(weekday - 1))
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, d.Location())
}

func evalNoSpendDays(target int) func([]model.Transaction, time.Time) (QuestProgress, string) {
	return func(all []model.Transaction, now time.Time) (QuestProgress, string) {
		weekStart := startOfWeek(now)
		spendByDay := map[string]float64{}
		for _, t := range all {
			if t.Type != model.Expense || t.CreatedAt.Before(weekStart) || t.CreatedAt.After(now) {
				continue
			}
			spendByDay[t.CreatedAt.Format("2006-01-02")] += t.Amount
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
		for _, t := range all {
			if t.Type != model.Expense || t.Category != category {
				continue
			}
			if t.CreatedAt.Before(weekStart) || t.CreatedAt.After(now) {
				continue
			}
			sum += t.Amount
		}
		weekEnd := weekStart.AddDate(0, 0, 7)
		done := !now.Before(weekEnd) && sum <= cap
		title := fmt.Sprintf("Keep %s under ฿%s", label, formatThb(cap))
		return QuestProgress{Current: sum, Target: cap, Unit: "thb", Done: done}, title
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
	var income, expense float64
	for _, t := range all {
		if t.CreatedAt.Year() != ref.Year() || t.CreatedAt.Month() != ref.Month() {
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
	count := 0
	for _, t := range all {
		if t.Type != model.Expense || t.BehaviorTag != "impulse" {
			continue
		}
		if t.CreatedAt.Before(weekStart) || t.CreatedAt.After(now) {
			continue
		}
		count++
	}
	return QuestProgress{Current: float64(count), Target: 0, Unit: "count", Done: count == 0}, "Zero impulse buys this week"
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
	days := make(map[string]bool, len(all))
	for _, t := range all {
		days[t.CreatedAt.Format("2006-01-02")] = true
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
		t, err := time.Parse("2006-01-02", k)
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

const xpPerLevel = 150

var levelNames = []string{"Newcomer", "Saver", "Planner", "Strategist", "Sage"}

func levelForXP(xp int) (level int, name string, xpIntoLevel int, xpForNext int, nextName string) {
	level = xp/xpPerLevel + 1
	xpIntoLevel = xp % xpPerLevel
	xpForNext = xpPerLevel
	idx := level - 1
	if idx >= len(levelNames) {
		idx = len(levelNames) - 1
	}
	nextIdx := idx + 1
	if nextIdx >= len(levelNames) {
		nextIdx = len(levelNames) - 1
	}
	return level, levelNames[idx], xpIntoLevel, xpForNext, levelNames[nextIdx]
}

// ErrQuestsInProgress is returned by RerollQuests when the user's current
// batch still has an incomplete quest.
var ErrQuestsInProgress = errors.New("complete your current quests before requesting a new set")

type QuestView struct {
	Key        string  `json:"key"`
	Title      string  `json:"title"`
	Difficulty string  `json:"difficulty"`
	Accent     string  `json:"accent"`
	XP         int     `json:"xp"`
	Current    float64 `json:"current"`
	Target     float64 `json:"target"`
	Unit       string  `json:"unit"`
	Done       bool    `json:"done"`
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

type QuestBoard struct {
	Quests    []QuestView     `json:"quests"`
	Profile   GameProfileView `json:"profile"`
	Streak    StreakView      `json:"streak"`
	CanReroll bool            `json:"can_reroll"`
}

type QuestService interface {
	GetQuests(userID string) (*QuestBoard, error)
	RerollQuests(userID string) (*QuestBoard, error)
}

type questService struct {
	repo   repository.QuestRepository
	txRepo repository.TransactionRepository
}

func NewQuestService(repo repository.QuestRepository, txRepo repository.TransactionRepository) QuestService {
	return &questService{repo: repo, txRepo: txRepo}
}

func (s *questService) GetQuests(userID string) (*QuestBoard, error) {
	userID = model.UserIDOrDefault(userID)
	now := time.Now()

	transactions, err := s.txRepo.FindAllByUserID(userID)
	if err != nil {
		return nil, err
	}
	profile, err := s.repo.FindGameProfile(userID)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.FindLatestBatch(userID)
	if err != nil {
		return nil, err
	}

	views := make([]QuestView, 0, len(rows))
	allDone := true
	profileDirty := false

	for i := range rows {
		row := &rows[i]
		tmpl, ok := templateByKey(row.TemplateKey)
		if !ok {
			continue
		}

		progress, title := tmpl.Evaluate(transactions, now)

		if progress.Done && row.CompletedAt == nil {
			completedAt := now
			row.CompletedAt = &completedAt
		}
		if progress.Done && !row.XPAwarded {
			row.XPAwarded = true
			profile.TotalXP += tmpl.XP
			profileDirty = true
		}
		if row.CompletedAt != nil || row.XPAwarded {
			if err := s.repo.UpdateAssignment(row); err != nil {
				return nil, err
			}
		}
		if row.CompletedAt == nil {
			allDone = false
		}

		views = append(views, QuestView{
			Key: string(tmpl.Key), Title: title, Difficulty: string(tmpl.Difficulty), Accent: tmpl.Accent, XP: tmpl.XP,
			Current: progress.Current, Target: progress.Target, Unit: progress.Unit, Done: row.CompletedAt != nil,
		})
	}

	if profileDirty {
		if err := s.repo.UpsertGameProfile(profile); err != nil {
			return nil, err
		}
	}

	level, levelName, xpInto, xpNext, nextLevelName := levelForXP(profile.TotalXP)
	current, best := calcStreaks(transactions, now)

	return &QuestBoard{
		Quests:    views,
		Profile:   GameProfileView{Level: level, LevelName: levelName, NextLevelName: nextLevelName, TotalXP: profile.TotalXP, XPIntoLevel: xpInto, XPForNextLevel: xpNext},
		Streak:    StreakView{CurrentDays: current, BestDays: best},
		CanReroll: allDone,
	}, nil
}

func (s *questService) RerollQuests(userID string) (*QuestBoard, error) {
	userID = model.UserIDOrDefault(userID)

	existing, err := s.repo.FindLatestBatch(userID)
	if err != nil {
		return nil, err
	}
	for _, row := range existing {
		if row.CompletedAt == nil {
			return nil, ErrQuestsInProgress
		}
	}

	pool := append([]QuestTemplate{}, questTemplatePool...)
	rand.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	pickCount := 3
	if pickCount > len(pool) {
		pickCount = len(pool)
	}

	now := time.Now()
	batchID := fmt.Sprintf("%d-%d", now.UnixNano(), rand.Int63())
	rows := make([]model.UserQuestAssignment, 0, pickCount)
	for _, tmpl := range pool[:pickCount] {
		rows = append(rows, model.UserQuestAssignment{
			UserID: userID, BatchID: batchID, TemplateKey: tmpl.Key, AssignedAt: now,
		})
	}
	if err := s.repo.CreateBatch(rows); err != nil {
		return nil, err
	}

	return s.GetQuests(userID)
}
