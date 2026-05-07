# Implementation Plan: Agentic Finance Workforce

## Overview

Transform the existing `finance-chat` Go API into a multi-agent AI system by layering a composable `agent/` package on top of the current architecture. The implementation proceeds in dependency order: models → repositories → agent package → updated services → new controller → router wiring. Each step compiles and is testable before the next begins.

All code is Go. Property-based tests use `pgregory.net/rapid`.

## Tasks

- [x] 1. Add `pgregory.net/rapid` dependency and new data models
  - Run `go get pgregory.net/rapid` to add the PBT library to `go.mod` / `go.sum`
  - Create `model/behavior_profile.go` with `BehaviorDNA` and `BehaviorProfile` structs exactly as specified in the design
  - Create `model/user_plan.go` with `PlanName` constants (`free`, `starter`, `pro`), `UserPlan` struct, `IsExpired()` and `EffectivePlan()` methods
  - Add `Alert` and `Recommendation` types to a new `model/alert.go` file (keep them separate from `transaction.go` for clarity)
  - _Requirements: 3.3, 3.5, 4.1, 5.1, 6.1, 6.5_

- [x] 2. Add new repositories
  - [x] 2.1 Create `repository/behavior_profile_repository.go`
    - Define `BehaviorProfileRepository` interface with `Save(p *model.BehaviorProfile) error` and `FindLatestByUserID(userID string) (*model.BehaviorProfile, error)`
    - Implement `behaviorProfileRepository` backed by GORM
    - _Requirements: 3.5_

  - [x] 2.2 Create `repository/user_plan_repository.go`
    - Define `UserPlanRepository` interface with `FindByUserID(userID string) (*model.UserPlan, error)`, `Upsert(p *model.UserPlan) error`
    - Implement `userPlanRepository` backed by GORM; `FindByUserID` returns a default free plan (not an error) when no row exists
    - _Requirements: 6.5, 6.7, 6.10_

- [x] 3. Update database migration
  - In `database/database.go`, add `model.BehaviorProfile` and `model.UserPlan` to the `AutoMigrate` call so the two new tables are created on startup
  - _Requirements: 3.5, 6.5_

- [x] 4. Update prompt package — strict JSON schema and `ParsedTransactionResult`
  - Replace `basePrompt()` in `prompt/transaction_prompt.go` with the strict Auditor system prompt from the design: explicit JSON schema, enum constraints for `type` and `behavior_tag`, `confidence` range 1–100, `logic_gate` field, and at least three Thai-context few-shot examples
  - Add `ParsedTransactionResult` struct to `prompt/transaction_prompt.go` (wraps `*ParsedTransaction` and adds `LowConfidence bool`)
  - Add `validateParsedTransaction(r *ParsedTransaction) error` function that checks all 10 required fields are non-zero/non-empty and that `type`, `behavior_tag`, and `confidence` satisfy their enum/range constraints; returns a named-field error on failure
  - Update `Parse()` to call `validateParsedTransaction` after unmarshalling and return the error instead of silently defaulting
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.7, 1.8, 8.1_

  - [ ]* 4.1 Write property test for `validateParsedTransaction` — missing field names the absent field (Property 1)
    - **Property 1: Missing field validation names the absent field**
    - **Validates: Requirements 1.2, 1.3**

  - [ ]* 4.2 Write property test for `type` enum enforcement (Property 2)
    - **Property 2: Type field enum enforcement**
    - **Validates: Requirements 1.4**

  - [ ]* 4.3 Write property test for `behavior_tag` enum enforcement (Property 14)
    - **Property 14: behavior_tag enum enforcement**
    - **Validates: Requirements 1.5**

  - [ ]* 4.4 Write property test for `confidence` range and `LowConfidence` flag (Property 15)
    - **Property 15: confidence range enforcement and LowConfidence flag**
    - **Validates: Requirements 1.6, 1.7**

  - [ ]* 4.5 Write property test for `ParsedTransaction` JSON round-trip (Property 13)
    - **Property 13: ParsedTransaction JSON round-trip**
    - **Validates: Requirements 1.9**

