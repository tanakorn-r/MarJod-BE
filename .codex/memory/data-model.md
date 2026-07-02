# Data Model

The database is managed through GORM models in `model/` and migrated by `database.Migrate`.

## Ownership

Most user-owned tables have `user_id` with default `default`. Always normalize with `model.UserIDOrDefault(userID)`.

Legacy rows created before LINE ownership are backfilled to `default`; they cannot be reliably assigned to real LINE users after the fact unless external mapping exists.

## Transaction

`model.Transaction`

- `ID uint` — primary key.
- `UserID string` — owner boundary; required for isolation.
- `RawMessage string` — original user text.
- `Type TransactionType` — `income` or `expense`.
- `Amount float64` — normalized absolute amount.
- `Category string` — high-level category, e.g. `Food & Beverage`.
- `SubCategory string` — finer category, e.g. `Coffee`.
- `Brand string` — merchant/brand; `General` if unknown.
- `Description string` — display text.
- `BehaviorTag string` — `impulse`, `necessity`, `social`, `treat`, `recurring`.
- `CreatedAt time.Time` — stamped in Thailand time (`Asia/Bangkok`, GMT+7) for new transactions and used for monthly analytics, trends, daily charts, quests.
- `WalletID *uint` — current wallet assignment.

AI parsing, analytics, quests, spending DNA, and frontend charts depend heavily on `Type`, `Amount`, `Category`, `SubCategory`, `Brand`, `BehaviorTag`, `CreatedAt`, `WalletID`, and `UserID`. Calendar grouping should use `timeutil` Thailand helpers, not raw UTC/server-local date formatting.

## UserCorrection

Stores manual classification fixes:

- `UserID`
- `RawMessage`
- `Category`
- `SubCategory`
- `Brand`
- `BehaviorTag`
- `CreatedAt`

Corrections may be injected into future prompts as compact examples. Keep them user-scoped.

## BehaviorProfile and BehaviorDNA

`BehaviorDNA` is an API/domain struct. `BehaviorProfile` persists flattened computed metrics:

- `DominantCategory`
- `ImpulseFrequency`
- `LuxuryDriftIndex`
- `LuxuryDriftDetected`
- `TopBrands` as JSON-encoded string
- `InsufficientData`

The profiler uses transaction history and writes profiles for starter/pro users.

## UserPlan

`UserPlan` stores a unique row per user:

- `Plan`: `free`, `starter`, `pro`
- `StartDate`
- `ExpiryDate`

`EffectivePlan()` falls back to `free` after expiry. Plan controls the agent pipeline:

- free — auditor
- starter — auditor, profiler, nagger
- pro — auditor, profiler, nagger, strategist

## Wallet

`Wallet` stores budget containers:

- `UserID`
- `Name`
- `Icon`
- `Target`
- `Spent` is computed (`gorm:"-"`), not persisted.
- `IsArchived`
- `StartDate`, `EndDate`

`GeneralWalletID` is `0`, but repository behavior resolves that compatibility ID to a persisted General wallet via `EnsureGeneralWallet`. Be careful around `wallet_id=0`, `NULL`, and persisted General wallet IDs.

Updating wallet `id=0` should ensure a persisted General wallet exists, update that row, and return the persisted wallet. Clients should prefer the returned persisted ID for follow-up analytics/current-wallet calls.

Analytics must resolve `wallet_id=0` to the persisted General wallet before reading `monthly_summary.target` or filtering transactions; otherwise it may show the hardcoded virtual default instead of the updated target.

Transaction list and Spending DNA also support optional wallet filtering through `/api/transactions?wallet_id=...` and `/api/analytics/dna?wallet_id=...`; when `wallet_id=0`, resolve to the persisted General wallet before filtering transactions.

`UserWalletSelection` stores the current wallet per user. LINE/webhook chat transaction creation must resolve this selection before saving a transaction. If a valid selection already exists, use it. If no valid selection exists for any reason, create/find the persisted General wallet and select it. Transactions should save the persisted wallet ID, not the virtual `GeneralWalletID` (`0`). Do not fall back to the first active custom wallet; that can make transaction placement feel random.

