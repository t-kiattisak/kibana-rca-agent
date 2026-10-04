# Architecture Guide: AI Multi-Agent Incident War Room

## 1. Overview & Motivation

In traditional SRE automation, automated Root Cause Analysis (RCA) typically relies on a **Single-Prompt monolithic LLM call**. While quick to implement, this pattern suffers from severe production weaknesses:
- **Context Dilution:** When code, infrastructure, and business rules are combined into one massive prompt, the model hallucinates or glosses over critical nuances.
- **Role Confusion:** A single prompt struggles to distinguish between an immediate platform mitigation (e.g., Pod restart) and a permanent code fix (e.g., defer transaction rollback).
- **Excessive Token Overhead:** Pushing all documentation into one prompt quickly exhausts model token budgets and cloud rate limits.

To solve this, our agent implements the **Multi-Agent War Room Pattern**, modeling a real-world incident response command center where specialized AI personas independently investigate an incident and debate solutions before the Incident Commander synthesizes a binding action plan.

---

## 2. Multi-Agent War Room Lifecycle

```mermaid
sequenceDiagram
    autonumber
    participant ES as Elasticsearch (Logs & Vector kNN)
    participant Coord as War Room Coordinator
    participant Tech as 🧑‍💻 Tech Lead Agent
    participant SRE as 🛠️ SRE Lead Agent
    participant Prod as 👔 Product Lead Agent
    participant Cmd as 👑 Incident Commander
    participant Discord as Discord Notification

    Note over Coord: Incident Spike Detected (>= 3 errors)
    Coord->>ES: 1. Fetch Correlated Logs & Error Signatures
    ES-->>Coord: Return 25 correlated error entries & traces

    par Role-Filtered Document Retrieval & Expert Investigation
        Coord->>ES: kNN Search (filter: role=developer)
        ES-->>Tech: Top-1 Architecture Chunk
        Coord->>Tech: Analyze Code & Stack Traces
        Tech-->>Coord: Perspective (Code Hotfix & Test Proposals)
    and
        Coord->>ES: kNN Search (filter: role=sre)
        ES-->>SRE: Top-1 Runbook Chunk
        Coord->>SRE: Analyze Infra & Pool Starvation
        SRE-->>Coord: Perspective (Rolling Restart & Pool Tuning)
    and
        Coord->>ES: kNN Search (filter: role=product)
        ES-->>Prod: Top-1 Business Spec Chunk
        Coord->>Prod: Analyze SLA & Revenue Impact
        Prod-->>Coord: Perspective (Degraded Mode & SLA Metrics)
    end

    Note over Coord,Cmd: 2. Consensus Synthesis Phase
    Coord->>Cmd: Debate Context (Incident + All 3 Expert Perspectives)
    Cmd-->>Coord: Synthesized Action Plan (Immediate, Short-term, Permanent)

    Note over Coord,Discord: 3. Delivery Phase
    Coord->>Discord: Post Multi-Agent War Room Card with Token Metrics
```

---

## 3. Specialized Role Specifications

Each role implements the Go interface `ExpertAgent` (`internal/role/base.go`):

```go
type ExpertAgent interface {
    Name() string
    RoleTitle() string
    Analyze(ctx context.Context, incident *model.IncidentContext, docs []KnowledgeDoc) (*model.RolePerspective, error)
}
```

### 🧑‍💻 1. Software Tech Lead & Code Architect (`internal/role/tech_lead_agent.go`)
- **Primary Focus:** Application source code, stack traces, transaction lifecycles, and context propagation.
- **Investigation Targets:**
  - `nil-pointer dereference` panics and missing defensive checks.
  - Context deadline omissions (`context.WithTimeout`).
  - Database connection leaks (missing `defer tx.Rollback()`).
  - Unit/Integration test coverage gaps.
- **Knowledge Base Filter:** `category in ["architecture", "test"]` (e.g., `architecture-order-repo.md`).
- **Output:** Concrete code remediation proposals, PR guidance, and test recommendations.

---

