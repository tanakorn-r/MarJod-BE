# Context Pack

## Task

After every chat/LINE-created transaction is saved, evaluate active daily and weekly quests without blocking the user response.

## Relevant memory files read

- `AGENTS.md`
- `.codex/memory/repo-map.md`
- `.codex/memory/backend-architecture.md`
- `.codex/memory/api-contracts.md`
- `.codex/memory/data-model.md`
- `.codex/memory/common-failures.md`

## Relevant source files

- `service/transaction_service.go`
- `service/quest_service.go`
- `router/router.go`

## Current behavior

Quest completion and XP are evaluated when the Rewards board is requested. A newly saved transaction does not trigger quest evaluation immediately.

## Desired behavior

When the chat/LINE pipeline successfully saves a transaction, trigger active quest evaluation in a goroutine. The LINE/API response must not wait for quest progress calculation or XP awarding.

## Constraints

- Keep `GET /api/quests` read-only.
- Do not auto-generate quests.
- Evaluate only existing active daily/weekly assignments.
- Avoid making transaction creation fail if async quest evaluation fails.
- Preserve service/repository boundaries.

## Smallest patch plan

1. Add a quest-progress evaluator interface and optional setter to transaction service.
2. Call the evaluator asynchronously after successful chat pipeline completion.
3. Add an explicit `EvaluateActiveQuests` method to quest service that reuses the existing quest board evaluation.
4. Wire quest service into transaction service in the router.
5. Update memory with the new transaction-to-quest side effect.

## Check command

`go test ./controller ./router ./repository ./database ./timeutil && go build ./...`
