# Context Pack

## Task

Add a backend API for adjusting a wallet/monthly target.

## Relevant memory files read

- `.codex/memory/repo-map.md`
- `.codex/memory/backend-architecture.md`
- `.codex/memory/api-contracts.md`
- `.codex/memory/data-model.md`
- `.codex/memory/common-failures.md`

## Relevant source files

- `controller/wallet_controller.go`
- `service/wallet_service.go`
- `repository/wallet_repository.go`
- `model/wallet.go`
- `router/router.go`

## Current behavior

Wallets can be listed, created, selected as current, and archived. There is no API to update a wallet's target, so the frontend "Adjust monthly target" modal has no backend contract to save to.

## Desired behavior

Expose a user-scoped `PATCH /api/wallets/:id` endpoint that updates wallet target and optionally name/icon. For `id=0`, update the user's General wallet by ensuring it exists first.

## Constraints

- Preserve user isolation via `requestUserID`.
- Do not change existing wallet response fields.
- Do not allow archived wallets to be updated.
- Keep the diff narrow.

## Smallest patch plan

Add repository update method, service validation, controller request/handler, route registration, and API memory docs.

## Check command

`GOCACHE=/tmp/marjod-go-cache go build ./...`
