package prompt

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"finance-chat/model"
)

const (
	// MaxInputChars caps the raw user message before it reaches the LLM.
	MaxInputChars = 100
	// MaxOutputTokens caps the LLM response for the Auditor — a single JSON
	// object never needs more than ~200 tokens.
	MaxOutputTokens = 220
)

// ─────────────────────────────────────────────────────────────
// DTOs
// ─────────────────────────────────────────────────────────────

// ParsedTransaction is the raw output from the LLM, mapped 1-to-1 with the
// JSON schema enforced by the Auditor system prompt.
type ParsedTransaction struct {
	RawMessage  string  `json:"raw_message"`
	Type        string  `json:"type"` // "income" | "expense"
	Amount      float64 `json:"amount"`
	Category    string  `json:"category"`
	SubCategory string  `json:"sub_category"`
	Brand       string  `json:"brand"`
	BehaviorTag string  `json:"behavior_tag"` // "impulse"|"necessity"|"social"|"treat"|"recurring"
	Description string  `json:"description"`
	LogicGate   string  `json:"logic_gate"`
	Confidence  int     `json:"confidence"` // 1–100
}

// ParsedTransactionResult wraps ParsedTransaction with post-parse metadata.
type ParsedTransactionResult struct {
	*ParsedTransaction
	LowConfidence bool `json:"low_confidence"` // true if confidence < 60
}

// ─────────────────────────────────────────────────────────────
// PROMPT BUILDER
// ─────────────────────────────────────────────────────────────

// Build constructs the full Auditor prompt.
// The message is truncated to MaxInputChars before embedding.
func Build(message string, corrections []model.UserCorrection) string {
	// Hard-cap input to control token spend
	if len([]rune(message)) > MaxInputChars {
		runes := []rune(message)
		message = string(runes[:MaxInputChars])
	}

	var sb strings.Builder
	sb.WriteString(basePrompt())

	// Inject up to 3 user corrections (each is ~15 tokens)
	if len(corrections) > 0 {
		limit := min(len(corrections), 3)
		sb.WriteString("\nUSER PREFS:\n")
		for _, c := range corrections[:limit] {
			fmt.Fprintf(&sb, "- \"%s\"→cat:%s sub:%s tag:%s\n",
				extractKeyword(c.RawMessage), c.Category, c.SubCategory, c.BehaviorTag)
		}
	}

	fmt.Fprintf(&sb, "\nInput:\"%s\"\nJSON:", message)
	return sb.String()
}

// ─────────────────────────────────────────────────────────────
// BASE PROMPT — compact, schema-only, single example
// ─────────────────────────────────────────────────────────────

func basePrompt() string {
	return `You are a transaction classifier. Return ONLY valid JSON with ALL fields.

Schema:
{"raw_message":str,"type":"income"|"expense","amount":num,"category":str,"sub_category":str,"brand":str,"behavior_tag":"impulse"|"necessity"|"social"|"treat"|"recurring","description":str,"logic_gate":str,"confidence":1-100}

behavior_tag meanings:
- impulse: unplanned, emotional buy
- necessity: essential (food, transport, bills, medicine)
- social: with friends/colleagues (dining out, gifts)
- treat: planned self-reward (coffee, dessert, entertainment)
- recurring: regular fixed expense (salary, subscriptions, rent)

Rules: all 10 fields required. brand="General" if unknown. logic_gate = brief audit reason.

Examples:
Input: กาแฟ Starbucks 180
Output: {"raw_message":"กาแฟ Starbucks 180","type":"expense","amount":180,"category":"Food & Beverage","sub_category":"Coffee","brand":"Starbucks","behavior_tag":"treat","description":"Coffee at Starbucks 180 THB","logic_gate":"Starbucks→Food. กาแฟ→Coffee. Planned coffee→treat.","confidence":97}

Input: เงินเดือน 35000
Output: {"raw_message":"เงินเดือน 35000","type":"income","amount":35000,"category":"Salary & Income","sub_category":"Salary","brand":"General","behavior_tag":"recurring","description":"Monthly salary 35,000 THB","logic_gate":"เงินเดือน→income. Fixed monthly→recurring.","confidence":99}

Input: ข้าวกับเพื่อน 250
Output: {"raw_message":"ข้าวกับเพื่อน 250","type":"expense","amount":250,"category":"Food & Beverage","sub_category":"Dining","brand":"General","behavior_tag":"social","description":"Lunch with friends 250 THB","logic_gate":"ข้าว→Food. กับเพื่อน→social dining.","confidence":90}`
}

