package prompt

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"finance-chat/model"
)

// ─────────────────────────────────────────────────────────────
// DTOs
// ─────────────────────────────────────────────────────────────

// ParsedTransaction is the raw output from the LLM, mapped 1-to-1 with the
// JSON schema enforced by the Auditor system prompt.
type ParsedTransaction struct {
	RawMessage  string  `json:"raw_message"`
	Type        string  `json:"type"`        // "income" | "expense"
	Amount      float64 `json:"amount"`
	Category    string  `json:"category"`
	SubCategory string  `json:"sub_category"`
	Brand       string  `json:"brand"`
	BehaviorTag string  `json:"behavior_tag"` // "impulse"|"necessity"|"social"|"treat"|"recurring"
	Description string  `json:"description"`
	LogicGate   string  `json:"logic_gate"`  // e.g. "Verified: Uniqlo → Shopping. Shirt → Clothing."
	Confidence  int     `json:"confidence"`  // 1–100
}

// ParsedTransactionResult wraps ParsedTransaction with post-parse metadata.
type ParsedTransactionResult struct {
	*ParsedTransaction
	LowConfidence bool `json:"low_confidence"` // true if confidence < 60
}

// ─────────────────────────────────────────────────────────────
// PROMPT BUILDER
// ─────────────────────────────────────────────────────────────

func Build(message string, corrections []model.UserCorrection) string {
	var sb strings.Builder

	sb.WriteString(basePrompt())

	// Inject user learning (Limited to last 5 to save tokens/focus)
	if len(corrections) > 0 {
		sb.WriteString("\n----------------------------------------\n")
		sb.WriteString("USER PREFERENCES (Prioritize these patterns):\n")

		limit := len(corrections)
		if limit > 5 {
			limit = 5
		}

		for _, c := range corrections[0:limit] {
			sb.WriteString(fmt.Sprintf(
				`- When input is like "%s" → use category: %s, sub: %s, tag: %s`+"\n",
				extractKeyword(c.RawMessage),
				c.Category,
				c.SubCategory,
				c.BehaviorTag,
			))
		}
	}

	sb.WriteString(fmt.Sprintf(`
----------------------------------------
FINAL TASK:
Parse this input into ONE JSON object. Use Thai context for brands.

Input: "%s"

Return ONLY JSON.
`, message))

	return sb.String()
}

// ─────────────────────────────────────────────────────────────
// BASE PROMPT — The Chief Auditor (strict JSON schema)
// ─────────────────────────────────────────────────────────────