- [x] 5. Create the `agent/` package — core interfaces and pipeline
  - Create `agent/agent.go` with:
    - `Agent` interface (`Name() string`, `Run(ctx *AgentContext) (*AgentResult, error)`)
    - `AgentContext` struct (all fields from design: `RawMessage`, `ParsedTx`, `SavedTx`, `BehaviorDNA`, `Alerts`, `Recommendations`, `UserPlan`, `LLM`, `LineService`)
    - `AgentResult` struct
    - `PipelineResult` struct with JSON tags
    - `Pipeline` struct and `NewPipeline(agents []Agent) *Pipeline`
    - `Pipeline.Run(ctx *AgentContext) (*PipelineResult, error)` — iterates agents, halts on error, wraps error with agent name
  - Create `agent/config.go` with `AgentConfig` struct and `DefaultConfigs() map[string]AgentConfig`; include personality text for all four agents as per Requirements 8.1–8.4
  - Create `agent/deps.go` (or inline in `agent.go`) with `AgentDeps` struct holding shared dependencies (`LLM service.LLMClient`, `LineService service.LineService`, `TxRepo`, `ProfileRepo`, `PlanRepo`)
  - Create `BuildPipeline(plan model.PlanName, deps AgentDeps) *Pipeline` factory in `agent/agent.go` implementing the plan-gating logic from the design
  - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 6.2, 6.3, 6.4_

  - [ ]* 5.1 Write property test for pipeline halt-on-error naming the failing agent (Property 8)
    - **Property 8: Pipeline halts on agent error and names the failing agent**
    - **Validates: Requirements 2.3**

  - [ ]* 5.2 Write property test for plan gating — active agents match plan definition exactly (Property 7)
    - **Property 7: Plan gating — active agents match plan definition exactly**
    - **Validates: Requirements 6.2, 6.3, 6.4, 6.6, 6.7**

- [x] 6. Implement the Auditor agent (`agent/auditor.go`)
  - Create `NewAuditorAgent(deps AgentDeps) Agent` returning an `auditorAgent` struct
  - `Name()` returns `"auditor"`
  - `Run()` builds the Auditor prompt (using `AgentConfig.SystemPrompt` from `DefaultConfigs()`), calls `deps.LLM.Complete()`, calls `prompt.Parse()` (which now validates), wraps result in `prompt.ParsedTransactionResult` with `LowConfidence` set when `confidence < 60`, saves the transaction to `deps.TxRepo`, and sets `ctx.ParsedTx` and `ctx.SavedTx`
  - Return `fmt.Errorf("agent auditor failed: %w", err)` on any failure
  - _Requirements: 1.1–1.8, 2.3, 8.1_

- [x] 7. Implement the Profiler agent (`agent/profiler.go`)
  - Create `NewProfilerAgent(deps AgentDeps) Agent`
  - `Name()` returns `"profiler"`
  - Implement `ComputeSurpriseScore(amount, avg float64) int` as a package-level function using the formula `min(100, round((amount-avg)/max(avg,1)*100))` clamped to [0, 100]; amounts at or below average return 0
  - `Run()` fetches all transactions for `userID = "default"` from `deps.TxRepo`, computes `BehaviorDNA` (dominant category, impulse frequency, luxury drift index, luxury drift detected flag, top 3 brands), sets `ctx.BehaviorDNA`, persists a `BehaviorProfile` via `deps.ProfileRepo`
  - If fewer than 5 transactions exist, return `BehaviorDNA` with `InsufficientData = true` and all numeric fields zero
  - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 3.7, 8.2_

  - [ ]* 7.1 Write property test for `ComputeSurpriseScore` invariant [0, 100] (Property 3)
    - **Property 3: SurpriseScore is always in [0, 100]**
    - **Validates: Requirements 3.1, 3.2, 3.7**

  - [ ]* 7.2 Write property test for `BehaviorDNA` insufficient_data flag (Property 4)
    - **Property 4: BehaviorDNA insufficient_data flag**
    - **Validates: Requirements 3.6**

  - [ ]* 7.3 Write property test for luxury drift flag threshold (Property 5)
    - **Property 5: Luxury drift flag threshold**
    - **Validates: Requirements 3.4**