## Quests

`QuestPreset` stores editable DB-backed preset quest definitions:

- `Key` — unique stable template key used by `UserQuestAssignment.TemplateKey`.
- `Name` — display name/title returned in quest views.
- `Logo` — small emoji/icon for Rewards UI.
- `Period` — `daily` or `weekly`.
- `Difficulty` — `basic`, `advanced`, `expert`, `master`, `grand_master`; legacy `easy`, `medium`, `hard` remain accepted.
- `XP` — reward amount granted once per assignment.
- `Accent` — display accent color string.
- `RuleType` — evaluator identifier (`log_transaction_count`, `spend_cap`, `no_impulse`, `no_spend_days`, `category_spend_cap`, `coffee_spend_cap`, `beat_last_month_savings_rate`).
- `Target`, `Unit`, `Category`, `SubCategory`, `BehaviorTag` — rule metadata used by quest progress evaluation.
- `IsActive` — active presets are eligible for new random assignments; inactive presets remain readable for old assignments.

Default daily/weekly quest presets are seeded by `database.Migrate` from `model.DefaultQuestPresets()` using conflict-do-nothing behavior on `quest_presets.key`. This makes next-version deploys create missing presets without overwriting edited rows. Do not rely only on request-time lazy creation for preset availability.

`UserQuestAssignment` stores assigned quest templates and completion/payout state:

- `UserID`
- `BatchID`
- `Period` — `daily` or `weekly`.
- `TemplateKey`
- `AssignedAt`
- `ExpiresAt`
- `CompletedAt`
- `XPAwarded`

Quest progress is computed live from transactions after the assignment's `AssignedAt`; do not store derived progress unless intentionally redesigning. Quest day/week grouping uses Thailand time (`Asia/Bangkok`, GMT+7), not server-local or UTC dates. A quest assignment is active only when both `AssignedAt` is present and `ExpiresAt` is in the future; missing dates should make that period return an empty quest list.

`UserGameProfile` stores total XP by user. Level is derived in service code. The quest repository uses find-or-create behavior, so a missing profile is persisted once with `total_xp=0` the first time Rewards evaluation needs it; the async transaction quest hook creates this row after a user's first saved transaction even if no quest XP is earned yet.

Quest batches are not created by `GET /api/quests`. Users explicitly activate a random daily or weekly batch via `POST /api/quests/generate`. Only one daily set is allowed per Thailand day and only one weekly set is allowed per Thailand week; enforce this by checking existing `UserQuestAssignment.AssignedAt` rows within the current period window. Daily batches expire at the next Thailand midnight (`Asia/Bangkok`, GMT+7); weekly batches expire after Sunday at the next Monday 00:00 Thailand time. Expired rows must remain in the DB for history and should not be deleted or reused. Template keys must match `quest_presets.key`. Current difficulty ladder values are `basic`, `advanced`, `expert`, `master`, and `grand_master`; old `easy`, `medium`, `hard` values remain for compatibility.

The quest board response should expose preset metadata (`title`, `logo`, `difficulty`, `xp`, `accent`) plus persisted assignment state (`period`, `assigned_at`, `expires_at`, `completed_at`, `xp_awarded`), separated `daily_quests` and `weekly_quests`, level-gated `levels`/`rewards`, and a `stats` summary derived from active batch rows plus `UserGameProfile.TotalXP`. Do not create mock quest rows per user; assign preset templates by writing `UserQuestAssignment` rows.

## Prompt parser DTO

`prompt.ParsedTransaction` requires all fields:

- `raw_message`
- `type`
- `amount`
- `category`
- `sub_category`
- `brand`
- `behavior_tag`
- `description`
- `logic_gate`
- `confidence`

Parser validation intentionally rejects incomplete LLM responses. Do not remove required fields just to make tests pass; update prompt/tests together if schema changes.
