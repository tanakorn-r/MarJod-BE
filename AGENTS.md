# Default Codex Workflow

For every task, first read this `AGENTS.md`.

Do not scan the whole repository by default. Use the repo memory system first.

Before editing:

1. Classify the task:
   - UI
   - backend
   - bug fix
   - refactor
   - test
   - architecture
2. Read only the relevant files from `.codex/memory/`.
3. Identify the minimum source files needed.
4. For medium or large changes, create a compact context pack in `.codex/context-packs/`.
5. Patch with the smallest safe diff.

## Token Discipline

- Prefer targeted reads over full-repo exploration.
- Do not open generated folders.
- Do not read unrelated files unless necessary.
- Do not rewrite code that is not part of the task.
- Stop after the first correct working fix.

## UI Tasks

For UI tasks, read:

- `.codex/memory/repo-map.md`
- `.codex/memory/frontend-ui.md`
- `.codex/memory/design-system.md`
- `.codex/memory/common-failures.md`

UI quality bar:

- Premium fintech SaaS quality.
- Strong hierarchy.
- Professional spacing.
- Clean cards/modals.
- No default tutorial-looking components.
- No random colors.
- Mobile responsive.
- Reuse existing components and design tokens.

## Backend Tasks

For backend tasks, read:

- `.codex/memory/repo-map.md`
- `.codex/memory/backend-architecture.md`
- `.codex/memory/api-contracts.md`
- `.codex/memory/data-model.md`

Backend rules:

- Preserve existing service/repository/controller patterns.
- Do not change API contracts casually.
- Do not bypass validation.
- Do not hardcode data.

## Bug Fixes

For bug fixes, read:

- `.codex/memory/repo-map.md`
- `.codex/memory/common-failures.md`
- Any memory file related to the broken area.

Bug-fix workflow:

1. Reproduce or locate the bug.
2. Explain root cause briefly.
3. Patch smallest diff.
4. Run the narrowest relevant check.

## Memory Maintenance

When a task changes repo structure, architecture, API contracts, data models, design system, domain logic, or common workflows, update the relevant `.codex/memory/*.md` file.

Do not update memory for tiny implementation-only changes.

Update memory when:

- A new page, route, component group, service, repository, endpoint, model, schema, store, or major utility is added.
- An API request/response shape changes.
- A data model field is added, removed, or meaningfully changed.
- A new design pattern or reusable UI component is introduced.
- A repeated bug or failure pattern is discovered.
- A task creates a new convention future agents should follow.

Before finishing, check whether memory should be updated.

Final response must include:

- Memory updated: yes/no
- If yes, list memory files changed.
- If no, explain briefly why not.

## Compact Context Pack

For medium/large tasks, create a compact context pack before editing:

```md
# Context Pack

## Task

...

## Relevant memory files read

...

## Relevant source files

...

## Current behavior

...

## Desired behavior

...

## Constraints

...

## Smallest patch plan

...

## Check command

...
```
