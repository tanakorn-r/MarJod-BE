package service

import (
	"finance-chat/model"
	"finance-chat/timeutil"
	"math"
	"sort"
	"strings"
	"time"
)

var dnaDiscretionaryCategories = map[string]bool{
	"Shopping":        true,
	"Entertainment":   true,
	"Food & Beverage": true,
}

// BrandSpend is one brand's real spend total over the trailing 30 days.
type BrandSpend struct {
	Brand  string  `json:"brand"`
	Visits int     `json:"visits"`
	Amount float64 `json:"amount"`
}

// SpendingDNA is a behavioral snapshot computed entirely from real
// transactions — every field is derived, nothing is editorialized beyond
// rule-based labels (High/Medium/Low, archetype name) applied to real
// percentages and ratios.
type SpendingDNA struct {
	Archetype            string       `json:"archetype"`
	ArchetypeDescription string       `json:"archetype_description"`
	Consistency          string       `json:"consistency"`
	ImpulseControl       string       `json:"impulse_control"`
	SaveRate             float64      `json:"save_rate"`
	Volatility           string       `json:"volatility"`
	DominantCategory     string       `json:"dominant_category"`
	DominantCategoryPct  float64      `json:"dominant_category_pct"`
	ImpulseFrequency30d  int          `json:"impulse_frequency_30d"`
	LuxuryDriftIndex     float64      `json:"luxury_drift_index"`
	LuxuryDriftDetected  bool         `json:"luxury_drift_detected"`
	TopBrands            []BrandSpend `json:"top_brands"`
	InsufficientData     bool         `json:"insufficient_data"`
}

func (s *transactionService) GetSpendingDNA(userID string, walletID ...uint) (*SpendingDNA, error) {
	userID = model.UserIDOrDefault(userID)
	list, err := s.repo.FindAllByUserID(userID)
	if err != nil {
		return nil, err
	}
	if len(walletID) > 0 {
		effectiveWalletID := walletID[0]
		if walletID[0] == model.GeneralWalletID {
			wallet, err := s.walletRepo.GetWalletByID(userID, walletID[0])
			if err != nil {
				return nil, err
			}
			effectiveWalletID = wallet.ID
		}
		list = filterByWallet(list, effectiveWalletID)
	}
	if len(list) < 5 {
		return &SpendingDNA{InsufficientData: true}, nil
	}

	now := timeutil.Now()
	thirtyAgo := now.AddDate(0, 0, -30)
	sixtyAgo := now.AddDate(0, 0, -60)

	var last30 []model.Transaction
	for _, t := range list {
		if t.CreatedAt.After(thirtyAgo) && !t.CreatedAt.After(now) {
			last30 = append(last30, t)
		}
	}

	tagPct := behaviorTagPercentages(last30)
	consistency := consistencyLabel(last30, thirtyAgo, now)
	impulseControl := impulseControlLabel(tagPct["impulse"])
	volatility := volatilityLabel(dailyExpenseSeries(last30, thirtyAgo, now))
	saveRate := savingsRateForMonth(list, now)
	dominantCategory, dominantPct := dominantCategoryFor(last30)
	impulseFreq := countImpulse30d(list, thirtyAgo)
	driftIndex, driftDetected := luxuryDrift(list, now, thirtyAgo, sixtyAgo)
	topBrands := topBrandsLast30Days(last30)
	archetype, archetypeDesc := deriveArchetype(tagPct)

	return &SpendingDNA{
		Archetype:            archetype,
		ArchetypeDescription: archetypeDesc,
		Consistency:          consistency,
		ImpulseControl:       impulseControl,
		SaveRate:             saveRate,
		Volatility:           volatility,
		DominantCategory:     dominantCategory,
		DominantCategoryPct:  dominantPct,
		ImpulseFrequency30d:  impulseFreq,
		LuxuryDriftIndex:     driftIndex,
		LuxuryDriftDetected:  driftDetected,
		TopBrands:            topBrands,
		InsufficientData:     false,
	}, nil
}

// behaviorTagPercentages returns each behavior tag's share of total expense
// within the given transaction slice (0-100).
func behaviorTagPercentages(txs []model.Transaction) map[string]float64 {
	amounts := map[string]float64{}
	var total float64
	for _, t := range txs {
		if t.Type != model.Expense {
			continue
		}
		tag := t.BehaviorTag
		if tag == "" {
			tag = "necessity"
		}
		amounts[tag] += t.Amount
		total += t.Amount
	}
	pct := map[string]float64{}
	if total <= 0 {
		return pct
	}
	for tag, amt := range amounts {
		pct[tag] = amt / total * 100
	}
	return pct
}

// consistencyLabel buckets how regularly the user logs transactions: the
// share of days in the window that have at least one transaction.
func consistencyLabel(txs []model.Transaction, from, to time.Time) string {
	days := map[string]bool{}
	for _, t := range txs {
		days[timeutil.DateKey(t.CreatedAt)] = true
	}
	totalDays := int(math.Round(to.Sub(from).Hours() / 24))
	if totalDays <= 0 {
		totalDays = 1
	}
	pct := float64(len(days)) / float64(totalDays) * 100
	switch {
	case pct >= 70:
		return "High"
	case pct >= 40:
		return "Medium"
	default:
		return "Low"
	}
}

