package agent

import (
	"fmt"

	"finance-chat/model"
	"finance-chat/prompt"
)

// auditorAgent is the first agent in the pipeline. It parses the raw user
// message into a structured transaction and persists it to the database.
type auditorAgent struct {
	deps AgentDeps
}

// NewAuditorAgent returns a fully-wired Auditor agent.
func NewAuditorAgent(deps AgentDeps) Agent {
	return &auditorAgent{deps: deps}
}

// Name satisfies the Agent interface.
func (a *auditorAgent) Name() string { return "auditor" }

// Run executes the Auditor pipeline step:
//  1. Build the strict Auditor prompt from the raw message.
//  2. Call the LLM to get a JSON response.
//  3. Parse and validate the JSON response into a ParsedTransactionResult.
//  4. Build a model.Transaction from the parsed result and persist it.
//  5. Store both the parsed result and the saved transaction in the context.
func (a *auditorAgent) Run(ctx *AgentContext) (*AgentResult, error) {
	// 1. Build prompt — the Auditor uses the strict system prompt embedded in prompt.Build()
	p := prompt.Build(ctx.RawMessage, nil)

	// 2. Call LLM
	rawResponse, err := a.deps.LLM.Complete(p)
	if err != nil {
		return nil, fmt.Errorf("agent auditor failed: %w", err)
	}

	// 3. Parse and validate the LLM response
	parsedResult, err := prompt.Parse(rawResponse)
	if err != nil {
		return nil, fmt.Errorf("agent auditor failed: %w", err)
	}

	// 4. Store parsed result in context
	ctx.ParsedTx = parsedResult

	// 5. Build transaction model from parsed result
	parsed := parsedResult.ParsedTransaction
	txType := model.TransactionType(parsed.Type)
	if txType != model.Income && txType != model.Expense {
		txType = model.Expense
	}

	tx := &model.Transaction{
		RawMessage:  ctx.RawMessage,
		Type:        txType,
		Amount:      parsed.Amount,
		Category:    parsed.Category,
		SubCategory: parsed.SubCategory,
		Brand:       parsed.Brand,
		Description: parsed.Description,
		BehaviorTag: parsed.BehaviorTag,
	}

	// 6. Persist the transaction
	if err := a.deps.TxRepo.Create(tx); err != nil {
		return nil, fmt.Errorf("agent auditor failed: %w", err)
	}

	// 7. Store saved transaction in context
	ctx.SavedTx = tx

	return &AgentResult{AgentName: "auditor", Data: ctx.SavedTx}, nil
}
