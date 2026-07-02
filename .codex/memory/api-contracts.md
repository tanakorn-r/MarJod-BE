# API Contracts

All `/api` routes require `Authorization: Bearer <LIFF ID token>` unless `AUTH_DEV_BYPASS=true` in local development. The backend verifies the token with LINE and derives `userID`; production clients should not send or trust arbitrary user IDs.

In local development only, `AUTH_DEV_BYPASS=true` skips LINE token verification. If a request includes `?user_id=<mock LINE user id>`, middleware uses that mock user ID; otherwise it falls back to `AUTH_DEV_USER_ID`. This mirrors the MarJod frontend's `VITE_AUTH_DEV_BYPASS=true` mode.

## Common error shape

```json
{ "error": "message" }
```

## Chat

`POST /api/chat`

Request:

```json
{ "message": "กาแฟ Starbucks 180" }
```

Response is `agent.PipelineResult`:

```json
{
  "transaction": {
    "id": 1,
    "user_id": "U...",
    "raw_message": "กาแฟ Starbucks 180",
    "type": "expense",
    "amount": 180,
    "category": "Food & Beverage",
    "sub_category": "Coffee",
    "brand": "Starbucks",
    "description": "Coffee at Starbucks 180 THB",
    "behavior_tag": "treat",
    "created_at": "2026-06-26T..."
  },
  "behavior_dna": null,
  "alerts": [],
  "recommendations": []
}
```

Do not casually rename transaction JSON fields; frontend charts and correction UI depend on them.

After a successful saved transaction, the backend triggers active daily/weekly quest evaluation asynchronously. This side effect must not delay the chat or LINE webhook response, and it must not create quest batches automatically.

## Transactions

- `GET /api/transactions?wallet_id=0` returns `[]model.Transaction`, newest first, optionally scoped to a wallet. Omit `wallet_id` for all wallets. `wallet_id=0` resolves to the persisted General wallet before filtering.
- `GET /api/transactions/:id` returns one user-owned transaction.
- `DELETE /api/transactions/:id` returns `{ "message": "deleted" }`.
- `GET /api/transactions/by-category?category=Food%20%26%20Beverage&month=2026-06&page=1&limit=20` returns:

```json
{
  "category": "Food & Beverage",
  "month": "2026-06",
  "page": 1,
  "limit": 20,
  "total": 42,
  "pages": 3,
  "items": []
}
```

## Corrections

`PATCH /api/transactions/:id/correct`

Request fields are optional, and only non-empty fields are applied:

```json
{
  "category": "Food & Beverage",
  "sub_category": "Coffee",
  "brand": "Starbucks",
  "behavior_tag": "treat"
}
```

Response:

```json
{ "message": "correction saved" }
```

`GET /api/corrections` returns correction examples. `DELETE /api/corrections/:id` removes one.

## Summary

`GET /api/summary`

```json
{
  "total_income": 50000,
  "total_expense": 1200,
  "balance": 48800
}
```

## Analytics dashboard

`GET /api/analytics?month=2026-06&wallet_id=0`

This endpoint must not call AI. Response shape:

Calendar filtering uses Thailand time (`Asia/Bangkok`, GMT+7). For example, `month=2026-06` means `2026-06-01T00:00:00+07:00` through `2026-07-01T00:00:00+07:00`.

```json
{
  "monthly_summary": {
    "month": "June 2026",
    "total_income": 50000,
    "total_expense": 1200,
    "balance": 48800,
    "alert": null,
    "target": 50000
  },
  "recent_highlights": [],
  "daily_spending": [],
  "category_breakdown": [],
  "behavior_breakdown": [],
  "savings_comparison": { "label": "vs last month", "amount": 0, "percentage": 0 },
  "expense_comparison": { "label": "vs last month", "amount": 0, "percentage": 0 },
  "irregular_purchases": [],
  "spending_by_day_of_week": [],
  "behavior_insights": []
}
```

`wallet_id` is optional. `0` represents General in some wallet paths; verify wallet behavior before changing this contract.

## AI analytics insight

`POST /api/analytics/insight?month=2026-06&wallet_id=0`

