# Task Playbooks

## Fix UI issue

Read first:

- `.codex/memory/frontend-ui.md`
- `.codex/memory/design-system.md`
- `.codex/memory/api-contracts.md`

Inspect:

- If UI repo is not present here, ask for/open the frontend repo.
- For backend-driven UI bugs, inspect `controller/transaction_controller.go` and `service/transaction_service.go`.

Plan:

1. Identify the exact state: loading, empty, success, error, hover, focus, disabled, responsive.
2. Confirm whether the backend contract already provides the needed field.
3. Make the smallest UI/API fix.
4. Run the narrowest UI check available in the frontend repo, or backend tests if contract changed.

## Add frontend feature

Read first:

- `.codex/memory/frontend-ui.md`
- `.codex/memory/design-system.md`
- `.codex/memory/api-contracts.md`

Inspect:

- Frontend routing/components/state files in the frontend repo.
- Backend contract source if new data is required: `controller/`, `service/`, `model/`.

Plan:

1. Map UI states before coding.
2. Reuse existing components and tokens.
3. Keep AI insight explicit; do not auto-trigger token-spending calls.
4. Add contract handling for auth, empty data, and API errors.

## Add backend endpoint

Read first:

- `.codex/memory/backend-architecture.md`
- `.codex/memory/api-contracts.md`
- `.codex/memory/data-model.md`

Inspect:

- `router/router.go`
- Matching `controller/*_controller.go`
- Matching `service/*_service.go`
- Matching `repository/*_repository.go`
- `model/` structs

Plan:

1. Define request/response contract.
2. Add service interface method and implementation.
3. Add repository method only if a new query is needed.
4. Add controller handler with validation and stable errors.
5. Register route in `router/router.go`.
6. Update Swagger annotations/docs if public.
7. Add/adjust focused tests.

## Fix bug

Read first:

- `.codex/memory/common-failures.md`
- `.codex/memory/task-playbooks.md`
- The relevant architecture file.

Inspect:

- Reproduction path from controller to service to repository.
- Existing tests near the failing behavior.
- Recent diffs if the worktree is dirty.

Plan:

1. Reproduce with the narrowest command or code inspection.
2. Identify layer that owns the bug.
3. Patch minimally.
4. Add a regression test when practical.
5. Run focused test, then broader check if risk is high.

## Refactor safely

Read first:

- `.codex/memory/architecture.md`
- `.codex/memory/common-failures.md`
- Relevant layer memory.

Inspect:

- Callers with `rg`.
- Tests covering the touched API.

Plan:

1. Create a context pack.
2. Preserve public JSON/API behavior unless explicitly changing it.
3. Move code in small steps.
4. Run tests after each meaningful step.
5. Avoid opportunistic unrelated cleanup.

## Add tests

Read first:

- `.codex/memory/backend-architecture.md`
- `.codex/memory/data-model.md`

Inspect:

- `service/transaction_service_test.go`
- `database/database_test.go`
- `prompt/prompt_test.go`
- Existing mocks in test files.

Plan:

1. Prefer focused unit tests for services/prompt/database migration.
2. Mock LLM clients; do not call real OpenAI in tests.
3. Use temp DBs for migration tests.
4. Include user-isolation cases for user-owned data.

## Debug API/data issue

Read first:

- `.codex/memory/api-contracts.md`
- `.codex/memory/backend-architecture.md`
- `.codex/memory/data-model.md`
- `.codex/memory/common-failures.md`

Inspect:

- `middleware/line_auth.go`
- Controller for the endpoint.
- Service method.
- Repository query and model tags.
- Migration/backfill code if fields are missing.

Plan:

1. Confirm auth/user ID source.
2. Confirm query params and path values.
3. Trace service and repository filters.
4. Check whether old rows are under `default`.
5. Run a focused test or safe read-only DB/query check.
