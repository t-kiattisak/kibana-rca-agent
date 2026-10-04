# ADR-002: System Runtime Stack — TypeScript (LangGraph) for Agent & Go for Target App

## Status
Accepted

## Date
2026-10-04

## Context
The AI RCA Agent system consists of two primary operational components:
1. **Target Demo Application (`cmd/app`):** Generates high-throughput HTTP traffic and emits structured error logs into Elasticsearch to simulate production incidents.
2. **AI RCA Agent (`agent/`):** Listens to incident spikes, retrieves correlated context and vector embeddings from Elasticsearch, coordinates multi-role expert agents, runs verification loops, and delivers actionable incident cards to Discord.

We needed to select the optimal technology runtime for each tier based on their distinct operational characteristics.

---

## Decision

We adopt a decoupled two-tier architecture:

### 1. Agent Orchestrator: Node.js / TypeScript with LangGraph JS (`agent/`)
- **Runtime:** Node.js (TypeScript)
- **Framework:** `@langchain/langgraph`
- **Rationale:**
  - **Native Cyclic Graph & State Management:** LangGraph JS provides first-class support for stateful cyclic graphs (`StateGraph`), enabling multi-agent debate and iterative deep-dive verification loops.
  - **Typed Functional Annotations:** `Annotation.Root` provides deterministic channel reducers and concurrent fan-out/fan-in coordination.
  - **Official SDK Support:** Direct access to `@google/genai` and `@elastic/elasticsearch`.

### 2. Target Demo Application: Go (`cmd/app`)
- **Runtime:** Go (Golang)
- **Rationale:**
  - Lightweight web service with near-zero memory footprint (~15MB RAM).
  - High concurrency handling to simulate realistic thread pool exhaustion and HTTP gateway timeouts.
  - Generates realistic stack traces and Goroutine panics for analysis.

---

## Consequences

### Positive
- **Optimal Tool for the Job:** LangGraph JS handles complex multi-agent state machines effortlessly, while Go provides high performance for the simulated service.
- **Maintainability:** Clear separation between the monitored service and the AI triage engine.
- **Extensibility:** New agent roles or tools can be added to LangGraph in TypeScript without recompiling backend binaries.

### Negative
- Requires maintaining two runtimes: Go for the demo container and Node.js for the AI agent.