func impulseControlLabel(impulsePct float64) string {
	switch {
	case impulsePct < 8:
		return "High"
	case impulsePct < 20:
		return "Medium"
	default:
		return "Low"
	}
}

// dailyExpenseSeries returns one total per calendar day in [from, to),
// including zero-spend days, for a volatility (variance) calculation.
func dailyExpenseSeries(txs []model.Transaction, from, to time.Time) []float64 {
	byDay := map[string]float64{}
	for _, t := range txs {
		if t.Type != model.Expense {
			continue
		}
		byDay[timeutil.DateKey(t.CreatedAt)] += t.Amount
	}
	var series []float64
	for d := from; d.Before(to); d = d.AddDate(0, 0, 1) {
		series = append(series, byDay[timeutil.DateKey(d)])
	}
	return series
}

func volatilityLabel(series []float64) string {
	if len(series) == 0 {
		return "Low"
	}
	var sum float64
	for _, v := range series {
		sum += v
	}
	mean := sum / float64(len(series))
	if mean <= 0 {
		return "Low"
	}
	var variance float64
	for _, v := range series {
		variance += (v - mean) * (v - mean)
	}
	variance /= float64(len(series))
	cv := math.Sqrt(variance) / mean
	switch {
	case cv < 0.6:
		return "Low"
	case cv < 1.2:
		return "Medium"
	default:
		return "High"
	}
}

func dominantCategoryFor(txs []model.Transaction) (string, float64) {
	amounts := map[string]float64{}
	var total float64
	for _, t := range txs {
		if t.Type != model.Expense {
			continue
		}
		amounts[t.Category] += t.Amount
		total += t.Amount
	}
	if total <= 0 {
		return "", 0
	}
	var top string
	var topAmt float64
	for cat, amt := range amounts {
		if amt > topAmt {
			top, topAmt = cat, amt
		}
	}
	return top, topAmt / total * 100
}

func countImpulse30d(all []model.Transaction, thirtyAgo time.Time) int {
	count := 0
	for _, t := range all {
		if t.BehaviorTag == "impulse" && t.CreatedAt.After(thirtyAgo) {
			count++
		}
	}
	return count
}

// luxuryDrift compares discretionary spend in the trailing 30 days against
// the 30 days before that — mirrors agent/profiler.go's existing formula.
func luxuryDrift(all []model.Transaction, now, thirtyAgo, sixtyAgo time.Time) (float64, bool) {
	var current, prior float64
	for _, t := range all {
		if t.Type != model.Expense || !dnaDiscretionaryCategories[t.Category] {
			continue
		}
		if t.CreatedAt.After(thirtyAgo) && !t.CreatedAt.After(now) {
			current += t.Amount
		} else if t.CreatedAt.After(sixtyAgo) && !t.CreatedAt.After(thirtyAgo) {
			prior += t.Amount
		}
	}
	if prior <= 0 {
		return 0, false
	}
	index := (current - prior) / prior * 100
	return index, index > 20
}

var genericBrandPlaceholders = map[string]bool{
	"general": true, "n/a": true, "na": true, "unknown": true, "other": true,
}

func isRealBrand(brand string) bool {
	return brand != "" && !genericBrandPlaceholders[strings.ToLower(strings.TrimSpace(brand))]
}

func topBrandsLast30Days(txs []model.Transaction) []BrandSpend {
	type agg struct {
		visits int
		amount float64
	}
	byBrand := map[string]agg{}
	for _, t := range txs {
		if t.Type != model.Expense || !isRealBrand(t.Brand) {
			continue
		}
		a := byBrand[t.Brand]
		a.visits++
		a.amount += t.Amount
		byBrand[t.Brand] = a
	}
	result := make([]BrandSpend, 0, len(byBrand))
	for brand, a := range byBrand {
		result = append(result, BrandSpend{Brand: brand, Visits: a.visits, Amount: a.amount})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Amount != result[j].Amount {
			return result[i].Amount > result[j].Amount
		}
		return result[i].Brand < result[j].Brand
	})
	if len(result) > 5 {
		result = result[:5]
	}
	return result
}

// deriveArchetype maps real behavior-tag percentages to a human-readable
// label via fixed, ordered rules — same spirit as the existing
// on-track/watch/over-budget budget theming, just applied to spending mix.
func deriveArchetype(pct map[string]float64) (string, string) {
	switch {
	case pct["impulse"] >= 25:
		return "The Impulse Spender", "Purchases often happen in the moment — small decisions add up fast."
	case pct["necessity"] >= 60:
		return "The Essentials-First Saver", "Most of your spend covers the basics — discretionary spending stays in check."
	case pct["treat"] >= 20 && pct["impulse"] < 15:
		return "The Steady Treater", "You cover essentials reliably, then reward yourself in small, frequent treats."
	case pct["social"] >= 20:
		return "The Social Spender", "A meaningful share of your spend goes toward time with others."
	case pct["recurring"] >= 30:
		return "The Subscriber", "Recurring bills and subscriptions make up a big share of your spend."
	default:
		return "The Balanced Spender", "Your spending is spread fairly evenly across needs, treats, and savings."
	}
}
