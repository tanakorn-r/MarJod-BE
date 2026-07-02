# Frontend UI

No frontend source exists in this repository at inspection time: no `package.json`, React/Vue/Next/Vite app, Tailwind config, or `.tsx/.jsx` components were found. Treat this repo as the backend/API contract source for a separate UI.

## Expected UI areas from backend contracts

- Chat entry / transaction capture
  - Calls `POST /api/chat` or `POST /api/chat/stream`.
  - Shows parsed transaction, behavior DNA, alerts, recommendations.

- Transaction list/detail
  - Calls `GET /api/transactions`, `GET /api/transactions/:id`, `DELETE /api/transactions/:id`.
  - Correction form calls `PATCH /api/transactions/:id/correct`.

- Analytics dashboard
  - Calls `GET /api/analytics?month=YYYY-MM&wallet_id=...` for normal page load.
  - Calls `GET /api/analytics/trend?month=YYYY-MM&months=6` for lightweight trend charts.
  - Calls `GET /api/analytics/dna` for spending personality/profile cards.
  - Calls `POST /api/analytics/insight?month=YYYY-MM&wallet_id=...` only when the user asks for AI analysis.

- Wallets
  - Calls wallet endpoints for listing, creating, and selecting current wallet.
  - Analytics accepts `wallet_id` to scope the dashboard.

- Quests
  - Calls quest endpoints for current quest batch and reroll.

## State management guidance for the frontend repo

- Store auth state from LIFF and send `Authorization: Bearer <id_token>` to every `/api` request.
- Do not let the user choose arbitrary `user_id`; the backend derives it from LINE auth.
- Cache normal analytics separately from AI insight. AI insight should be lazy, refreshable, and visibly token-spending/explicit.
- Model loading, empty, error, and retry states for every dashboard panel.
- For analytics, preserve partial rendering: if AI insight is unavailable, deterministic dashboard sections should still render.

## Routing/pages to create in a frontend repo

- `/dashboard` or `/analytics` — monthly finance dashboard and explicit AI insight action.
- `/transactions` — list, filters, category drill-down, correction entry points.
- `/wallets` — wallet budgets and current-wallet selection.
- `/quests` — gamified goals and progress.
- `/settings` — auth/account/debug/runtime configuration if needed.

## Reusable components to prefer

- Metric card for income, expense, balance, wallet target.
- Category breakdown list/chart.
- Daily spending chart.
- Behavior-tag breakdown.
- Irregular purchase list.
- AI insight panel with explicit “Generate insight” button.
- Empty-state card for new users or insufficient data.
- Inline error banner with retry action.

## Where to edit for common UI-facing backend tasks

- Response shape changes: `service/transaction_service.go` structs and `controller/transaction_controller.go`.
- New route: `router/router.go`, matching controller, service interface/implementation, Swagger docs.
- Auth/user behavior: `middleware/line_auth.go` and controller `requestUserID`.
- Wallet filtering: analytics controller and `filterByWallet`/wallet repository behavior.
