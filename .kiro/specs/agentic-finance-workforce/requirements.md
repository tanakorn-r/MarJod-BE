# Requirements Document

## Introduction

The Agentic Finance Workforce transforms the existing passive Go finance chat API into an active, multi-agent AI system. Instead of a single monolithic LLM call, the system is modelled as a "staff" of specialized AI agents — each with a distinct personality, responsibility, and goal. Users interact with this workforce through the existing chat interface and a new agent management dashboard. A SaaS subscription model gates access to advanced agents, allowing users to "hire" more powerful agents by upgrading their plan.

The four core agents are:

| Agent | Role | Primary Goal |
|---|---|---|
| **The Auditor** | Parses raw chat into structured JSON | Accuracy & Speed |
| **The Profiler** | Calculates Surprise Score and Behavior DNA from history | Detecting Luxury Drift |
| **The Nagger** | Sends real-time alerts when spending patterns break | Friction & Discipline |
| **The Strategist** | Suggests investments/savings based on behavior profile | Growth & Net Worth |

---

## Glossary

- **Agent**: A specialized AI worker with a defined role, personality, and LLM prompt strategy.
- **Auditor**: The agent responsible for parsing raw natural-language messages into structured `Transaction` JSON. Corresponds to the existing Ollama LLM flow.
- **Profiler**: The agent responsible for computing behavioral metrics (Surprise Score, Behavior DNA) from the user's transaction history.
- **Nagger**: The agent responsible for detecting real-time spending pattern violations and dispatching alerts (LINE message, in-app notification).
- **Strategist**: The agent responsible for generating personalized investment and savings recommendations based on the Behavior DNA.
- **Surprise Score**: A numeric score (0–100) representing how much a transaction deviates from the user's established spending baseline for that category.
- **Behavior DNA**: A structured profile object summarizing the user's recurring spending patterns, dominant categories, impulse frequency, and luxury drift index.
- **Luxury Drift**: A trend where discretionary/luxury spending gradually increases over time relative to income.
- **Behavior Tag**: An existing field on `Transaction` — one of: `impulse`, `necessity`, `social`, `treat`, `recurring`.
- **Plan**: A subscription tier that determines which agents a user has access to. Plans: `free`, `starter`, `pro`.
- **Agent Marketplace**: The UI and API surface where users view available agents and manage their active plan.
- **Workforce**: The set of agents currently active for a given user.
- **Pipeline**: The ordered sequence of agent invocations triggered by a single user chat message.
- **LLMClient**: The existing Go interface (`Complete` / `CompleteStream`) used to communicate with the Ollama backend.
- **Transaction**: The existing GORM model storing a parsed financial event.
- **UserCorrection**: The existing GORM model storing user-supplied classification fixes used as few-shot examples.
- **JSON Schema**: The strict output contract the Auditor must produce, matching the `ParsedTransaction` struct fields exactly.
- **LINE**: The messaging platform integrated via webhook for push notifications.

---

## Requirements

### Requirement 1: Auditor Agent — Strict JSON Output Alignment

**User Story:** As a developer, I want the Auditor agent's LLM prompt to enforce a strict JSON schema, so that downstream agents always receive well-formed, predictable data without manual normalization hacks.

#### Acceptance Criteria

1. THE Auditor SHALL include an explicit JSON schema definition in its system prompt, listing every field name, type, and allowed values for the `ParsedTransaction` output.
2. WHEN the LLM response contains a JSON object, THE Auditor SHALL validate that all required fields (`raw_message`, `type`, `amount`, `category`, `sub_category`, `brand`, `behavior_tag`, `description`, `logic_gate`, `confidence`) are present before accepting the result.
3. IF a required field is missing from the LLM response, THEN THE Auditor SHALL return a structured error indicating which field is absent, rather than silently defaulting.
4. THE Auditor SHALL constrain the `type` field to exactly the values `"income"` or `"expense"` within the prompt, and THE Auditor SHALL reject any response where `type` is neither value.
5. THE Auditor SHALL constrain the `behavior_tag` field to exactly the values `"impulse"`, `"necessity"`, `"social"`, `"treat"`, or `"recurring"` within the prompt, and THE Auditor SHALL reject any response where `behavior_tag` is outside this set.
6. THE Auditor SHALL constrain the `confidence` field to an integer in the range 1–100 within the prompt.
7. WHEN the `confidence` score is below 60, THE Auditor SHALL include a `low_confidence` flag in the parsed result so callers can surface a review prompt to the user.
8. THE Auditor SHALL include at least three Thai-context few-shot examples in its prompt that demonstrate correct field alignment between `brand`, `category`, `sub_category`, and `behavior_tag`.
9. FOR ALL valid natural-language inputs, parsing the input and re-serializing the resulting `ParsedTransaction` to JSON SHALL produce a document that passes JSON schema validation (round-trip property).