This endpoint explicitly calls the internal OpenAI client. Response:

```json
{
  "status": "ready",
  "health": "healthy",
  "headline": "short headline",
  "summary": "2-3 concise sentences",
  "key_findings": ["finding"],
  "recommendations": [
    {
      "priority": "high",
      "title": "short title",
      "action": "specific action",
      "rationale": "why it helps"
    }
  ]
}
```

Possible `status`: `ready`, `insufficient_data`, `unavailable`. Possible `health`: `healthy`, `watch`, `critical`.

## Trend and DNA

- `GET /api/analytics/trend?month=2026-06&months=6` returns `[{ "month": "2026-01", "total_expense": 1234 }]`.
- `GET /api/analytics/dna?wallet_id=0` returns rule-derived spending DNA, optionally scoped to a wallet. Omit `wallet_id` for all-wallet DNA. If fewer than 5 matching transactions exist, `insufficient_data` is true. `wallet_id=0` resolves to the persisted General wallet before filtering.

## Wallets

- `GET /api/wallets` returns `[]model.Wallet`.
- `POST /api/wallets` creates a wallet with `{ "name": "Japan Trip", "icon": "🗺️", "target": 20000 }`.
- `PATCH /api/wallets/:id` updates a wallet and returns `model.Wallet`. Body fields are optional, but at least one must be present: `{ "name": "Monthly", "icon": "👝", "target": 50000 }`. Passing `id=0` updates the user's General wallet after ensuring it exists, and returns the persisted wallet row.
- `GET /api/wallets/current` returns `{ "wallet": model.Wallet }`. If no valid selection exists yet, the backend creates/finds the persisted General wallet and stores it in `user_wallet_selections`.
- `PATCH /api/wallets/current` sets the current wallet with `{ "wallet_id": 1 }`. Passing `wallet_id=0` resolves to the user's persisted General wallet and stores that persisted ID in `user_wallet_selections`.
- `POST /api/chat` and LINE webhook-created transactions use `user_wallet_selections` through the current-wallet resolver before saving the transaction's `wallet_id`.

## Quests / Rewards

- `GET /api/quests` returns the Rewards board for existing active daily and weekly quest batches. This endpoint must not auto-assign quests just because the page opened.
- A quest batch is active only when its assignments have valid `assigned_at` and future `expires_at` values. If the date window is missing or expired, `GET /api/quests` returns an empty list for that period.
- `POST /api/quests/generate` activates quests by randomly assigning an active preset batch for a requested period. Body: `{ "period": "daily" }` or `{ "period": "weekly" }`. Only one daily set is allowed per Thailand day and only one weekly set is allowed per Thailand week; if a set was already generated in the current period window, the endpoint returns the current board without creating duplicates.
- Daily quest batches expire at the next Thailand midnight (`Asia/Bangkok`, GMT+7). Weekly quest batches expire after Sunday at the next Monday 00:00 Thailand time. Expired batches are left in the DB for history; users activate a new batch with `POST /api/quests/generate`.
- `POST /api/quests/reroll` is disabled under the one-set-per-week rule and returns `400`; weekly activation should use `POST /api/quests/generate`. `can_reroll` should stay `false`.
- Quests are not mocked per user. Preset quest templates live in DB-backed `quest_presets`; user-specific assignment/completion/payout/expiry state comes from `user_quest_assignments`; total XP comes from `user_game_profiles`.
- `GET /api/quests` returns:

