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
	p := prompt.Build(ctx.RawMessage, nil)

	rawResponse, err := a.deps.LLM.CompleteWithTokenLimit(p, prompt.MaxOutputTokens)
	if err != nil {
		return nil, fmt.Errorf("agent auditor failed: %w", err)
	}

	parsedResult, err := prompt.Parse(rawResponse)
	if err != nil {
		return nil, fmt.Errorf("agent auditor failed: %w", err)
	}

	ctx.ParsedTx = parsedResult

	normalizedUID := model.UserIDOrDefault(ctx.LineUserID)

	if a.deps.WalletRepo == nil {
		return nil, fmt.Errorf("agent auditor failed: wallet repository is nil")
	}

	currentWallet, err := a.deps.WalletRepo.GetCurrentWallet(normalizedUID)
	if err != nil {
		return nil, fmt.Errorf("agent auditor failed to get current wallet: %w", err)
	}

	parsed := parsedResult.ParsedTransaction

	txType := model.TransactionType(parsed.Type)
	if txType != model.Income && txType != model.Expense {
		txType = model.Expense
	}

	tx := &model.Transaction{
		UserID:      normalizedUID,
		WalletID:    &currentWallet.ID,
		RawMessage:  ctx.RawMessage,
		Type:        txType,
		Amount:      parsed.Amount,
		Category:    parsed.Category,
		SubCategory: parsed.SubCategory,
		Brand:       parsed.Brand,
		Description: parsed.Description,
		BehaviorTag: parsed.BehaviorTag,
	}

	if err := a.deps.TxRepo.Create(tx); err != nil {
		return nil, fmt.Errorf("agent auditor failed: %w", err)
	}

	ctx.SavedTx = tx

	return &AgentResult{AgentName: "auditor", Data: ctx.SavedTx}, nil
}