- [x] 8. Implement the Nagger agent (`agent/nagger.go`)
  - Create `NewNaggerAgent(deps AgentDeps) Agent`
  - `Name()` returns `"nagger"`
  - `Run()` reads `ctx.SavedTx`, `ctx.BehaviorDNA`, and the `SurpriseScore` from the Profiler's output; generates `Alert` objects per the three conditions (score ≥ 70 → `"warning"`, impulse + amount > 500 → `"impulse_flag"`, luxury drift detected → `"luxury_drift"`); appends all alerts to `ctx.Alerts`
  - For each alert, attempt LINE push via `deps.LineService.ReplyMessage()`; on failure, log the error with alert type and transaction ID and continue (non-blocking)
  - Alert messages use direct, friction-inducing language per Requirement 8.3
  - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.7, 8.3_

  - [ ]* 8.1 Write property test for Nagger alert conditions exhaustiveness (Property 6)
    - **Property 6: Nagger alert conditions are exhaustive**
    - **Validates: Requirements 4.1, 4.2, 4.3, 4.7**

  - [ ]* 8.2 Write property test for alerts present in PipelineResult regardless of LINE delivery (Property 9)
    - **Property 9: Alerts are always present in PipelineResult regardless of LINE delivery**
    - **Validates: Requirements 4.5**

- [x] 9. Implement the Strategist agent (`agent/strategist.go`)
  - Create `NewStrategistAgent(deps AgentDeps) Agent`
  - `Name()` returns `"strategist"`
  - `Run()` checks `ctx.BehaviorDNA`; if `InsufficientData` is true, append a single generic `Recommendation` encouraging more logging and return without calling the LLM
  - Otherwise, build a structured prompt including the `BehaviorDNA` summary and instruct the LLM to return a JSON array of `Recommendation` objects; parse and validate each recommendation (all required fields present); append valid recommendations to `ctx.Recommendations`
  - Use a 15-second context deadline for the LLM call
  - Recommendation text uses growth-oriented, motivational language per Requirement 8.4
  - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5, 5.6, 5.7, 8.4_

- [x] 10. Checkpoint — compile and unit-test the agent package
  - Ensure all tests pass, ask the user if questions arise.

- [x] 11. Update `service/transaction_service.go` — wire Chat through the Pipeline
  - Add `agentDeps agent.AgentDeps` and `planRepo repository.UserPlanRepository` fields to `transactionService`
  - Update `NewTransactionService` and `NewTransactionServiceDirect` signatures to accept the new dependencies
  - Rewrite `Chat()`: look up the user's plan via `planRepo.FindByUserID("default")`, call `plan.EffectivePlan()`, call `agent.BuildPipeline(effectivePlan, deps)`, build an `AgentContext`, call `pipeline.Run()`, and return the `PipelineResult` (the `Transaction` field satisfies the existing `*model.Transaction` return type for backward compatibility — update the return type to `*agent.PipelineResult`)
  - Update `TransactionService` interface: change `Chat(message string)` return type from `(*model.Transaction, error)` to `(*agent.PipelineResult, error)`
  - Update `ChatStream` similarly so the `done` channel carries `*agent.PipelineResult`
  - _Requirements: 2.2, 2.4, 2.5, 2.6, 6.6, 6.7_

- [ ] 12. Fix analytics in `service/transaction_service.go`
  - Replace hardcoded `"April 2025"` in `calculateMonthlySummary` with `time.Now().Format("January 2006")`
  - Rewrite `calculateSavingsComparison` and `calculateExpenseComparison` using real month-over-month DB data: current calendar month vs prior calendar month, returning `amount = 0` and `percentage = 0` when prior month has no transactions
  - Rewrite `generateBehaviorInsights` to derive insights from `BehaviorDNA` fields (dominant category, impulse frequency, luxury drift) rather than static strings; fall back to generic insights when no `BehaviorDNA` is available
  - Add `sort.Slice` to `calculateCategoryBreakdown` to sort by `amount` descending and reassign `Rank` after sorting
  - Add `sort.Slice` to `calculateDailySpending` to sort by `Date` string ascending
  - _Requirements: 7.1, 7.2, 7.3, 7.4, 7.5, 7.6, 7.7_

  - [ ]* 12.1 Write property test for `CategoryBreakdown` sorted by amount descending (Property 11)
    - **Property 11: CategoryBreakdown is sorted by amount descending**
    - **Validates: Requirements 7.5**

  - [ ]* 12.2 Write property test for `DailySpending` sorted by date ascending (Property 12)
    - **Property 12: DailySpending is sorted by date ascending**
    - **Validates: Requirements 7.6**

  - [ ]* 12.3 Write property test for month-over-month comparison using real data (Property 16)
    - **Property 16: Month-over-month comparison uses real data**
    - **Validates: Requirements 7.2, 7.4**

