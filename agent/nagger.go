package agent

import (
	"fmt"
	"log"

	"finance-chat/model"
)

// naggerAgent generates behavioral alerts based on transaction data, behavior DNA,
// and the surprise score computed by the Profiler agent.
type naggerAgent struct {
	deps AgentDeps
}

// NewNaggerAgent returns a fully-implemented Nagger agent.
func NewNaggerAgent(deps AgentDeps) Agent {
	return &naggerAgent{deps: deps}
}

// Name returns the agent's identifier.
func (n *naggerAgent) Name() string { return "nagger" }

// Run evaluates the three alert conditions and appends any triggered alerts to
// ctx.Alerts. For each alert it also attempts a LINE push notification (non-blocking).
func (n *naggerAgent) Run(ctx *AgentContext) (*AgentResult, error) {
	// Guard: both SavedTx and BehaviorDNA must be present.
	if ctx.SavedTx == nil || ctx.BehaviorDNA == nil {
		return &AgentResult{AgentName: "nagger", Data: ctx.Alerts}, nil
	}

	tx := ctx.SavedTx
	dna := ctx.BehaviorDNA
	var alerts []model.Alert

	// ── Condition 1: Warning — SurpriseScore ≥ 70 ────────────────────────────
	if ctx.SurpriseScore >= 70 {
		// We cannot reliably reverse-engineer the category average from the score
		// alone, so CategoryAvg is left as 0. The score itself was computed by the
		// Profiler and the message conveys the multiplier directly.
		avg := 0.0
		multiplier := tx.Amount / max(avg, 1)
		alerts = append(alerts, model.Alert{
			Type:        "warning",
			Message:     fmt.Sprintf("You just spent %.0f on %s — that's %.0fx your usual. Intentional?", tx.Amount, tx.Category, multiplier),
			Category:    tx.Category,
			Amount:      tx.Amount,
			CategoryAvg: avg,
		})
	}

	// ── Condition 2: Impulse flag — behavior_tag == "impulse" AND amount > 500 ─
	if tx.BehaviorTag == "impulse" && tx.Amount > 500 {
		alerts = append(alerts, model.Alert{
			Type:     "impulse_flag",
			Message:  fmt.Sprintf("Impulse buy alert: %.0f on %s. Was this planned?", tx.Amount, tx.Brand),
			Category: tx.Category,
			Amount:   tx.Amount,
		})
	}

	// ── Condition 3: Luxury drift — BehaviorDNA.LuxuryDriftDetected == true ───
	if dna.LuxuryDriftDetected {
		alerts = append(alerts, model.Alert{
			Type:    "luxury_drift",
			Message: fmt.Sprintf("Luxury drift detected: your discretionary spending is up %.1f%% this month. Your wallet is drifting.", dna.LuxuryDriftIndex),
		})
	}

	// Append all generated alerts to the shared context.
	ctx.Alerts = append(ctx.Alerts, alerts...)

	// ── LINE push notifications (non-blocking) ────────────────────────────────
	if ctx.LineUserID != "" && n.deps.LineService != nil {
		for _, alert := range alerts {
			if err := n.deps.LineService.ReplyMessage(ctx.LineUserID, alert.Message); err != nil {
				log.Printf("[nagger] LINE push failed for alert %s on tx %d: %v", alert.Type, tx.ID, err)
				// continue — LINE failures must not block the pipeline
			}
		}
	}

	return &AgentResult{AgentName: "nagger", Data: ctx.Alerts}, nil
}

// max returns the larger of a and b (float64 helper).
func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
