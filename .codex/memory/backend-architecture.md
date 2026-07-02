# Backend Architecture

## Layers

- `router/` wires dependencies and routes.
- `middleware/` authenticates `/api` requests.
- `controller/` adapts HTTP/webhook requests to service calls.
- `service/` implements business logic and aggregation.
- `agent/` implements plan-dependent AI transaction pipeline.
- `prompt/` defines transaction-classification prompt schema and parser.
- `repository/` performs GORM queries.
- `model/` defines persisted schemas and JSON field names.
- `database/` opens DB and migrates schemas.

## Controller pattern

Controllers should:

- Parse path/query/body values.
- Use `requestUserID(ctx)` for `/api` user ownership.
- Return `400` for malformed input, `401` from middleware auth failures, `404` for missing detail reads, `500` for unexpected service errors.
- Return JSON with `ErrorResponse{Error: ...}` for errors.
- Avoid direct database access.

## Service pattern

Services should:

- Normalize `userID` with `model.UserIDOrDefault`.
- Keep business rules here, not in repositories/controllers.
- Accept interfaces for dependencies where tests need mocks.
- Keep AI calls explicit and bounded with token limits.
- Preserve deterministic analytics even when AI is unavailable.
- Use `timeutil` for app calendar logic. Product days, weeks, months, transaction timestamps, analytics grouping, Spending DNA windows, and Rewards timers are Thailand time (`Asia/Bangkok`, GMT+7), not server-local or UTC calendar dates.
- Trigger Rewards quest progress after successful chat/LINE transaction creation through the transaction service's optional quest evaluator hook. This is intentionally fire-and-forget so LINE/webhook and `/api/chat` responses do not wait on quest completion or XP awarding.

Important service interface: `service.TransactionService` in `service/transaction_service.go`.

## Repository pattern

Repositories should:

- Filter user-owned data with `WHERE user_id = ?`.
- Normalize user IDs before queries.
- Keep query details out of services unless the operation is pure in-memory aggregation.
- Avoid destructive migrations or delete/recreate patterns.

## Important endpoints

- `POST /webhook` — LINE webhook, no `/api` auth middleware.
- `POST /api/chat` — blocking natural-language transaction parse.
- `POST /api/chat/stream` — SSE shape exists, but currently wraps full pipeline rather than true token streaming.
- `GET /api/transactions` — user-scoped list.
- `GET /api/transactions/by-category` — category/month pagination.
- `GET /api/transactions/:id` — user-scoped detail.
- `DELETE /api/transactions/:id` — user-scoped delete.
- `PATCH /api/transactions/:id/correct` — update classification and persist correction example.
- `GET /api/summary` — income/expense/balance.
- `GET /api/analytics` — calculated dashboard; no AI spend.
- `GET /api/analytics/trend` — lightweight monthly expense trend.
- `GET /api/analytics/dna` — behavioral spending DNA.
- `POST /api/analytics/insight` — explicit AI insight.
- Wallet endpoints include list/create/update/current-wallet routes; `PATCH /api/wallets/:id` updates target/name/icon and treats `id=0` as the General-wallet compatibility path.
- Quest endpoints are registered in `router/router.go`; reward board endpoints use `user_quest_assignments` for user state and `quest_presets` for editable daily/weekly preset definitions. `GET /api/quests` is read-only; `POST /api/quests/generate` explicitly activates a random daily or weekly batch.
- Quest preset CRUD lives under `/api/quest-presets`; delete archives presets with `is_active=false` so assigned quest history remains readable.

## Validation and errors

- Request body validation uses Gin binding where tags exist.
- Month format is `YYYY-MM`, parsed with `time.Parse("2006-01", raw)`.
- Prompt validation requires all transaction fields, including `logic_gate` and `confidence`.
- `behavior_tag` must be one of `impulse`, `necessity`, `social`, `treat`, `recurring`.

## Migration style

- `database.Migrate` uses GORM `AutoMigrate`.
- Legacy rows are backfilled to `model.DefaultUserID` rather than deleted.
- Default quest presets are seeded during `database.Migrate` with conflict-do-nothing semantics so deploy/migration creates missing preset rows without overwriting admin edits.
- Add migration tests when changing required fields or ownership fields.
