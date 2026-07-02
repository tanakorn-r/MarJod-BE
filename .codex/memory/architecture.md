# Architecture

This is a Go/Gin backend with layered architecture:

```text
HTTP / LINE
  -> controller
  -> service
  -> agent pipeline / business logic
  -> repository
  -> GORM
  -> SQLite or Turso/libsql
```

There is no frontend package in this repository. API contracts and design guidance exist here to support a separate frontend.

## Runtime flow

- `main.go` loads `config.Get()`, opens `database.Get()`, runs `database.Migrate(db)`, then starts `router.Setup(db, cfg)`.
- `router.Setup` constructs repositories, services, OpenAI clients, LINE service, controllers, middleware, CORS, Swagger, webhook, and `/api` routes.
- `/webhook` is public to LINE and must validate/store `source.userId`.
- `/api` routes use `middleware.RequireLineAuth`, which verifies a LIFF ID token and stores the user ID in Gin context.

## Backend boundaries

- Controllers should parse HTTP inputs, call services, and return JSON. Keep business logic out of controllers.
- Services own business rules, aggregation, AI prompt composition for analytics insight, and orchestration.
- Repositories own database queries and must include `user_id` filters for user-owned data.
- Models define persisted shape and JSON field names.
- Agents own pipeline steps for chat parsing and behavior features.
- Prompts own LLM schema and parse/validation rules.

## AI flow

- Chat parsing uses `service.NewOpenAIClient(cfg)` with `OPENAI_API_KEY`.
- Dashboard AI insight uses `service.NewOpenAIInternalClient(cfg)` with `OPENAI_API_KEY_INTERNAL`.
- `GET /api/analytics` must stay deterministic and token-free.
- `POST /api/analytics/insight` may call AI, but should send only compact aggregate data, not raw unnecessary transaction text.

## Data flow for a chat transaction

1. Controller/webhook receives message and user ID.
2. `TransactionService.Chat(userID, message)` loads the user's plan.
3. `agent.BuildPipeline(plan, deps)` picks agents.
4. `auditorAgent` builds prompt, calls LLM, parses JSON, gets current wallet, saves transaction.
5. Higher plans may compute behavior DNA, alerts, and recommendations.
6. API returns `agent.PipelineResult`.

## Conventions

- Normalize user IDs with `model.UserIDOrDefault`.
- Use `time.Parse("2006-01", raw)` for month params.
- JSON errors generally use `controller.ErrorResponse{Error: ...}`.
- Keep generated Swagger in `docs/` in sync with route annotations when API changes.
- Prefer focused tests in `service`, `prompt`, or `database` over broad manual checks.

## Important dependencies

- Gin for HTTP routing.
- GORM for persistence.
- SQLite locally; Turso/libsql remotely when `TURSO_URL` is set.
- OpenAI Go SDK for LLM calls.
- LINE Messaging/LIFF endpoints for auth, webhook signatures, and replies.
- Swaggo for Swagger generation.
