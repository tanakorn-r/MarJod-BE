package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"finance-chat/model"
)

// strategistAgent generates personalized financial recommendations based on the
// user's BehaviorDNA profile using LLM-powered analysis.
type strategistAgent struct {
	deps AgentDeps
}

// NewStrategistAgent returns a fully-implemented Strategist agent.
func NewStrategistAgent(deps AgentDeps) Agent {
	return &strategistAgent{deps: deps}
}

// Name returns the agent's identifier.
func (s *strategistAgent) Name() string { return "strategist" }

// Run generates financial recommendations. If insufficient data exists, it returns
// a generic recommendation. Otherwise, it uses the LLM to generate personalized
// recommendations based on the BehaviorDNA profile.
func (s *strategistAgent) Run(ctx *AgentContext) (*AgentResult, error) {
	// Case A: Insufficient data — return generic recommendation without calling LLM
	if ctx.BehaviorDNA == nil || ctx.BehaviorDNA.InsufficientData {
		ctx.Recommendations = append(ctx.Recommendations, model.Recommendation{
			Title:                "Build Your Financial Picture",
			Description:          "Log at least 5 transactions to unlock personalized wealth-building strategies. Every baht tracked is a step toward financial clarity.",
			EstimatedMonthlySave: 0,
			Priority:             "low",
		})
		return &AgentResult{AgentName: "strategist", Data: ctx.Recommendations}, nil
	}

	// Case B: Full analysis — build prompt and call LLM
	dna := ctx.BehaviorDNA
	topBrandsStr := strings.Join(dna.TopBrands, ", ")
	if topBrandsStr == "" {
		topBrandsStr = "none"
	}

	prompt := fmt.Sprintf(`You are The Strategist. Growth-oriented. Motivational. Focused on net worth.

Based on this user's BehaviorDNA:
- Dominant Category: %s
- Impulse Frequency: %d times in last 30 days
- Luxury Drift Index: %.1f%%
- Luxury Drift Detected: %t
- Top Brands: %s

Generate 2-3 actionable financial recommendations. Return ONLY a JSON array:
[
  {
    "title": "string",
    "description": "string",
    "estimated_monthly_save": number,
    "priority": "high"|"medium"|"low"
  }
]

Focus on compounding gains and long-term wealth. Be specific with numbers.`,
		dna.DominantCategory,
		dna.ImpulseFrequency,
		dna.LuxuryDriftIndex,
		dna.LuxuryDriftDetected,
		topBrandsStr,
	)

	// Call LLM with 15-second timeout using goroutine + channel pattern
	type llmResult struct {
		response string
		err      error
	}
	ch := make(chan llmResult, 1)

	go func() {
		resp, err := s.deps.LLM.Complete(prompt)
		ch <- llmResult{resp, err}
	}()

	timeoutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var rawResponse string
	select {
	case res := <-ch:
		if res.err != nil {
			return nil, fmt.Errorf("agent strategist failed: %w", res.err)
		}
		rawResponse = res.response
	case <-timeoutCtx.Done():
		return nil, fmt.Errorf("agent strategist failed: LLM timeout after 15s")
	}

	// Parse JSON array from LLM response
	start := strings.Index(rawResponse, "[")
	end := strings.LastIndex(rawResponse, "]")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("agent strategist failed: no valid JSON array found in LLM response")
	}

	jsonArray := rawResponse[start : end+1]

	var recommendations []model.Recommendation
	if err := json.Unmarshal([]byte(jsonArray), &recommendations); err != nil {
		return nil, fmt.Errorf("agent strategist failed: invalid JSON array: %w", err)
	}

	// Validate each recommendation
	validRecommendations := make([]model.Recommendation, 0, len(recommendations))
	for _, rec := range recommendations {
		if rec.Title == "" || rec.Description == "" || rec.Priority == "" {
			continue // skip invalid recommendations
		}
		if rec.Priority != "high" && rec.Priority != "medium" && rec.Priority != "low" {
			continue // skip invalid priority values
		}
		validRecommendations = append(validRecommendations, rec)
	}

	// Append valid recommendations to context
	ctx.Recommendations = append(ctx.Recommendations, validRecommendations...)

	return &AgentResult{AgentName: "strategist", Data: ctx.Recommendations}, nil
}