func basePrompt() string {
	return `You are The Chief Auditor. Precise. Terse. Audit-focused.
Your sole output is a single valid JSON object. No prose. No markdown. No explanation.

### REQUIRED JSON SCHEMA

{
  "raw_message":   string   — the original user input, verbatim,
  "type":          string   — MUST be exactly "income" or "expense",
  "amount":        number   — positive numeric value (no currency symbols),
  "category":      string   — top-level spending domain (e.g. "Food & Beverage", "Shopping", "Transport", "Bill", "Health", "Entertainment", "Salary & Income"),
  "sub_category":  string   — logical subset of category (e.g. "Coffee", "Shirt", "Taxi"),
  "brand":         string   — real-world brand or merchant name (e.g. "Starbucks", "Uniqlo", "Grab"); use "General" if unknown,
  "behavior_tag":  string   — MUST be exactly one of: "impulse", "necessity", "social", "treat", "recurring",
  "description":   string   — one concise sentence describing the transaction,
  "logic_gate":    string   — your audit trail, e.g. "Verified: Uniqlo → Shopping. Shirt → Clothing.",
  "confidence":    integer  — your certainty score, integer between 1 and 100 (inclusive)
}

### FIELD CONSTRAINTS (STRICT)
- "type": only "income" or "expense" — any other value is REJECTED.
- "behavior_tag": only "impulse", "necessity", "social", "treat", or "recurring" — any other value is REJECTED.
- "confidence": integer 1–100 — decimals and out-of-range values are REJECTED.
- NEVER omit any field. All 10 fields are mandatory.
- Return ONLY valid JSON. No text before or after the JSON object.

### FEW-SHOT EXAMPLES (Thai context)

Example 1 — Clothing purchase at Uniqlo:
Input: "ซื้อเสื้อ Uniqlo 590 บาท"
{
  "raw_message":  "ซื้อเสื้อ Uniqlo 590 บาท",
  "type":         "expense",
  "amount":       590,
  "category":     "Shopping",
  "sub_category": "Clothing",
  "brand":        "Uniqlo",
  "behavior_tag": "treat",
  "description":  "Purchased a shirt at Uniqlo for 590 THB.",
  "logic_gate":   "Verified: Uniqlo → Shopping. เสื้อ (shirt) → Clothing. Price 590 THB is moderate treat.",
  "confidence":   95
}

Example 2 — Coffee at Starbucks:
Input: "กาแฟ starbucks 180"
{
  "raw_message":  "กาแฟ starbucks 180",
  "type":         "expense",
  "amount":       180,
  "category":     "Food & Beverage",
  "sub_category": "Coffee",
  "brand":        "Starbucks",
  "behavior_tag": "treat",
  "description":  "Coffee at Starbucks for 180 THB.",
  "logic_gate":   "Verified: Starbucks → Food & Beverage. กาแฟ (coffee) → Coffee. Flagged: premium price → treat.",
  "confidence":   97
}

Example 3 — Monthly salary income:
Input: "เงินเดือน 35000"
{
  "raw_message":  "เงินเดือน 35000",
  "type":         "income",
  "amount":       35000,
  "category":     "Salary & Income",
  "sub_category": "Salary",
  "brand":        "General",
  "behavior_tag": "recurring",
  "description":  "Monthly salary of 35,000 THB.",
  "logic_gate":   "Verified: เงินเดือน → income. Salary → recurring. Rejected: expense classification.",
  "confidence":   99
}

Example 4 — Grab taxi ride:
Input: "grab ไปออฟฟิศ 85 บาท"
{
  "raw_message":  "grab ไปออฟฟิศ 85 บาท",
  "type":         "expense",
  "amount":       85,
  "category":     "Transport",
  "sub_category": "Taxi",
  "brand":        "Grab",
  "behavior_tag": "necessity",
  "description":  "Grab taxi to office for 85 THB.",
  "logic_gate":   "Verified: Grab → Transport. ไปออฟฟิศ (to office) → necessity commute. Flagged: daily pattern → necessity.",
  "confidence":   96
}

Example 5 — Electricity bill:
Input: "ค่าไฟ 1200"
{
  "raw_message":  "ค่าไฟ 1200",
  "type":         "expense",
  "amount":       1200,
  "category":     "Bill",
  "sub_category": "Electricity",
  "brand":        "MEA",
  "behavior_tag": "recurring",
  "description":  "Monthly electricity bill of 1,200 THB.",
  "logic_gate":   "Verified: ค่าไฟ → Bill. Electricity → recurring utility. Rejected: impulse/treat classification.",
  "confidence":   98
}
`
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

// validateParsedTransaction checks all required fields and enum constraints.
func validateParsedTransaction(r *ParsedTransaction) error {
	// Check all 10 required fields are non-zero/non-empty
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

	// Validate type enum
	if !validTypes[r.Type] {
		return fmt.Errorf("invalid value for field type: %q (must be \"income\" or \"expense\")", r.Type)
	}

	// Validate behavior_tag enum
	if !validBehaviorTags[r.BehaviorTag] {
		return fmt.Errorf("invalid value for field behavior_tag: %q (must be one of: impulse, necessity, social, treat, recurring)", r.BehaviorTag)
	}

	// Validate confidence range [1, 100]
	if r.Confidence < 1 || r.Confidence > 100 {
		return fmt.Errorf("invalid value for field confidence: %d (must be between 1 and 100)", r.Confidence)
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
		return nil, fmt.Errorf("no valid JSON found")
	}

	clean := raw[start : end+1]

	var result ParsedTransaction
	if err := json.Unmarshal([]byte(clean), &result); err != nil {
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
	// 1. Amount Safety
	r.Amount = math.Abs(r.Amount)

	// 2. Category Normalization (Standardize strings)
	cat := strings.ToLower(r.Category)
	if strings.Contains(cat, "food") || strings.Contains(cat, "drink") {
		r.Category = "Food & Beverage"
	} else if strings.Contains(cat, "transport") || strings.Contains(cat, "travel") {
		r.Category = "Transport"
	} else if strings.Contains(cat, "bill") || strings.Contains(cat, "utility") {
		r.Category = "Bill"
	} else if strings.Contains(cat, "shop") {
		r.Category = "Shopping"
	} else {
		r.Category = "Other"
	}

	// 3. Brand Cleanup
	r.Brand = strings.TrimSpace(r.Brand)
	if r.Brand == "" || strings.ToLower(r.Brand) == "unknown" {
		r.Brand = "General"
	}

	// 4. Description Fix (Anti-hallucination)
	// We want to ensure the description actually reflects reality
	if strings.TrimSpace(r.Description) == "" {
		r.Description = fmt.Sprintf("%s at %s", r.SubCategory, r.Brand)
	}

	return r
}

// ─────────────────────────────────────────────────────────────
// HELPERS
// ─────────────────────────────────────────────────────────────

func extractKeyword(input string) string {
	// Simple cleaner to extract the "core" of a correction
	// e.g. "Buy coffee at bts" -> "coffee bts"
	stopWords := map[string]bool{"ค่า": true, "กิน": true, "ซื้อ": true, "ไป": true, "บาท": true}
	words := strings.Fields(strings.ToLower(input))
	var result []string

	for _, w := range words {
		if _, err := strconv.ParseFloat(w, 64); err == nil {
			continue // skip numbers
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