---

### Requirement 2: Agent Pipeline Architecture

**User Story:** As a backend engineer, I want a composable agent pipeline, so that each agent can be developed, tested, and replaced independently without rewriting the entire chat flow.

#### Acceptance Criteria

1. THE System SHALL define an `Agent` interface with at minimum a `Name() string` method and a `Run(ctx AgentContext) (AgentResult, error)` method.
2. THE Pipeline SHALL execute agents in a defined order: Auditor → Profiler → Nagger → Strategist.
3. WHEN an agent in the Pipeline returns an error, THE Pipeline SHALL halt execution of subsequent agents and return the error to the caller with the name of the failing agent.
4. THE Pipeline SHALL pass a shared `AgentContext` struct between agents, allowing each agent to read outputs produced by earlier agents in the same pipeline run.
5. THE System SHALL allow the Pipeline to be configured with a subset of agents based on the user's active Plan, so that agents unavailable on the user's Plan are skipped without error.
6. WHEN the Pipeline completes successfully, THE System SHALL return a `PipelineResult` containing the parsed `Transaction`, the `BehaviorProfile`, any `Alert` objects raised by the Nagger, and any `Recommendation` objects produced by the Strategist.
7. THE Pipeline SHALL complete the Auditor stage within 10 seconds for a single chat message under normal LLM load.

---

### Requirement 3: Profiler Agent — Surprise Score and Behavior DNA

**User Story:** As a user, I want the system to understand my spending personality over time, so that I can see when my habits are drifting toward luxury or impulse spending.

#### Acceptance Criteria

1. WHEN a new `Transaction` is saved, THE Profiler SHALL compute a `SurpriseScore` (integer 0–100) for that transaction by comparing its amount to the user's rolling 30-day average for the same `category`.
2. THE Profiler SHALL define `SurpriseScore` as: `min(100, round((amount - avg) / max(avg, 1) * 100))`, where `avg` is the 30-day category average and amounts below average produce a score of 0.
3. THE Profiler SHALL compute a `BehaviorDNA` profile object containing: dominant category (by total spend), impulse frequency (count of `behavior_tag = "impulse"` in last 30 days), luxury drift index (percentage change in discretionary spend vs. prior 30-day period), and top 3 brands by frequency.
4. WHEN the luxury drift index exceeds 20%, THE Profiler SHALL set a `luxury_drift_detected` boolean flag to `true` in the `BehaviorDNA`.
5. THE Profiler SHALL persist the computed `BehaviorDNA` to a `behavior_profiles` database table, keyed by user ID and computed date.
6. IF fewer than 5 transactions exist in the user's history, THEN THE Profiler SHALL return a `BehaviorDNA` with all metrics set to zero and a `insufficient_data` flag set to `true`.
7. FOR ALL transaction histories, the `SurpriseScore` SHALL remain in the range [0, 100] regardless of transaction amount or category average (invariant property).

---

### Requirement 4: Nagger Agent — Real-Time Spending Alerts

**User Story:** As a user, I want to receive an immediate alert when I break my spending patterns, so that I feel friction before making impulsive purchases.

#### Acceptance Criteria

1. WHEN a transaction's `SurpriseScore` is 70 or above, THE Nagger SHALL generate an `Alert` of type `"warning"` containing the category, the transaction amount, and the 30-day category average.
2. WHEN a transaction's `behavior_tag` is `"impulse"` and the amount exceeds 500 (base currency), THE Nagger SHALL generate an `Alert` of type `"impulse_flag"`.
3. WHEN the `luxury_drift_detected` flag in `BehaviorDNA` is `true`, THE Nagger SHALL generate an `Alert` of type `"luxury_drift"` with a message describing the percentage increase.
4. WHERE the LINE integration is enabled, THE Nagger SHALL push each generated `Alert` as a LINE message to the user within 5 seconds of the transaction being saved.
5. THE Nagger SHALL include all generated `Alert` objects in the `PipelineResult` regardless of whether LINE delivery succeeds, so the API response always contains alert data.
6. IF the LINE push fails, THEN THE Nagger SHALL log the failure with the alert type and transaction ID, and THE Nagger SHALL NOT block the pipeline from completing.
7. WHEN no alert conditions are met for a transaction, THE Nagger SHALL return an empty alerts list without error.

---

### Requirement 5: Strategist Agent — Savings and Investment Recommendations

**User Story:** As a user, I want personalized financial recommendations based on my actual spending behavior, so that I can grow my net worth rather than just track expenses.

#### Acceptance Criteria