### 🛠️ 2. Principal SRE & Platform Engineer (`internal/role/sre_agent.go`)
- **Primary Focus:** Infrastructure health, container orchestration, connection pool saturation, and dependency resilience.
- **Investigation Targets:**
  - HikariCP / database pool starvation (active connections vs max connections).
  - Outbound latency and 3rd-party upstream timeouts (HTTP 504 / gateway failures).
  - Kubernetes pod crash loops and memory limits.
- **Knowledge Base Filter:** `category in ["infrastructure", "runbook"]` (e.g., `infra-runbook-db.md`).
- **Output:** Operational runbook commands, rolling restart procedures, circuit breaker settings, and config tuning.

---

### 👔 3. Product & Business Operations Lead (`internal/role/product_lead_agent.go`)
- **Primary Focus:** Customer experience, SLA compliance, revenue risk, and business fallback policies.
- **Investigation Targets:**
  - Estimated GMV revenue loss (e.g., Average Order Value [AOV] $\times$ failed checkout count).
  - Breach of SLA / SLO targets (e.g., 99.9% order success rate).
  - Customer retention and cart abandonment risks.
- **Knowledge Base Filter:** `category in ["business_policy"]` (e.g., `business-campaign-rules.md`).
- **Output:** Activation of Degraded Mode (ADR-005 asynchronous queuing), customer messaging, and marketing campaign pauses.

---

### 👑 4. Incident Commander Agent (`internal/role/commander_agent.go`)
- **Primary Focus:** Arbitration, cross-functional prioritization, and decisive consensus.
- **Investigation Targets:**
  - Synthesizing conflicting recommendations (e.g., SRE wanting immediate pod restarts vs Tech Lead needing runtime memory dumps).
  - Assessing true incident severity (`CRITICAL`, `HIGH`, `MEDIUM`).
  - Structuring a chronological, actionable remediation runbook.
- **Output:**
  1. **Immediate Workaround (< 5 mins):** Rolling restart, scaling pool, switching to degraded mode.
  2. **Short-term Remediation (< 2 hours):** Hotfix deployment, config map tuning.
  3. **Permanent Prevention (< 2 sprints):** Circuit breaker implementation, transaction refactoring, integration tests.

---

## 4. Per-Role Token Usage & Cost Transparency

In a production environment, visibility into AI inference costs is essential. Every role independently tracks token usage using Gemini's `UsageMetadata`:

```go
type TokenUsage struct {
    PromptTokens     int `json:"prompt_tokens"`
    CandidatesTokens int `json:"candidates_tokens"`
    TotalTokens      int `json:"total_tokens"`
}
```

### Discord Embed Integration
The Discord webhook card renders individual token metrics for each role and a combined total:

```text
🧑‍💻 Software Tech Lead & Code Architect
Assessment: ...
Proposed Action: ...
(Tokens: in:1240, out:310, total:1550)

🛠️ Principal SRE & Platform Engineer
Assessment: ...
Proposed Action: ...
(Tokens: in:1190, out:280, total:1470)

👔 Product & Business Lead
Assessment: ...
Proposed Action: ...
(Tokens: in:980, out:210, total:1190)

👑 Incident Commander
Assessment: ...
(Tokens: in:1640, out:346, total:1986)

----------------------------------------------------
💰 Token Consumption Tracker:
Prompt/Input : 5,050 tokens
Output/Reason: 1,146 tokens
Total Count  : 6,196 tokens
----------------------------------------------------
```

---

## 5. Extensibility: Adding a New Role

To add a new role (e.g., `SecurityLeadAgent` or `ComplianceAgent`):
1. Create a new file in `internal/role/security_lead_agent.go`.
2. Implement the `ExpertAgent` interface (`Name()`, `RoleTitle()`, and `Analyze()`).
3. Add a specialized markdown knowledge file under `docs/knowledge/security-policies.md` with `Target Role: security`.
4. Register the new role inside `RunModularWarRoom` in `internal/analyzer/modular_warroom.go`.
