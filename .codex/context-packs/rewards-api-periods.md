# Context Pack

## Task

Support the Rewards page API with daily tasks that expire daily, weekly tasks that expire weekly, and a leveling/unlock ladder for rewards.

## Relevant memory files read

- `.codex/memory/repo-map.md`
- `.codex/memory/backend-architecture.md`
- `.codex/memory/api-contracts.md`
- `.codex/memory/data-model.md`
- `.codex/memory/common-failures.md`

## Relevant source files

- `model/quest.go`
- `repository/quest_repository.go`
- `service/quest_service.go`
- `controller/quest_controller.go`
- `database/database.go`
- `router/router.go`
- `/Users/dino/Downloads/MarJod Dashboard (standalone).html`

## Current behavior

`GET /api/quests` returns one generic latest quest batch. Assignments have no period or expiry metadata. Profile has simple level fields only.

## Desired behavior

`GET /api/quests` returns a Rewards board with daily quests, weekly quests, expiry timestamps, stats, profile XP/level, and level-gated rewards/unlocks. Daily assignment batches reset after the current day expires. Weekly assignment batches reset after the current week expires.

## Constraints

- Do not mock per-user quest rows.
- Preset quest templates are allowed; user-specific assignment/completion/payout state must be persisted.
- Preserve existing response fields where possible.
- Use AutoMigrate; do not delete existing data.
- Keep API user-scoped.

## Smallest patch plan

1. Add `period` and `expires_at` to `UserQuestAssignment`.
2. Add repository lookup for latest active batch by period.
3. Split templates into daily and weekly pools.
4. Add daily/weekly quest arrays and level reward ladder to `QuestBoard`.
5. Preserve legacy `quests` as weekly quests.
6. Add focused service tests around expiry rotation and level unlocks.
7. Update memory docs.

## Check command

```sh
env GOCACHE=/tmp/marjod-go-cache go test ./service ./repository
env GOCACHE=/tmp/marjod-go-cache go build ./...
```