// ─────────────────────────────────────────────────────────────
// VALIDATION
// ─────────────────────────────────────────────────────────────

var validTypes = map[string]bool{
	"income":  true,
	"expense": true,
}

var validBehaviorTags = map[string]bool{
	"impulse":   true,
	"necessity": true,
	"social":    true,
	"treat":     true,
	"recurring": true,
}

func validateParsedTransaction(r *ParsedTransaction) error {
	if r.RawMessage == "" {
		return fmt.Errorf("missing required field: raw_message")
	}
	if r.Type == "" {
		return fmt.Errorf("missing required field: type")
	}
	if r.Amount == 0 {
		return fmt.Errorf("missing required field: amount")
	}
	if r.Category == "" {
		return fmt.Errorf("missing required field: category")
	}
	if r.SubCategory == "" {
		return fmt.Errorf("missing required field: sub_category")
	}
	if r.Brand == "" {
		return fmt.Errorf("missing required field: brand")
	}
	if r.BehaviorTag == "" {
		return fmt.Errorf("missing required field: behavior_tag")
	}
	if r.Description == "" {
		return fmt.Errorf("missing required field: description")
	}
	if r.LogicGate == "" {
		return fmt.Errorf("missing required field: logic_gate")
	}
	if r.Confidence == 0 {
		return fmt.Errorf("missing required field: confidence")
	}
	if !validTypes[r.Type] {
		return fmt.Errorf("invalid type: %q (must be income or expense)", r.Type)
	}
	if !validBehaviorTags[r.BehaviorTag] {
		return fmt.Errorf("invalid behavior_tag: %q", r.BehaviorTag)
	}
	if r.Confidence < 1 || r.Confidence > 100 {
		return fmt.Errorf("invalid confidence: %d (must be 1–100)", r.Confidence)
	}
	return nil
}

// ─────────────────────────────────────────────────────────────
// PARSE & NORMALIZE
// ─────────────────────────────────────────────────────────────

func Parse(raw string) (*ParsedTransactionResult, error) {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no valid JSON found in LLM response")
	}

	var result ParsedTransaction
	if err := json.Unmarshal([]byte(raw[start:end+1]), &result); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if err := validateParsedTransaction(&result); err != nil {
		return nil, err
	}

	normalized := Normalize(&result)
	return &ParsedTransactionResult{
		ParsedTransaction: normalized,
		LowConfidence:     normalized.Confidence < 60,
	}, nil
}

func Normalize(r *ParsedTransaction) *ParsedTransaction {
	r.Amount = math.Abs(r.Amount)

	// Standardize category — preserve LLM value if it's already a known category
	known := map[string]string{
		"food":          "Food & Beverage",
		"beverage":      "Food & Beverage",
		"drink":         "Food & Beverage",
		"restaurant":    "Food & Beverage",
		"hotel":         "Hotel",
		"transport":     "Transport",
		"travel":        "Transport",
		"taxi":          "Transport",
		"bill":          "Bill",
		"utility":       "Bill",
		"electric":      "Bill",
		"shopping":      "Shopping",
		"shop":          "Shopping",
		"clothing":      "Shopping",
		"health":        "Health",
		"medical":       "Health",
		"entertainment": "Entertainment",
		"salary":        "Salary & Income",
		"income":        "Salary & Income",
	}
	cat := strings.ToLower(r.Category)
	for key, normalized := range known {
		if strings.Contains(cat, key) {
			r.Category = normalized
			break
		}
	}

	r.Brand = strings.TrimSpace(r.Brand)
	if r.Brand == "" || strings.EqualFold(r.Brand, "unknown") {
		r.Brand = "General"
	}

	if strings.TrimSpace(r.Description) == "" {
		r.Description = fmt.Sprintf("%s at %s", r.SubCategory, r.Brand)
	}

	return r
}

// ─────────────────────────────────────────────────────────────
// HELPERS
// ─────────────────────────────────────────────────────────────

func extractKeyword(input string) string {
	stopWords := map[string]bool{"ค่า": true, "กิน": true, "ซื้อ": true, "ไป": true, "บาท": true}
	words := strings.Fields(strings.ToLower(input))
	var result []string
	for _, w := range words {
		if _, err := strconv.ParseFloat(w, 64); err == nil {
			continue
		}
		if stopWords[w] {
			continue
		}
		result = append(result, w)
	}
	if len(result) == 0 {
		return input
	}
	return strings.Join(result, " ")
}