1. WHEN the `BehaviorDNA` is available in the `AgentContext`, THE Strategist SHALL generate at least one `Recommendation` object containing a title, description, estimated monthly savings amount, and priority level (`high`, `medium`, `low`).
2. WHEN the luxury drift index in `BehaviorDNA` exceeds 20%, THE Strategist SHALL generate a `Recommendation` with priority `"high"` targeting the dominant discretionary category.
3. WHEN the impulse frequency in `BehaviorDNA` is 5 or more in the last 30 days, THE Strategist SHALL generate a `Recommendation` suggesting a spending cap for the top impulse category.
4. THE Strategist SHALL use the LLMClient to generate recommendation text, with a structured prompt that includes the `BehaviorDNA` summary and instructs the LLM to return a JSON array of `Recommendation` objects.
5. THE Strategist SHALL validate that each LLM-generated `Recommendation` contains all required fields before including it in the `PipelineResult`.
6. IF the `BehaviorDNA` has `insufficient_data` set to `true`, THEN THE Strategist SHALL return a single generic `Recommendation` encouraging the user to log more transactions, without calling the LLM.
7. THE Strategist SHALL complete recommendation generation within 15 seconds under normal LLM load.

---

### Requirement 6: SaaS Agent Marketplace and Plan Management

**User Story:** As a product owner, I want users to be able to "hire" agents by subscribing to a plan, so that the system generates revenue while giving free users a useful baseline experience.

#### Acceptance Criteria

1. THE System SHALL define three subscription plans: `free`, `starter`, and `pro`, each with a defined set of included agents.
2. THE System SHALL include the Auditor agent on all plans including `free`.
3. THE System SHALL include the Profiler and Nagger agents on the `starter` and `pro` plans only.
4. THE System SHALL include the Strategist agent on the `pro` plan only.
5. THE System SHALL persist a `user_plans` table with fields: user ID, plan name, plan start date, and plan expiry date.
6. WHEN a user's plan is `free`, THE Pipeline SHALL skip the Profiler, Nagger, and Strategist agents and return only the parsed `Transaction` in the `PipelineResult`.
7. WHEN a user's plan has expired, THE System SHALL treat the user as `free` tier until the plan is renewed.
8. THE System SHALL expose a `GET /api/agents` endpoint that returns the list of all available agents, each with name, description, personality summary, and whether the agent is active on the user's current plan.
9. THE System SHALL expose a `GET /api/plans` endpoint that returns all available plans with their included agents and a placeholder price field.
10. THE System SHALL expose a `POST /api/plans/upgrade` endpoint that accepts a plan name and updates the user's active plan, returning the updated plan record.
11. WHEN a plan upgrade is requested with an invalid plan name, THE System SHALL return HTTP 400 with a descriptive error message.

---

### Requirement 7: Analytics Dashboard — Real Behavioral Data

**User Story:** As a user, I want the analytics dashboard to show real behavioral insights derived from my actual transaction history, so that I can trust the data I see.

#### Acceptance Criteria

1. THE System SHALL replace all hardcoded mock values in `GetAnalytics()` with calculations derived from the actual `Transaction` records in the database.
2. THE System SHALL compute `SavingsComparison` and `ExpenseComparison` by comparing the current calendar month's totals against the prior calendar month's totals from real transaction data.
3. THE System SHALL populate `BehaviorInsights` using the `BehaviorDNA` computed by the Profiler, rather than static strings.
4. WHEN no transactions exist for the prior month, THE System SHALL return comparison metrics with `amount` of 0 and `percentage` of 0, rather than hardcoded values.
5. THE System SHALL sort `CategoryBreakdown` entries by `amount` descending before returning them.
6. THE System SHALL sort `DailySpending` entries by `date` ascending before returning them.
7. WHEN the `MonthlySummary` month label is computed, THE System SHALL derive it from the current system date rather than a hardcoded string.

---

### Requirement 8: Agent Personality and Prompt Identity

**User Story:** As a user, I want each agent to have a distinct personality in its responses, so that interacting with the workforce feels engaging rather than generic.

#### Acceptance Criteria

1. THE Auditor's system prompt SHALL identify the agent as "The Chief Auditor" with a tone that is precise, terse, and audit-focused, using language like "Verified:", "Rejected:", and "Flagged:".
2. THE Profiler's system prompt SHALL identify the agent as "The Profiler" with a tone that is analytical and pattern-focused, referencing behavioral psychology concepts.
3. THE Nagger's alert messages SHALL use direct, friction-inducing language (e.g., "You just spent 3× your usual amount on coffee. Intentional?") rather than neutral informational text.
4. THE Strategist's recommendation text SHALL use growth-oriented, motivational language focused on net worth and long-term wealth building.
5. THE System SHALL store each agent's name, personality description, and prompt template as a configurable struct, so that personality text can be updated without changing business logic.

