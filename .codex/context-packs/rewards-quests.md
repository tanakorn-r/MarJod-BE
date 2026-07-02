# Context Pack

## Task

Support Rewards by expanding preset expense quests and assigning random quests automatically the first time a user opens the quest page.

## Relevant memory files read

- `AGENTS.md`
- `.codex/memory/repo-map.md`
- `.codex/memory/product.md`
- `.codex/memory/api-contracts.md`

## Relevant source files

- `service/quest_service.go`
- `model/quest.go`
- `repository/quest_repository.go`
- `controller/quest_controller.go`

## Current behavior

`GET /api/quests` evaluates the latest quest batch. If the user has never had a batch, it returns an empty quest list. Quest difficulties are only easy/medium/hard, and the pool has one broad food cap quest labeled coffee.

## Desired behavior

First open should assign a random quest batch. Quest presets should include coffee spending caps at Basic, Advanced, Expert, Master, and Grand Master levels: under 1000/500/300/200/100 THB this week.

## Constraints

- Preserve existing quest assignment tables; no migration needed.
- Progress remains computed live from real transactions.
- No OpenAI/token call for normal quest reads.
- Keep response shape compatible: `difficulty` remains a string.

## Smallest patch plan

Add new difficulty constants, add coffee-specific cap evaluator/templates, auto-create a random batch when no latest batch exists, update memory/docs, run build/focused checks.

## Check command

`GOCACHE=/tmp/marjod-go-cache go build ./...`
