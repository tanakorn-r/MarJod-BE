package agent

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"finance-chat/model"
)

// discretionaryCategories defines the categories used for luxury drift calculation.
var discretionaryCategories = map[string]bool{
	"Shopping":        true,
	"Entertainment":   true,
	"Food & Beverage": true,
}

// profilerAgent computes behavioral metrics from the user's transaction history.
type profilerAgent struct {
	deps AgentDeps
}

// NewProfilerAgent returns a fully-wired Profiler agent.
func NewProfilerAgent(deps AgentDeps) Agent {
	return &profilerAgent{deps: deps}
}

// Name satisfies the Agent interface.
func (p *profilerAgent) Name() string { return "profiler" }

// ComputeSurpriseScore computes how surprising a transaction amount is relative
// to the user's 30-day category average.
//
// Formula: min(100, round((amount - avg) / max(avg, 1) * 100))
// Amounts at or below average return 0. Result is always in [0, 100].
func ComputeSurpriseScore(amount, avg float64) int {
	if amount <= avg {
		return 0
	}
	denom := math.Max(avg, 1)
	raw := math.Round((amount - avg) / denom * 100)
	if raw < 0 {
		return 0
	}
	if raw > 100 {
		return 100
	}
	return int(raw)
}

// Run executes the Profiler pipeline step:
//  1. Fetch all transactions.
//  2. If fewer than 5, set InsufficientData and return early.
//  3. Compute BehaviorDNA fields.
//  4. Compute SurpriseScore for ctx.SavedTx.
//  5. Persist BehaviorProfile.
//  6. Set ctx.BehaviorDNA and ctx.SurpriseScore.
func (p *profilerAgent) Run(ctx *AgentContext) (*AgentResult, error) {
	userID := model.UserIDOrDefault(ctx.LineUserID)
	txs, err := p.deps.TxRepo.FindAllByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("agent profiler failed: %w", err)
	}

	// Insufficient data check
	if len(txs) < 5 {
		ctx.BehaviorDNA = &model.BehaviorDNA{InsufficientData: true}
		return &AgentResult{AgentName: "profiler", Data: ctx.BehaviorDNA}, nil
	}

	now := time.Now()
	thirtyDaysAgo := now.AddDate(0, 0, -30)
	sixtyDaysAgo := now.AddDate(0, 0, -60)

	// ── Dominant Category ────────────────────────────────────────────────────
	// Category with highest total spend (sum of amounts for expenses)
	categorySpend := make(map[string]float64)
	for _, tx := range txs {
		if tx.Type == model.Expense {
			categorySpend[tx.Category] += tx.Amount
		}
	}
	dominantCategory := ""
	var maxSpend float64
	for cat, spend := range categorySpend {
		if spend > maxSpend {
			maxSpend = spend
			dominantCategory = cat
		}
	}

	// ── Impulse Frequency ────────────────────────────────────────────────────
	// Count of transactions with BehaviorTag == "impulse" in last 30 days
	impulseFrequency := 0
	for _, tx := range txs {
		if tx.BehaviorTag == "impulse" && tx.CreatedAt.After(thirtyDaysAgo) {
			impulseFrequency++
		}
	}

	// ── Luxury Drift Index ───────────────────────────────────────────────────
	// Percentage change in discretionary spend vs prior 30-day period
	var currentDiscretionary, priorDiscretionary float64
	for _, tx := range txs {
		if !discretionaryCategories[tx.Category] {
			continue
		}
		if tx.Type != model.Expense {
			continue
		}
		if tx.CreatedAt.After(thirtyDaysAgo) {
			currentDiscretionary += tx.Amount
		} else if tx.CreatedAt.After(sixtyDaysAgo) {
			priorDiscretionary += tx.Amount
		}
	}
	luxuryDriftIndex := 0.0
	if priorDiscretionary > 0 {
		luxuryDriftIndex = ((currentDiscretionary - priorDiscretionary) / math.Max(priorDiscretionary, 1)) * 100
	}
	luxuryDriftDetected := luxuryDriftIndex > 20.0

	// ── Top Brands ───────────────────────────────────────────────────────────
	// Top 3 brand names by transaction frequency
	brandCount := make(map[string]int)
	for _, tx := range txs {
		if tx.Brand != "" {
			brandCount[tx.Brand]++
		}
	}
	type brandFreq struct {
		name  string
		count int
	}
	brands := make([]brandFreq, 0, len(brandCount))
	for name, count := range brandCount {
		brands = append(brands, brandFreq{name, count})
	}
	sort.Slice(brands, func(i, j int) bool {
		if brands[i].count != brands[j].count {
			return brands[i].count > brands[j].count
		}
		return brands[i].name < brands[j].name
	})
	topBrands := make([]string, 0, 3)
	for i := 0; i < len(brands) && i < 3; i++ {
		topBrands = append(topBrands, brands[i].name)
	}

	// ── Assemble BehaviorDNA ─────────────────────────────────────────────────
	dna := &model.BehaviorDNA{
		DominantCategory:    dominantCategory,
		ImpulseFrequency:    impulseFrequency,
		LuxuryDriftIndex:    luxuryDriftIndex,
		LuxuryDriftDetected: luxuryDriftDetected,
		TopBrands:           topBrands,
		InsufficientData:    false,
	}

	// ── Surprise Score for ctx.SavedTx ───────────────────────────────────────
	if ctx.SavedTx != nil {
		var categoryTotal float64
		var categoryCount int
		for _, tx := range txs {
			if tx.Category == ctx.SavedTx.Category &&
				tx.Type == model.Expense &&
				tx.CreatedAt.After(thirtyDaysAgo) &&
				tx.ID != ctx.SavedTx.ID {
				categoryTotal += tx.Amount
				categoryCount++
			}
		}
		avg := 0.0
		if categoryCount > 0 {
			avg = categoryTotal / float64(categoryCount)
		}
		ctx.SurpriseScore = ComputeSurpriseScore(ctx.SavedTx.Amount, avg)
	}

	ctx.BehaviorDNA = dna

	// ── Persist BehaviorProfile ───────────────────────────────────────────────
	topBrandsJSON, err := json.Marshal(topBrands)
	if err != nil {
		return nil, fmt.Errorf("agent profiler failed: %w", err)
	}

	profile := &model.BehaviorProfile{
		UserID:              userID,
		ComputedDate:        time.Now(),
		DominantCategory:    dna.DominantCategory,
		ImpulseFrequency:    dna.ImpulseFrequency,
		LuxuryDriftIndex:    dna.LuxuryDriftIndex,
		LuxuryDriftDetected: dna.LuxuryDriftDetected,
		TopBrands:           string(topBrandsJSON),
		InsufficientData:    dna.InsufficientData,
	}

	if err := p.deps.ProfileRepo.Save(profile); err != nil {
		return nil, fmt.Errorf("agent profiler failed: %w", err)
	}

	return &AgentResult{AgentName: "profiler", Data: ctx.BehaviorDNA}, nil
}
