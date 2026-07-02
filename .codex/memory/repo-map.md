# Repo Map

This repository is a Go backend for a LINE-connected personal finance tracker. No React/Next/Vite/Tailwind frontend source exists in this workspace at inspection time.

## Top-level files

- `main.go` — application entry point. Loads config, opens DB, runs migrations, starts Gin server.
- `go.mod`, `go.sum` — Go module dependencies. Main stack: Gin, GORM, SQLite/libsql/Turso, OpenAI Go SDK, Swagger.
- `Dockerfile`, `docker-compose.yml` — container runtime entry points.
- `README.md` — older quick-start docs. Treat with caution: it still mentions Ollama, while current code uses OpenAI.
- `API_ANALYTICS.md`, `CORRECTIONS_API.md`, `IMPLEMENTATION_SUMMARY.md` — feature docs; useful context but verify against source.
- `docs/` — generated Swagger artifacts. Regenerate after changing annotated routes.
- `scripts/` — deploy and utility scripts, including `scripts/deploy.sh`, `scripts/analyze_daily.sh`, and git hook helper.

## Source folders

- `config/` — environment loading and runtime config. Supports `ENV_FILE=.env.production`, SQLite/Turso config, OpenAI keys, LINE config, CORS, and local auth bypass.
- `database/` — GORM setup and migrations. `Migrate` performs AutoMigrate and legacy `user_id` backfills.
- `model/` — persisted schemas and DTO-like domain structs: transactions, corrections, behavior profiles, plans, wallets, quests, alerts.
- `repository/` — DB access layer. Keep user filtering here for data isolation.
- `service/` — business logic: transaction pipeline orchestration, analytics aggregation, AI insight, wallets, quests, spending DNA, OpenAI and LINE clients.
- `agent/` — plan-based AI agent pipeline: auditor, profiler, nagger, strategist.
- `prompt/` — transaction-classification prompt, parser, validation, and normalization.
- `controller/` — Gin HTTP handlers for API, LINE webhook, wallets, quests.
- `middleware/` — request middleware, especially LINE LIFF ID token auth for `/api`.
- `router/` — dependency wiring, CORS, Swagger, webhook, and `/api` route registration.

## Tests

- `service/transaction_service_test.go` — service/analytics/AI behavior tests with mocks.
- `database/database_test.go` — migration safety/backfill tests.
- `prompt/prompt_test.go` — LLM JSON parsing and validation tests.

## Start here by task

For API route changes:

- `router/router.go`
- `controller/*_controller.go`
- `service/transaction_service.go` or the matching service
- `docs/` Swagger output after regeneration

For transaction parsing:

- `prompt/transaction_prompt.go`
- `agent/auditor.go`
- `model/transaction.go`
- `service/transaction_service_test.go`

For per-user data isolation:

- `middleware/line_auth.go`
- `controller/transaction_controller.go`
- `controller/webhook_controller.go`
- `repository/*_repository.go`
- `model/transaction.go`, `model/user_correction.go`, `model/behavior_profile.go`, `model/user_plan.go`

For analytics/dashboard data:

- `service/transaction_service.go`
- `service/spending_dna_service.go`
- `controller/transaction_controller.go`
- `model/transaction.go`
- `API_ANALYTICS.md`

For wallets:

- `model/wallet.go`
- `model/user_wallet_selection.go`
- `repository/wallet_repository.go`
- `service/wallet_service.go`
- `controller/wallet_controller.go`

For quests/gamification:

- `model/quest.go`
- `repository/quest_repository.go`
- `service/quest_service.go`
- `controller/quest_controller.go`

For config/runtime:

- `config/config.go`
- `database/database.go`
- `router/router.go`
- `Dockerfile`, `docker-compose.yml`, `scripts/deploy.sh`
