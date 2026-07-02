# Context Pack

## Task

Apply Thailand calendar time (`Asia/Bangkok`, GMT+7) to transaction timestamps and all day/week/month grouping, not only quests.

## Relevant memory files read

- `AGENTS.md`
- `.codex/memory/backend-architecture.md`
- `.codex/memory/data-model.md`
- `.codex/memory/common-failures.md`

## Relevant source files

- `repository/transaction_repository.go`
- `service/transaction_service.go`
- `service/spending_dna_service.go`
- `service/quest_service.go`
- `agent/profiler.go`
- `controller/transaction_controller.go`

## Current behavior

Some quest logic uses Thailand time, but transactions and analytics still rely on server-local or UTC boundaries in several places.

## Desired behavior

New transactions are stamped in Thailand time. Daily/monthly/category/trend/DNA/quest calendar calculations use Thailand time consistently.

## Constraints

- Preserve existing API shapes.
- Do not rewrite unrelated architecture.
- Keep DB data; no destructive migrations.
- Use narrow checks because existing `go test ./service` has stale tests.

## Smallest patch plan

1. Add shared `timeutil` package for Thailand location, now, and calendar boundaries.
2. Use it in transaction repository create/today/category filters.
3. Use it in transaction analytics, trend, formatting, controller month parse.
4. Use it in spending DNA and quest service.
5. Update memory and run relevant checks/build.

## Check command

`env GOCACHE=/tmp/marjod-go-cache go test ./controller ./router ./repository ./timeutil && env GOCACHE=/tmp/marjod-go-cache go build ./...`