```json
{
  "quests": [
    {
      "key": "coffee_cap_basic_1000",
      "period": "weekly",
      "title": "Basic coffee quest: spend under ฿1000 this week",
      "logo": "☕",
      "difficulty": "basic",
      "accent": "#D9463B",
      "xp": 30,
      "current": 180,
      "target": 1000,
      "unit": "thb",
      "done": false,
      "assigned_at": "2026-06-27T...",
      "expires_at": "2026-06-29T00:00:00+07:00",
      "completed_at": null,
      "xp_awarded": false
    }
  ],
  "daily_quests": [
    {
      "key": "daily_log_transaction",
      "period": "daily",
      "title": "Log a transaction today",
      "logo": "✍️",
      "difficulty": "basic",
      "accent": "#3D9B6E",
      "xp": 10,
      "current": 1,
      "target": 1,
      "unit": "count",
      "done": true,
      "assigned_at": "2026-06-27T...",
      "expires_at": "2026-06-28T00:00:00+07:00",
      "completed_at": "2026-06-27T...",
      "xp_awarded": true
    }
  ],
  "weekly_quests": [
    {
      "key": "coffee_cap_basic_1000",
      "period": "weekly",
      "title": "Basic coffee quest: spend under ฿1000 this week",
      "logo": "☕",
      "difficulty": "basic",
      "accent": "#D9463B",
      "xp": 30,
      "current": 180,
      "target": 1000,
      "unit": "thb",
      "done": false,
      "assigned_at": "2026-06-27T...",
      "expires_at": "2026-06-29T00:00:00+07:00",
      "completed_at": null,
      "xp_awarded": false
    }
  ],
  "profile": {
    "level": 1,
    "level_name": "Newcomer",
    "next_level_name": "Saver",
    "total_xp": 0,
    "xp_into_level": 0,
    "xp_for_next_level": 150
  },
  "streak": { "current_days": 0, "best_days": 0 },
  "stats": {
    "current_batch_total": 3,
    "current_batch_completed": 0,
    "current_batch_remaining": 3,
    "current_batch_xp_available": 140,
    "current_batch_xp_earned": 0,
    "daily_total": 3,
    "daily_completed": 1,
    "weekly_total": 3,
    "weekly_completed": 0,
    "total_xp": 0
  },
  "levels": [
    {
      "level": 1,
      "name": "Newcomer",
      "xp_required": 0,
      "icon": "🌱",
      "feature": "Auto-categorized transactions",
      "cosmetic": "Clay theme",
      "state": "current",
      "unlocked": true
    }
  ],
  "rewards": [
    {
      "level": 1,
      "name": "Newcomer",
      "xp_required": 0,
      "icon": "🌱",
      "feature": "Auto-categorized transactions",
      "cosmetic": "Clay theme",
      "state": "current",
      "unlocked": true
    }
  ],
  "can_reroll": false,
  "can_generate_daily": false,
  "can_generate_weekly": false
}
```

- Quest `difficulty` is a string. Current values include `basic`, `advanced`, `expert`, `master`, `grand_master`; legacy values `easy`, `medium`, `hard` may still appear in old rows.
- Coffee expense cap quests use the ladder Basic/Advanced/Expert/Master/Grand Master for weekly coffee spend under 1000/500/300/200/100 THB.
- Level unlocks are returned in `levels` and `rewards`; each item has `state` of `locked`, `current`, or `unlocked`.
- Quest preset CRUD:
  - `GET /api/quest-presets?include_inactive=true` returns `[]model.QuestPreset`. Omit `include_inactive` for active-only rows.
  - `POST /api/quest-presets` creates a preset. Required fields: `key`, `name`, `period`, `difficulty`, `xp`, `rule_type`. Optional but recommended: `logo`, `accent`, `target`, `unit`, `category`, `sub_category`, `behavior_tag`, `is_active`.
  - `PATCH /api/quest-presets/:id` updates any editable preset fields.
  - `DELETE /api/quest-presets/:id` archives the preset by setting `is_active=false`; it does not hard-delete history.
  - Supported `rule_type` values: `log_transaction_count`, `spend_cap`, `no_impulse`, `no_spend_days`, `category_spend_cap`, `coffee_spend_cap`, `beat_last_month_savings_rate`.

## Contracts not to change casually

- LINE auth: `/api` user identity comes from LIFF token verification.
- Local mock auth: `?user_id=` is honored only when backend `AUTH_DEV_BYPASS=true`.
- `GET /api/analytics` must stay token-free.
- `POST /api/analytics/insight` is the explicit AI-spend boundary.
- Transaction field names and behavior tag values.
- Month query format `YYYY-MM`.
- User-owned repository filters.
- Wallet `id=0` is the General-wallet compatibility path; update APIs may return the persisted General wallet ID.