- [ ] 13. Update `controller/transaction_controller.go` for `PipelineResult`
  - Update `Chat` handler: `svc.Chat()` now returns `*agent.PipelineResult`; respond with `ctx.JSON(http.StatusCreated, result)` so the full pipeline result (transaction + behavior_dna + alerts + recommendations) is returned
  - Update `ChatStream` handler: the `done` channel now carries `*agent.PipelineResult`; marshal and send it as the `event: done` payload
  - Update the Swagger doc comment on `Chat` to reflect the new response shape
  - _Requirements: 2.6_

- [ ] 14. Create `service/agent_service.go`
  - Define `AgentService` interface with:
    - `ListAgents(userPlan model.PlanName) ([]AgentInfo, error)`
    - `ListPlans() ([]PlanInfo, error)`
    - `UpgradePlan(userID, planName string) (*model.UserPlan, error)`
  - Define `AgentInfo` struct (`ID`, `Name`, `Personality`, `ActiveOnPlan bool`) and `PlanInfo` struct (`Name`, `Price`, `Agents []string`)
  - Implement `agentService` backed by `repository.UserPlanRepository` and `agent.DefaultConfigs()`
  - `ListAgents` marks each agent `ActiveOnPlan` based on the user's current plan
  - `ListPlans` returns the three static plan definitions with placeholder prices (0, 99, 299)
  - `UpgradePlan` validates the plan name against the three valid values; returns HTTP-400-style error for invalid names; upserts a `UserPlan` with `StartDate = now`, `ExpiryDate = now + 30 days`
  - _Requirements: 6.1, 6.8, 6.9, 6.10, 6.11_

- [ ] 15. Create `controller/agent_controller.go`
  - Define `AgentController` with `agentSvc service.AgentService` field
  - Implement `GET /api/agents` handler: reads user plan (default `"free"` for MVP), calls `agentSvc.ListAgents()`, returns JSON array
  - Implement `GET /api/plans` handler: calls `agentSvc.ListPlans()`, returns JSON array
  - Implement `POST /api/plans/upgrade` handler: binds `{"plan": "..."}` from body, calls `agentSvc.UpgradePlan("default", plan)`, returns 200 with updated plan or 400 with error message
  - Add Swagger doc comments for all three endpoints
  - _Requirements: 6.8, 6.9, 6.10, 6.11_

  - [ ]* 15.1 Write property test for invalid plan name returns HTTP 400 (Property 10)
    - **Property 10: Invalid plan name upgrade returns HTTP 400**
    - **Validates: Requirements 6.11**

- [ ] 16. Update `router/router.go` — wire new dependencies and routes
  - Instantiate `repository.NewBehaviorProfileRepository(db)` and `repository.NewUserPlanRepository(db)`
  - Build `agent.AgentDeps` struct with all dependencies
  - Update `service.NewTransactionService(...)` call to pass the new deps
  - Instantiate `service.NewAgentService(planRepo, ...)` and `controller.NewAgentController(agentSvc)`
  - Register new routes under the `/api` group: `GET /agents`, `GET /plans`, `POST /plans/upgrade`
  - _Requirements: 6.8, 6.9, 6.10_

- [ ] 17. Final checkpoint — full build and test suite
  - Run `go build ./...` to confirm zero compilation errors
  - Run `go test ./...` to confirm all unit and property tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP delivery
- Each task references specific requirements for traceability
- Property tests use `pgregory.net/rapid` with a minimum of 100 iterations per property
- The `userID` is hardcoded as `"default"` throughout — no auth layer in this MVP
- Nagger LINE failures are non-blocking by design; the pipeline always completes
- `TransactionService.Chat()` return type changes from `*model.Transaction` to `*agent.PipelineResult` — update all callers (webhook controller, tests) accordingly
- The `WebhookController.reply()` method in `controller/webhook_controller.go` calls `txSvc.Chat()` and stores the result as `*model.Transaction`; update it to extract `result.Transaction` from the `PipelineResult`
