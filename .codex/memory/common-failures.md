# Common Failures

## Repo-specific mistakes to avoid

- Trusting `README.md` as fully current. It still references Ollama; current code uses OpenAI.
- Forgetting user isolation. Repository reads/writes for user-owned data must include `user_id`.
- Letting clients pass arbitrary `user_id` for `/api`. User identity should come from LINE LIFF auth middleware.
- Forgetting that frontend local mock mode also needs backend `AUTH_DEV_BYPASS=true`; otherwise `Authorization: Bearer dev-bypass` is rejected as an invalid LINE token.
- Treating local `?user_id=` mock identity as production behavior. It is honored only while backend `AUTH_DEV_BYPASS=true`.
- Saving webhook transactions without LINE `event.source.userId`.
- Calling AI from `GET /api/analytics`. AI insight belongs behind `POST /api/analytics/insight`.
- Breaking legacy data migration. Required ownership fields must backfill old rows instead of dropping/recreating data.
- Changing transaction parser schema without updating prompt validation and tests.
- Confusing General wallet `0`, persisted wallet IDs, and `NULL` wallet IDs.
- Saving chat/webhook transactions without first resolving the user's persisted current wallet selection. The resolver should use an existing valid selection, otherwise create/find and select the persisted General wallet; do not choose a first active custom wallet as fallback.
- Reading analytics for `wallet_id=0` from the virtual General wallet instead of resolving the persisted General wallet; this can make `monthly_summary.target` ignore wallet target updates.
- Assuming `POST /api/chat/stream` is true token streaming. Current implementation runs the full pipeline and emits the final result.
- Updating annotated routes without regenerating/checking Swagger docs.
- Auto-assigning quests from `GET /api/quests`. Opening the Rewards page should be read-only; users activate daily/weekly batches with `POST /api/quests/generate`.
- Treating Rewards as one timeless quest list. Rewards uses daily and weekly active batches with `expires_at`; expired rows are historical and a new active batch should be created only by explicit generation.
- Allowing duplicate quest generation/reroll in the same period window. Users get only one daily set per Thailand day and one weekly set per Thailand week, even if the set is completed early; `/api/quests/reroll` should stay unavailable/400 under this rule.
- Treating quest assignments with missing `assigned_at` or missing/zero `expires_at` as active. Missing date windows should return empty quests for that period.
- Assuming quest difficulties are only `easy`, `medium`, and `hard`; Rewards now uses `basic`, `advanced`, `expert`, `master`, and `grand_master`.
- Mocking per-user quest data in responses. Rewards must return DB-backed `quest_presets` metadata combined with persisted `user_quest_assignments` completion/payout state and `user_game_profiles` XP.
- Hard-deleting quest presets that may be referenced by assigned quests. `DELETE /api/quest-presets/:id` should archive with `is_active=false`.
- Awarding quest XP just for opening the page with no real activity. Daily/weekly tasks should be evaluated from transactions and paid out idempotently.
- Forgetting the non-blocking quest evaluation hook after chat/LINE transaction creation. Saved transactions should trigger active daily/weekly quest checks asynchronously; the user response must not wait for Rewards bookkeeping.
- Returning a temporary `UserGameProfile` for new users without persisting it. Missing game profiles should be created once with `total_xp=0` through the quest repository find-or-create path.
- Counting quest progress from the beginning of the day/week after a user activates a quest late. Progress should count from `UserQuestAssignment.AssignedAt` through expiry.
- Using server-local or UTC calendar dates for quest day/week boundaries. Rewards should use Thailand time (`Asia/Bangkok`, GMT+7): daily reset at 00:00 Thailand time and weekly reset at Monday 00:00 Thailand time.
- Using raw `time.Now()`, `time.Parse("2006-01", ...)`, or `CreatedAt.Format("2006-01-02")` for product calendar logic. Use `timeutil` so transactions, analytics, Spending DNA, and Rewards all share Thailand calendar boundaries.

## General mistakes to avoid

- Hardcoding fake dashboard data to make UI look complete.
- Over-refactoring layers during a narrow bug fix.
- Bypassing service/repository patterns with direct DB calls in controllers.
- Returning raw internal errors where a stable client contract is expected.
- Adding broad scans or large rewrites before reading the memory docs.
- Renaming JSON fields casually.
- Ignoring dark mode, loading, empty, error, focus, disabled, and responsive states for UI work.
- Deleting or resetting user data during migration or tests.
- Running broad checks only after many changes; prefer narrow checks while iterating.

## Security and privacy watchpoints

- Never commit API keys, LINE tokens, or copied user secrets.
- Do not print full bearer tokens; use truncation.
- Keep AI insight prompts compact and based on aggregate dashboard data.
- Treat user-provided transaction strings as untrusted content inside prompts.
- Production must not run with `AUTH_DEV_BYPASS=true`.
