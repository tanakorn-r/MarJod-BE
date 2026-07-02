# Context Pack

## Task

Move Rewards quest presets into DB-backed rows and add API controller support to create, edit, delete/list presets for daily and weekly quests.

## Relevant memory files read

- `.codex/memory/repo-map.md`
- `.codex/memory/backend-architecture.md`
- `.codex/memory/api-contracts.md`
- `.codex/memory/data-model.md`

## Relevant source files

- `model/quest.go`
- `repository/quest_repository.go`
- `service/quest_service.go`
- `controller/quest_controller.go`
- `router/router.go`
- `database/database.go`

## Current behavior

Quest templates are hardcoded in `service/quest_service.go`. User-specific assignments are persisted, but preset definitions cannot be created, edited, or deleted through the API.

## Desired behavior

Persist preset quests in DB with editable name/logo/xp plus enough rule metadata to evaluate daily/weekly task progress. Add API endpoints for preset CRUD and make reward assignment use active DB presets instead of only static service slices.

## Constraints

- Do not mock quests per user.
- Preserve `user_quest_assignments` history and XP behavior.
- Use smallest safe diff and existing controller/service/repository patterns.
- Deleting a preset should not destroy user assignment history.

## Smallest patch plan

1. Add `QuestPreset` model and migrate/seed defaults.
2. Extend quest repository with preset CRUD and active/all listing.
3. Convert DB presets into existing `QuestTemplate` evaluator functions.
4. Add controller handlers and `/api/quest-presets` routes.
5. Update memory docs and run narrow Go checks.

## Check command

`env GOCACHE=/tmp/marjod-go-cache go test ./controller ./router ./repository && env GOCACHE=/tmp/marjod-go-cache go build ./...`
