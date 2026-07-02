# Context Pack

## Task

Change Rewards quests from auto-initializing on `GET /api/quests` to explicit user activation/generation for daily or weekly quests.

## Relevant memory files read

- `AGENTS.md`
- `.codex/memory/api-contracts.md`
- `.codex/memory/data-model.md`
- `.codex/memory/common-failures.md`

## Relevant source files

- `model/quest.go`
- `repository/quest_repository.go`
- `service/quest_service.go`
- `controller/quest_controller.go`
- `router/router.go`
- `repository/quest_repository_test.go`

## Current behavior

`GET /api/quests` auto-creates daily and weekly quest batches when no active batch exists.

## Desired behavior

`GET /api/quests` only reads active assignments and returns empty daily/weekly lists when the user has not activated quests. A new explicit endpoint generates/activates random daily or weekly quests. Daily expires at next local midnight; weekly expires after Sunday at next Monday 00:00.

## Constraints

- Do not create mock per-user quest data.
- Keep user-specific state in `user_quest_assignments`.
- Preserve old assignment history.
- Smallest API/service diff.

## Smallest patch plan

1. Add service method to generate quests by period.
2. Stop `viewsForPeriod` from creating batches on read.
3. Add controller route `POST /api/quests/generate`.
4. Keep `POST /api/quests/reroll` compatibility for weekly completed batches.
5. Update memory and Swagger.

## Check command

`env GOCACHE=/tmp/marjod-go-cache go test ./controller ./router ./repository && env GOCACHE=/tmp/marjod-go-cache go build ./...`
