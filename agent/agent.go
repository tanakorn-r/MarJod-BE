package agent

import (
	"fmt"

	"finance-chat/model"
	"finance-chat/prompt"
	"finance-chat/repository"
)

// ─────────────────────────────────────────────────────────────
// External service interfaces
// ─────────────────────────────────────────────────────────────

// LLMClient is a generic interface for any LLM backend.
// It knows nothing about finance — just sends a prompt and returns text.
type LLMClient interface {
	// Complete sends a prompt and waits for the full response.
	Complete(prompt string) (string, error)

	// CompleteWithTokenLimit is like Complete but enforces a hard output token cap.
	CompleteWithTokenLimit(prompt string, maxTokens int) (string, error)

	// CompleteStream sends a prompt and returns a channel of token strings.
	// The caller must drain the channel. A non-nil error is sent as the last
	// value prefixed with "error:" if something goes wrong mid-stream.
	CompleteStream(prompt string) (<-chan string, error)
}

// LineService handles LINE Messaging API interactions.
type LineService interface {
	VerifySignature(body []byte, signature string) bool
	ReplyMessage(replyToken, text string) error
	// VerifyIDToken validates a LIFF ID token against LINE's verify endpoint
	// and returns the LINE userId (the token's "sub" claim).
	VerifyIDToken(idToken string) (string, error)
}

// ─────────────────────────────────────────────────────────────
// Core interfaces and types
// ─────────────────────────────────────────────────────────────

// Agent is the common interface every agent in the pipeline must satisfy.
type Agent interface {
	Name() string
	Run(ctx *AgentContext) (*AgentResult, error)
}

// AgentContext is the shared mutable state passed through the pipeline.
// Each agent reads from and writes to this struct.
type AgentContext struct {
	RawMessage      string
	ParsedTx        *prompt.ParsedTransactionResult
	SavedTx         *model.Transaction
	BehaviorDNA     *model.BehaviorDNA
	SurpriseScore   int
	Alerts          []model.Alert
	Recommendations []model.Recommendation
	UserPlan        model.PlanName
	LLM             LLMClient
	LineService     LineService
	LineUserID      string // LINE user ID for push notifications
}

// AgentResult holds the output of a single agent run.
type AgentResult struct {
	AgentName string
	Data      any
}

// PipelineResult is the final output returned to the caller after all agents run.
type PipelineResult struct {
	Transaction     *model.Transaction     `json:"transaction"`
	BehaviorDNA     *model.BehaviorDNA     `json:"behavior_dna,omitempty"`
	Alerts          []model.Alert          `json:"alerts"`
	Recommendations []model.Recommendation `json:"recommendations"`
}

// ─────────────────────────────────────────────────────────────
// Pipeline
// ─────────────────────────────────────────────────────────────

// Pipeline executes a sequence of agents in order, sharing a single AgentContext.
type Pipeline struct {
	agents []Agent
}

// NewPipeline creates a Pipeline from the provided agents slice.
func NewPipeline(agents []Agent) *Pipeline {
	return &Pipeline{agents: agents}
}

// Run executes agents in order. It halts on the first error, wrapping it with
// the responsible agent's name. On success it returns a PipelineResult built
// from the final AgentContext state.
func (p *Pipeline) Run(ctx *AgentContext) (*PipelineResult, error) {
	for _, a := range p.agents {
		if _, err := a.Run(ctx); err != nil {
			return nil, fmt.Errorf("agent %q: %w", a.Name(), err)
		}
	}

	return &PipelineResult{
		Transaction:     ctx.SavedTx,
		BehaviorDNA:     ctx.BehaviorDNA,
		Alerts:          ctx.Alerts,
		Recommendations: ctx.Recommendations,
	}, nil
}

// ─────────────────────────────────────────────────────────────
// AgentDeps — shared dependencies injected into agents
// ─────────────────────────────────────────────────────────────

// AgentDeps bundles all external dependencies that agents may need.
type AgentDeps struct {
	LLM          LLMClient
	AnalyticsLLM LLMClient
	LineService  LineService
	TxRepo       repository.TransactionRepository
	ProfileRepo  repository.BehaviorProfileRepository
	PlanRepo     repository.UserPlanRepository
	WalletRepo   repository.WalletRepositoryInterface
}

// ─────────────────────────────────────────────────────────────
// BuildPipeline factory
// ─────────────────────────────────────────────────────────────

// BuildPipeline constructs the agent pipeline appropriate for the given plan:
//   - free    → Auditor only
//   - starter → Auditor + Profiler + Nagger
//   - pro     → Auditor + Profiler + Nagger + Strategist
func BuildPipeline(plan model.PlanName, deps AgentDeps) *Pipeline {
	switch plan {
	case model.PlanStarter:
		return NewPipeline([]Agent{
			NewAuditorAgent(deps),
			NewProfilerAgent(deps),
			NewNaggerAgent(deps),
		})
	case model.PlanPro:
		return NewPipeline([]Agent{
			NewAuditorAgent(deps),
			NewProfilerAgent(deps),
			NewNaggerAgent(deps),
			NewStrategistAgent(deps),
		})
	default: // free (and any unknown plan)
		return NewPipeline([]Agent{
			NewAuditorAgent(deps),
		})
	}
}
