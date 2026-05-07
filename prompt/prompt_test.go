package prompt_test

import (
	"finance-chat/model"
	"finance-chat/prompt"
	"strings"
	"testing"
)

// fullJSON returns a complete, valid JSON string for Parse tests.
// Callers can override individual fields by passing key-value pairs.
func fullJSON(overrides map[string]string) string {
	fields := map[string]string{
		"raw_message":  "starbucks 180",
		"type":         "expense",
		"amount":       "180",
		"category":     "Food & Beverage",
		"sub_category": "Coffee",
		"brand":        "Starbucks",
		"behavior_tag": "treat",
		"description":  "Coffee at Starbucks",
		"logic_gate":   "Verified: Starbucks → Food & Beverage.",
		"confidence":   "90",
	}
	for k, v := range overrides {
		fields[k] = v
	}
	return `{` +
		`"raw_message":"` + fields["raw_message"] + `",` +
		`"type":"` + fields["type"] + `",` +
		`"amount":` + fields["amount"] + `,` +
		`"category":"` + fields["category"] + `",` +
		`"sub_category":"` + fields["sub_category"] + `",` +
		`"brand":"` + fields["brand"] + `",` +
		`"behavior_tag":"` + fields["behavior_tag"] + `",` +
		`"description":"` + fields["description"] + `",` +
		`"logic_gate":"` + fields["logic_gate"] + `",` +
		`"confidence":` + fields["confidence"] +
		`}`
}

// ── Parse tests ───────────────────────────────────────────────────────────────

func TestParse_CleanJSON(t *testing.T) {
	raw := fullJSON(map[string]string{
		"raw_message":  "starbucks 180",
		"type":         "expense",
		"amount":       "180",
		"category":     "Food & Beverage",
		"sub_category": "Coffee",
		"brand":        "Starbucks",
		"behavior_tag": "treat",
		"description":  "Latte at Starbucks",
		"logic_gate":   "Verified: Starbucks → Food & Beverage.",
		"confidence":   "90",
	})

	got, err := prompt.Parse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertEqual(t, "type", "expense", got.Type)
	assertEqual(t, "amount", 180.0, got.Amount)
	assertEqual(t, "category", "Food & Beverage", got.Category)
	assertEqual(t, "sub_category", "Coffee", got.SubCategory)
	assertEqual(t, "brand", "Starbucks", got.Brand)
	assertEqual(t, "behavior_tag", "treat", got.BehaviorTag)
}

func TestParse_JSONWrappedInText(t *testing.T) {
	// LLM sometimes adds explanation before/after the JSON
	raw := `Sure! Here is the result:
` + fullJSON(map[string]string{
		"raw_message":  "kfc 250",
		"type":         "expense",
		"amount":       "250",
		"category":     "Food & Beverage",
		"sub_category": "Restaurant",
		"brand":        "KFC",
		"behavior_tag": "treat",
		"description":  "KFC meal",
		"logic_gate":   "Verified: KFC → Food & Beverage.",
		"confidence":   "92",
	}) + `
Hope that helps!`

	got, err := prompt.Parse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertEqual(t, "type", "expense", got.Type)
	assertEqual(t, "amount", 250.0, got.Amount)
	assertEqual(t, "brand", "KFC", got.Brand)
}

func TestParse_IncomeType(t *testing.T) {
	raw := fullJSON(map[string]string{
		"raw_message":  "เงินเดือน 50000",
		"type":         "income",
		"amount":       "50000",
		"category":     "Salary & Income",
		"sub_category": "Salary",
		"brand":        "Company",
		"behavior_tag": "recurring",
		"description":  "Monthly salary",
		"logic_gate":   "Verified: เงินเดือน → income.",
		"confidence":   "99",
	})

	got, err := prompt.Parse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertEqual(t, "type", "income", got.Type)
	assertEqual(t, "amount", 50000.0, got.Amount)
}

func TestParse_DecimalAmount(t *testing.T) {
	raw := fullJSON(map[string]string{
		"raw_message":  "shopee 99.50",
		"type":         "expense",
		"amount":       "99.50",
		"category":     "Shopping",
		"sub_category": "General",
		"brand":        "Shopee",
		"behavior_tag": "impulse",
		"description":  "Online purchase",
		"logic_gate":   "Verified: Shopee → Shopping.",
		"confidence":   "85",
	})

	got, err := prompt.Parse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Amount != 99.50 {
		t.Errorf("amount: want 99.50, got %v", got.Amount)
	}
}

func TestParse_BrandFallsBackToGeneral(t *testing.T) {
	// When brand is "unknown", Normalize replaces it with "General"
	raw := fullJSON(map[string]string{
		"brand": "unknown",
	})

	got, err := prompt.Parse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertEqual(t, "brand", "General", got.Brand)
}

