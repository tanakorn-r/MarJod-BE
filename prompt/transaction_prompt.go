package prompt

import (
	"encoding/json"
	"finance-chat/model"
	"fmt"
	"strconv"
	"strings"
)

// ParsedTransaction is the structured result we expect from the LLM.
type ParsedTransaction struct {
	Type        string  `json:"type"`
	Amount      float64 `json:"amount"`
	Category    string  `json:"category"`
	SubCategory string  `json:"sub_category"`
	Brand       string  `json:"brand"`
	Description string  `json:"description"`
	BehaviorTag string  `json:"behavior_tag"`
}

// Build returns the prompt, optionally enriched with past user corrections.
func Build(message string, corrections []model.UserCorrection) string {
	var sb strings.Builder

	sb.WriteString(`You are a financial transaction parser and behavior analyzer.

Return ONLY valid JSON. No explanation. No extra text.

You MUST fill every field. NEVER leave anything empty.
If unsure, make the best reasonable guess.

----------------------------------------
OUTPUT FORMAT:

{
  "raw_message": "",
  "type": "",
  "amount": 0,
  "category": "",
  "sub_category": "",
  "brand": "",
  "behavior_tag": "",
  "description": ""
}

----------------------------------------
TYPE:

- "expense" (default)
- "income" ONLY if clearly receiving money

Income keywords:
เงินเดือน, โบนัส, ขาย, รายได้, ได้รับ, refund, salary

----------------------------------------
AMOUNT:

- Extract number from input
- Always positive number

----------------------------------------
CATEGORY (choose ONE only):

- Food & Beverage
- Transport
- Bill
- Shopping
- Other

----------------------------------------
SUB CATEGORY (important for analytics):

Food & Beverage:
- Coffee
- Restaurant
- Drink
- Convenience

Transport:
- Fuel
- Taxi
- Public Transport
- Travel (flight / hotel)

Bill:
- Rent
- Utility
- Subscription
- Installment

Shopping:
- Clothing
- General
- Grocery

Other:
- Entertainment
- Health
- Education
- Investment

----------------------------------------
BEHAVIOR TAG (VERY IMPORTANT for insights):

Choose ONE:

- essential        (rent, bills, water, electricity)
- daily            (food, convenience store)
- cafe             (coffee, cafe lifestyle)
- dining           (restaurants)
- transport        (fuel, taxi)
- subscription     (netflix, spotify)
- shopping         (products, clothes)
- entertainment    (movies, theme park, events)
- travel           (flight, hotel)
- health           (hospital, gym)
- investment       (stocks, gold, crypto)
- income

----------------------------------------
THAI UNDERSTANDING RULES (CRITICAL):

1. "ค่า" means paying money (expense), BUT:

   - If service/subscription → Bill
     examples: ค่าไฟ, ค่าเน็ต, ค่าโทรศัพท์, ค่า netflix

   - If product → Shopping
     examples: ค่าเสื้อ, ค่ารองเท้า

   - If hotel/flight → Transport (Travel)
     examples: ค่าโรงแรม, ค่าตั๋วเครื่องบิน

   - If ticket/entry → Other (Entertainment)
     examples: ค่าเข้า disney, ค่าหนัง

----------------------------------------
BRAND RULE (VERY IMPORTANT):

- Extract the main entity, shop, or name
- English words are usually brand
- If unknown → use meaningful words from input
- NEVER leave blank

Examples:
- "Starbucks 200" → Starbucks
- "ค่า netflix 400" → Netflix
- "rawmat 60" → rawmat
- "ค่าคอนโด 12000" → Condo

----------------------------------------
DESCRIPTION:

Short and clear:
- "<subcategory> at <brand>"
OR
- "<subcategory> expense"

----------------------------------------
LEARNING CONTEXT (IMPORTANT):

User may use slang, typo, or mixed language.
You MUST generalize meaning.

Examples:
- "rawmat" → treat as brand
- "eat am are" → restaurant
- "pt" → fuel station
- "lawson" → convenience store

----------------------------------------

----------------------------------------
DESCRIPTION (VERY IMPORTANT):

Preserve important details from the original input.

Rules:
- Keep specific item names (americano, latte, burger, etc.)
- Include brand/location if present
- Keep it short but meaningful

Format priority:
1. "<specific item> at <brand>"
2. "<specific item>"
3. "<subcategory> at <brand>" (only if no item found)

Examples:

Input: ค่ากาแฟ americano 7-11 25
→ "Americano at 7-11"

Input: กิน KFC 200
→ "KFC meal"

Input: ค่า netflix 400
→ "Netflix subscription"

Input: น้ำมัน shell 1200
→ "Fuel at Shell"


EXAMPLES:

Input: ค่าเสื้อ Arrow 2500
Output:
{
  "raw_message": "ค่าเสื้อ Arrow 2500",
  "type": "expense",
  "amount": 2500,
  "category": "Shopping",
  "sub_category": "Clothing",
  "brand": "Arrow",
  "behavior_tag": "shopping",
  "description": "Clothing at Arrow"
}

Input: ค่า netflix 400
Output:
{
  "raw_message": "ค่า netflix 400",
  "type": "expense",
  "amount": 400,
  "category": "Bill",
  "sub_category": "Subscription",
  "brand": "Netflix",
  "behavior_tag": "subscription",
  "description": "Subscription at Netflix"
}

Input: ค่าโรงแรม intercontinental 12000
Output:
{
  "raw_message": "ค่าโรงแรม intercontinental 12000",
  "type": "expense",
  "amount": 12000,
  "category": "Transport",
  "sub_category": "Travel",
  "brand": "Intercontinental",
  "behavior_tag": "travel",
  "description": "Travel at Intercontinental"
}

Input: ค่าเข้า disney land 2500
Output:
{
  "raw_message": "ค่าเข้า disney land 2500",
  "type": "expense",
  "amount": 2500,
  "category": "Other",
  "sub_category": "Entertainment",
  "brand": "Disney Land",
  "behavior_tag": "entertainment",
  "description": "Entertainment at Disney Land"
}

----------------------------------------
NOW PARSE:

Input: {{USER_INPUT}}
`)

	if len(corrections) > 0 {
		sb.WriteString(`----------------------------------------
USER LEARNING RULE (HIGH PRIORITY):

The following are user-defined classification rules.

You MUST follow them when relevant,
but still consider the full context.

Do NOT blindly apply them if the input clearly belongs to a different concept.

----------------------------------------`)
		for _, c := range corrections {
			sb.WriteString(fmt.Sprintf(
				`- keyword "%s" should be classified as category: %s, sub_category: %s, behavior_tag: %s`+"\n",
				extractKeyword(c.RawMessage),
				c.Category,
				c.SubCategory,
				c.BehaviorTag,
			))
		}
		sb.WriteString("\n")
	}
	sb.WriteString(fmt.Sprintf("Message: \"%s\"\n\nJSON:", message))
	return sb.String()
}

func extractKeyword(input string) string {
	words := strings.Fields(strings.ToLower(input))

	var result []string

	for _, w := range words {
		if _, err := strconv.ParseFloat(w, 64); err == nil {
			continue
		}
		if w == "ค่า" || w == "กิน" || w == "ซื้อ" {
			continue
		}
		result = append(result, w)
	}

	if len(result) == 0 {
		return input
	}

	return strings.Join(result, " ")
}

// Parse extracts a ParsedTransaction from the raw LLM response text.
func Parse(raw string) (*ParsedTransaction, error) {
	start, end := -1, -1
	for i, c := range raw {
		if c == '{' && start == -1 {
			start = i
		}
		if c == '}' {
			end = i
		}
	}
	if start == -1 || end == -1 {
		return nil, fmt.Errorf("no JSON found in LLM response: %s", raw)
	}

	var result ParsedTransaction
	if err := json.Unmarshal([]byte(raw[start:end+1]), &result); err != nil {
		return nil, fmt.Errorf("invalid JSON from LLM: %w", err)
	}
	return &result, nil
}
