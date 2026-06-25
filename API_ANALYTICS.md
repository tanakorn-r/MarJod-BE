# Analytics API Documentation

This document describes the new analytics endpoints added to support the financial dashboard.

## New Endpoints

### 1. Get Transaction Details
**GET** `/api/transactions/:id`

Returns detailed information about a specific transaction.

**Response:**
```json
{
  "id": 1,
  "raw_message": "spent 250 baht on lunch",
  "type": "expense",
  "amount": 250,
  "category": "Food & Drink",
  "sub_category": "Lunch",
  "brand": "Restaurant Name",
  "description": "Lunch at restaurant",
  "behavior_tag": "necessity",
  "created_at": "2025-04-26T10:30:00Z"
}
```

### 2. Get Analytics Dashboard
**GET** `/api/analytics`

Returns comprehensive analytics data for the dashboard including:
- Monthly summary with alerts
- Recent highlighted transactions
- Daily spending chart data
- Category breakdown
- Savings/expense comparisons
- Irregular purchases
- Spending by day of week
- Rule-based behavioral insights

This endpoint does not call OpenAI and does not consume AI tokens.

**Response Structure:**
```json
{
  "monthly_summary": {
    "month": "April 2025",
    "total_income": 869500,
    "total_expense": 854320,
    "balance": 14100,
    "alert": {
      "type": "warning",
      "message": "You've spent 90% of your income this month..."
    }
  },
  "recent_highlights": [
    {
      "id": 1,
      "description": "Coffee Overload",
      "amount": 4200,
      "category": "Food & Drink",
      "highlight": "most_spent"
    }
  ],
  "daily_spending": [
    {
      "date": "2025-04-01",
      "income": 0,
      "expense": 1250
    }
  ],
  "category_breakdown": [
    {
      "rank": 1,
      "category": "Food & Drink",
      "sub_category": "",
      "amount": 14200,
      "percentage": 34.2
    }
  ],
  "savings_comparison": {
    "label": "vs last month",
    "amount": 32220,
    "percentage": 58.0
  },
  "expense_comparison": {
    "label": "vs last month",
    "amount": 22100,
    "percentage": -48.0
  },
  "irregular_purchases": [
    {
      "description": "Apple Watch - Blue Backlog",
      "brand": "Apple",
      "amount": 19500,
      "date": "Apr 15",
      "reason": "Higher than usual spending"
    }
  ],
  "spending_by_day_of_week": [
    {
      "day": "Mon",
      "amount": 1200
    }
  ],
  "behavior_insights": [
    {
      "icon": "☕",
      "title": "Daily Half-Habit",
      "description": "You're ordering coffee almost every day...",
      "metric": "5 times this week"
    }
  ]
}
```

### 3. Generate AI Finance Insight

**POST** `/api/analytics/insight?month=2026-06`

Explicitly calls OpenAI with the calculated dashboard data. Call this endpoint
only when the user asks to generate or refresh their AI insight.

```json
{
  "status": "ready",
  "health": "watch",
  "headline": "Spending is close to monthly income",
  "summary": "Expenses consumed most of this month's income. Food and discretionary purchases are the clearest opportunities to improve cash flow.",
  "key_findings": [
    "The savings margin is narrow",
    "Food & Drink is the largest expense category"
  ],
  "recommendations": [
    {
      "priority": "high",
      "title": "Set a weekly food limit",
      "action": "Cap Food & Drink spending for the rest of the month",
      "rationale": "It targets the largest controllable expense category"
    }
  ]
}
```

`status` is `insufficient_data` when the selected month has no transactions and
`unavailable` when OpenAI cannot be reached.

### 4. List All Corrections
**GET** `/api/corrections`

Returns all user corrections that have been made. These corrections are used to train the AI to better understand user preferences.

**Response:**
```json
[
  {
    "id": 1,
    "raw_message": "starbucks 180",
    "category": "Food & Drink",
    "sub_category": "Coffee",
    "brand": "Starbucks",
    "behavior_tag": "treat",
    "created_at": "2025-04-26T10:30:00Z"
  },
  {
    "id": 2,
    "raw_message": "grab taxi 120",
    "category": "Transport",
    "sub_category": "Taxi",
    "brand": "Grab",
    "behavior_tag": "necessity",
    "created_at": "2025-04-25T15:20:00Z"
  }
]
```

### 5. Delete a Correction
**DELETE** `/api/corrections/:id`

Removes a correction by its ID. This will stop the AI from learning from this example.

**Response:**
```json
{
  "message": "correction deleted"
}
```

## Implementation Notes

### Current Status
- ✅ Transaction list endpoint (existing)
- ✅ Transaction details endpoint (new)
- ✅ Analytics dashboard endpoint (new)
- ✅ List corrections endpoint (new)
- ✅ Delete correction endpoint (new)
- ✅ Real calculations for: monthly summary, category breakdown, daily spending
- 🔄 Mock data for: behavioral insights, irregular purchase detection, comparisons

### AI Integration

The dashboard aggregates are sent to OpenAI for a concise interpretation. Raw
transaction messages are intentionally excluded from the AI prompt. The
generated result explains the current financial picture, highlights important
patterns and returns prioritized actions.

The following areas can still be enhanced:

1. **Behavioral Insights** (`behavior_insights`)
   - AI should analyze spending patterns daily
   - Detect habits, trends, and anomalies
   - Generate personalized recommendations

2. **Irregular Purchases** (`irregular_purchases`)
   - AI should learn normal spending patterns
   - Flag unusual transactions with reasoning
   - Consider context (time, amount, category)

3. **Smart Alerts** (`monthly_summary.alert`)
   - AI should generate contextual warnings
   - Predict budget overruns
   - Suggest corrective actions

4. **Comparisons** (`savings_comparison`, `expense_comparison`)
   - Currently returns mock data
   - Should calculate actual month-over-month changes
   - Requires historical data tracking

### Suggested AI Implementation
Consider using a scheduled job (cron/bash) to:
1. Run daily analysis on transaction data
2. Generate insights using the LLM
3. Cache results for quick API responses
4. Update as new transactions are added

Example bash script:
```bash
#!/bin/bash
# Run daily at midnight
0 0 * * * /path/to/analyze_transactions.sh
```

## Testing

Test the endpoints:
```bash
# Get transaction details
curl http://localhost:8080/api/transactions/1

# Get analytics dashboard
curl http://localhost:8080/api/analytics

# List all corrections
curl http://localhost:8080/api/corrections

# Delete a correction
curl -X DELETE http://localhost:8080/api/corrections/1
```

## Swagger Documentation

All endpoints are documented in Swagger UI:
- Visit: `http://localhost:8080/swagger/index.html`
- Interactive API testing available
