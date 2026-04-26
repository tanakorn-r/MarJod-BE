package service

import (
	"finance-chat/model"
	"finance-chat/prompt"
	"finance-chat/repository"
	"fmt"
	"strings"
	"sync"
)

type TransactionService interface {
	Chat(message string) (*model.Transaction, error)
	ChatStream(message string) (<-chan string, <-chan *model.Transaction, <-chan error)
	Correct(transactionID uint, category, subCategory, brand, behaviorTag string) error
	List() ([]model.Transaction, error)
	Delete(id uint) error
	Summary() (*Summary, error)
}

type Summary struct {
	TotalIncome  float64 `json:"total_income"`
	TotalExpense float64 `json:"total_expense"`
	Balance      float64 `json:"balance"`
}

type transactionService struct {
	repo       repository.TransactionRepository
	correction repository.CorrectionRepository
	llm        LLMClient
}

var (
	txServiceInstance TransactionService
	txServiceOnce     sync.Once
)

func NewTransactionService(
	repo repository.TransactionRepository,
	correction repository.CorrectionRepository,
	llm LLMClient,
) TransactionService {
	txServiceOnce.Do(func() {
		txServiceInstance = &transactionService{repo: repo, correction: correction, llm: llm}
	})
	return txServiceInstance
}

// NewTransactionServiceDirect creates a new instance without the singleton —
// intended for use in tests only.
func NewTransactionServiceDirect(
	repo repository.TransactionRepository,
	correction repository.CorrectionRepository,
	llm LLMClient,
) TransactionService {
	return &transactionService{repo: repo, correction: correction, llm: llm}
}

func (s *transactionService) Chat(message string) (*model.Transaction, error) {
	corrections, _ := s.correction.FindRecent(100)
	raw, err := s.llm.Complete(prompt.Build(message, corrections))
	if err != nil {
		return nil, fmt.Errorf("llm call failed: %w", err)
	}
	return s.buildAndSave(message, raw)
}

func (s *transactionService) ChatStream(message string) (<-chan string, <-chan *model.Transaction, <-chan error) {
	tokens := make(chan string, 32)
	done := make(chan *model.Transaction, 1)
	errc := make(chan error, 1)

	corrections, _ := s.correction.FindRecent(20)
	streamCh, err := s.llm.CompleteStream(prompt.Build(message, corrections))
	if err != nil {
		errc <- fmt.Errorf("llm stream failed: %w", err)
		close(tokens)
		close(done)
		close(errc)
		return tokens, done, errc
	}

	go func() {
		defer close(tokens)
		defer close(errc)

		var sb strings.Builder
		for token := range streamCh {
			if strings.HasPrefix(token, "error:") {
				errc <- fmt.Errorf("stream error: %s", token)
				close(done)
				return
			}
			sb.WriteString(token)
			tokens <- token
		}

		tx, err := s.buildAndSave(message, sb.String())
		if err != nil {
			errc <- err
			close(done)
			return
		}
		done <- tx
		close(done)
	}()

	return tokens, done, errc
}

// Correct updates a transaction's classification and saves it as a correction
// so the AI learns from it on future requests.
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

func (s *transactionService) buildAndSave(message, raw string) (*model.Transaction, error) {
	parsed, err := prompt.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to parse llm response: %w", err)
	}

	txType := model.TransactionType(parsed.Type)
	if txType != model.Income && txType != model.Expense {
		txType = model.Expense
	}

	t := &model.Transaction{
		RawMessage:  message,
		Type:        txType,
		Amount:      parsed.Amount,
		Category:    parsed.Category,
		SubCategory: parsed.SubCategory,
		Brand:       parsed.Brand,
		Description: parsed.Description,
		BehaviorTag: parsed.BehaviorTag,
	}

	if err := s.repo.Create(t); err != nil {
		return nil, fmt.Errorf("failed to save transaction: %w", err)
	}
	return t, nil
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
