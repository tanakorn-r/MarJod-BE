# Product

Finance Chat is a personal finance tracker built around natural-language transaction entry. A user sends messages such as “กาแฟ Starbucks 180” or “เงินเดือน 35000”; the backend classifies the message into structured finance data, stores it, and powers summary, analytics, wallet, quest, and AI-insight experiences.

## Core user flows

1. LINE chat transaction capture
   - LINE sends a webhook event to `POST /webhook`.
   - The webhook reads `event.source.userId`, passes the text through the transaction pipeline, stores the transaction, and replies with a formatted summary.

2. LIFF/API dashboard access
   - Frontend calls `/api/*` with `Authorization: Bearer <LIFF ID token>`.
   - `middleware.RequireLineAuth` verifies the token with LINE and stores the LINE user ID in Gin context.
   - Controllers pass that user ID to services/repositories so users only see their own data.

3. Transaction correction
   - User edits category/subcategory/brand/behavior tag via `PATCH /api/transactions/:id/correct`.
   - Correction is saved in `user_corrections` and can be used as prompt context.

4. Analytics dashboard
   - `GET /api/analytics?month=YYYY-MM&wallet_id=...` returns calculated dashboard data without calling AI.
   - `POST /api/analytics/insight?month=YYYY-MM&wallet_id=...` explicitly spends AI tokens to interpret already-calculated aggregates.

5. Wallets and goals
   - Users can create wallet/occasion budgets, select a current wallet, and view wallet balances.
   - Transactions are assigned to the current wallet by the auditor agent.

6. Quests/gamification
   - On first quest-page open, the backend assigns a random batch of preset quests.
   - Quest progress is computed from real transaction history.
   - XP and levels help motivate consistent logging and better financial habits.

## Important domain concepts

- Transaction — one income or expense parsed from a user message.
- User ID — LINE user ID. This is the ownership boundary for transactions, corrections, profiles, plans, wallets, and quests.
- Behavior tag — AI-classified spending intent: `impulse`, `necessity`, `social`, `treat`, or `recurring`.
- Behavior DNA/profile — derived behavioral metrics such as dominant category, impulse frequency, luxury drift, and top brands.
- Dashboard analytics — deterministic aggregates for a month: summary, daily spending, category breakdown, comparisons, irregular purchases, behavior insights.
- AI personal finance insight — LLM-generated interpretation of dashboard aggregates. It must not run on normal analytics page load.
- Wallet — user budget container. `GeneralWalletID` is a special virtual/default concept; custom wallets are persisted rows.
- Quest — a preset challenge assigned randomly in batches, evaluated from real transactions. Expense quests include coffee-spending cap ladders from Basic to Grand Master.
- Plan — `free`, `starter`, or `pro`; controls which agents run in the pipeline.

## Product tone

The product should feel like a personal finance companion, not a bank statement. Insights should help users understand “what is going on in my life” without shame, fake certainty, or invented personal context.
