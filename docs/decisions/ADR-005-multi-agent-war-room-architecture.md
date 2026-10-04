# ADR-005: Multi-Agent War Room Pattern & Role-Based Knowledge Context

## Status
Accepted

## Date
2026-10-04

## Context
Production incidents are rarely isolated technical glitches; they intersect multiple organizational disciplines:
1. **Code & Architecture:** Stack traces, missing transaction timeouts, test suite gaps.
2. **Infrastructure & Platform (SRE):** Connection pool limits, container health, database server CPU/memory, network timeouts.
3. **Product & Business Operations:** SLA thresholds, campaign revenue impact ($ GMV losses), and fallback degraded modes.

A single generic prompt asking an LLM for an RCA produces generic advice. Moreover, large enterprise documentation bases (ADRs, Runbooks, Business Specs) cannot be dumped wholesale into a single prompt without risking token budget exhaustion and dilution of critical domain context.

## Decision
We decouple the incident investigation into a **Multi-Agent War Room** composed of independent role-based agents, coordinated by an Incident Commander:

```text
[Incident Trigger: Elasticsearch Correlated Logs]
                       │
       ┌───────────────┼───────────────┐
       ▼               ▼               ▼
[Tech Lead Agent] [SRE Lead Agent] [Product Lead Agent]
(architecture docs) (infra runbooks)  (business policies)
       │               │               │
       └───────────────┬───────────────┘
                       ▼
          [Incident Commander Agent]
                       │
         (Synthesizes Consensus & Plan)
                       │
                       ▼
       [Discord Rich Embed Dashboard]
         (With Per-Role Token Usage)
```

### 1. LangGraph StateGraph Architecture (`agent/src/`)
Each expert role is modeled as a functional node inside the LangGraph StateGraph (`agent/src/graph.ts`):
- **`techLeadNode` (`agent/src/nodes/techLeadNode.ts`):** Reads stack traces, identifies coding patterns, spots unit test gaps, and suggests code hotfixes. Filtered context: `target_role: "developer"`.
- **`sreLeadNode` (`agent/src/nodes/sreLeadNode.ts`):** Inspects HikariCP pool saturation, container restarts, and database metrics. Filtered context: `target_role: "sre"`.
- **`productLeadNode` (`agent/src/nodes/productLeadNode.ts`):** Assesses business SLA impact, GMV conversion drop, and invokes business fallback policies (e.g. Asynchronous intake). Filtered context: `target_role: "product"`.
- **`commanderNode` (`agent/src/nodes/commanderNode.ts`):** Reviews all expert testimonies, resolves priority conflicts, produces the unified 3-stage Action Plan, and evaluates whether a deep-dive cyclic loop is required.

### 2. Token Consumption & Cost Tracker
Each agent node extracts token metadata from the Gemini API response (`promptTokenCount`, `candidatesTokenCount`, `totalTokenCount`). 
The StateGraph accumulates token metrics per role and reports a detailed breakdown in the Discord Embed and Console:
- Per-role token consumption.
- Total incident triage token cost.

## Consequences
- **Positive:**
  - **Domain-Specific Precision:** Tech Lead does not hallucinate infra fixes, and SRE does not rewrite application business logic.
  - **Direct Document Citations:** Each role explicitly references relevant organizational docs (e.g., `architecture-order-repo.md`, `infra-runbook-db.md`).
  - **Financial Transparency:** Token tracking enables SRE leadership to track incident response AI costs per incident.
  - **Modularity:** Adding a new role (e.g., Security Lead, QA Lead, Compliance Auditor) simply requires implementing the `ExpertAgent` interface without modifying existing agents.
- **Trade-offs:**
  - Invoking multiple specialized agents generates 4 distinct LLM calls (3 experts + 1 commander), which increases total token usage and latency compared to a single-shot prompt (~4–8s total vs ~2s).