func TestParse_NoJSON_ReturnsError(t *testing.T) {
	_, err := prompt.Parse("I cannot parse this message.")
	if err == nil {
		t.Fatal("expected error for response with no JSON, got nil")
	}
}

func TestParse_InvalidJSON_ReturnsError(t *testing.T) {
	_, err := prompt.Parse(`{"type": "expense", "amount": }`)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

// ── Validation tests ──────────────────────────────────────────────────────────

func TestParse_InvalidType_ReturnsError(t *testing.T) {
	raw := fullJSON(map[string]string{"type": "transfer"})
	_, err := prompt.Parse(raw)
	if err == nil {
		t.Fatal("expected error for invalid type, got nil")
	}
	if !strings.Contains(err.Error(), "type") {
		t.Errorf("error should mention field name 'type', got: %v", err)
	}
}

func TestParse_InvalidBehaviorTag_ReturnsError(t *testing.T) {
	raw := fullJSON(map[string]string{"behavior_tag": "cafe"})
	_, err := prompt.Parse(raw)
	if err == nil {
		t.Fatal("expected error for invalid behavior_tag, got nil")
	}
	if !strings.Contains(err.Error(), "behavior_tag") {
		t.Errorf("error should mention field name 'behavior_tag', got: %v", err)
	}
}

func TestParse_ConfidenceOutOfRange_ReturnsError(t *testing.T) {
	raw := fullJSON(map[string]string{"confidence": "150"})
	_, err := prompt.Parse(raw)
	if err == nil {
		t.Fatal("expected error for confidence out of range, got nil")
	}
	if !strings.Contains(err.Error(), "confidence") {
		t.Errorf("error should mention field name 'confidence', got: %v", err)
	}
}

func TestParse_MissingRequiredField_ReturnsError(t *testing.T) {
	// Build JSON without logic_gate by constructing manually
	raw := `{"raw_message":"test","type":"expense","amount":100,"category":"Food & Beverage","sub_category":"Coffee","brand":"Starbucks","behavior_tag":"treat","description":"Coffee","confidence":90}`
	_, err := prompt.Parse(raw)
	if err == nil {
		t.Fatal("expected error for missing logic_gate, got nil")
	}
	if !strings.Contains(err.Error(), "logic_gate") {
		t.Errorf("error should mention missing field 'logic_gate', got: %v", err)
	}
}

func TestParse_LowConfidence_SetFlag(t *testing.T) {
	raw := fullJSON(map[string]string{"confidence": "45"})
	got, err := prompt.Parse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.LowConfidence {
		t.Error("LowConfidence should be true when confidence < 60")
	}
}

func TestParse_HighConfidence_FlagFalse(t *testing.T) {
	raw := fullJSON(map[string]string{"confidence": "90"})
	got, err := prompt.Parse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.LowConfidence {
		t.Error("LowConfidence should be false when confidence >= 60")
	}
}

// ── Build tests ───────────────────────────────────────────────────────────────

func TestBuild_ContainsMessage(t *testing.T) {
	p := prompt.Build("starbucks 180 baht", nil)
	if !strings.Contains(p, "starbucks 180 baht") {
		t.Error("prompt should contain the original message")
	}
}

func TestBuild_ContainsRequiredFields(t *testing.T) {
	p := prompt.Build("test", nil)
	requiredFields := []string{"type", "amount", "category", "sub_category", "brand", "behavior_tag", "description", "logic_gate", "confidence"}
	for _, f := range requiredFields {
		if !strings.Contains(p, f) {
			t.Errorf("prompt missing required field: %s", f)
		}
	}
}

func TestBuild_ContainsCorrectionContext(t *testing.T) {
	corrections := []model.UserCorrection{
		{RawMessage: "starbucks latte", Category: "Food & Beverage", SubCategory: "Coffee", Brand: "Starbucks", BehaviorTag: "treat"},
	}
	p := prompt.Build("starbucks 180", corrections)
	if !strings.Contains(p, "starbucks") {
		t.Error("prompt should contain correction keyword")
	}
	if !strings.Contains(p, "Food & Beverage") {
		t.Error("prompt should contain correction category")
	}
}

func TestBuild_NoCorrectionSection_WhenEmpty(t *testing.T) {
	p := prompt.Build("test message", nil)
	if strings.Contains(p, "USER LEARNING RULE") {
		t.Error("prompt should not contain correction section when no corrections provided")
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func assertEqual[T comparable](t *testing.T, field string, want, got T) {
	t.Helper()
	if want != got {
		t.Errorf("%s: want %v, got %v", field, want, got)
	}
}
