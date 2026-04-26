package prompt_test

import (
	"finance-chat/model"
	"finance-chat/prompt"
	"strings"
	"testing"
)

// ── Parse tests ───────────────────────────────────────────────────────────────

func TestParse_CleanJSON(t *testing.T) {
	raw := `{
		"type": "expense",
		"amount": 180,
		"category": "Food & Beverage",
		"sub_category": "Coffee",
		"brand": "Starbucks",
		"description": "Latte at Starbucks",
		"behavior_tag": "cafe"
	}`

	got, err := prompt.Parse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertEqual(t, "type", "expense", got.Type)
	assertEqual(t, "amount", 180.0, got.Amount)
	assertEqual(t, "category", "Food & Beverage", got.Category)
	assertEqual(t, "sub_category", "Coffee", got.SubCategory)
	assertEqual(t, "brand", "Starbucks", got.Brand)
	assertEqual(t, "behavior_tag", "cafe", got.BehaviorTag)
}

func TestParse_JSONWrappedInText(t *testing.T) {
	// LLM sometimes adds explanation before/after the JSON
	raw := `Sure! Here is the result:
{"type":"expense","amount":250,"category":"Food & Beverage","sub_category":"Restaurant","brand":"KFC","description":"KFC meal","behavior_tag":"dining"}
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
	raw := `{"type":"income","amount":50000,"category":"Salary & Income","sub_category":"Salary","brand":"Company","description":"Monthly salary","behavior_tag":"income"}`

	got, err := prompt.Parse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertEqual(t, "type", "income", got.Type)
	assertEqual(t, "amount", 50000.0, got.Amount)
}

func TestParse_DecimalAmount(t *testing.T) {
	raw := `{"type":"expense","amount":99.50,"category":"Shopping","sub_category":"General","brand":"Shopee","description":"Online purchase","behavior_tag":"shopping"}`

	got, err := prompt.Parse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Amount != 99.50 {
		t.Errorf("amount: want 99.50, got %v", got.Amount)
	}
}

func TestParse_EmptyBrand(t *testing.T) {
	raw := `{"type":"expense","amount":60,"category":"Food & Beverage","sub_category":"Drink","brand":"","description":"Soft drink","behavior_tag":"daily"}`

	got, err := prompt.Parse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertEqual(t, "brand", "", got.Brand)
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

// ── Build tests ───────────────────────────────────────────────────────────────

func TestBuild_ContainsMessage(t *testing.T) {
	p := prompt.Build("starbucks 180 baht", nil)
	if !strings.Contains(p, "starbucks 180 baht") {
		t.Error("prompt should contain the original message")
	}
}

func TestBuild_ContainsRequiredFields(t *testing.T) {
	p := prompt.Build("test", nil)
	requiredFields := []string{"type", "amount", "category", "sub_category", "brand", "behavior_tag", "description"}
	for _, f := range requiredFields {
		if !strings.Contains(p, f) {
			t.Errorf("prompt missing required field: %s", f)
		}
	}
}

func TestBuild_ContainsCorrectionContext(t *testing.T) {
	corrections := []model.UserCorrection{
		{RawMessage: "starbucks latte", Category: "Food & Beverage", SubCategory: "Coffee", Brand: "Starbucks", BehaviorTag: "cafe"},
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
