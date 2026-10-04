# Kibana AI Root Cause Analysis (RCA) Agent

AI-powered automated Incident Triage and Root Cause Analysis for systems monitored with Elasticsearch and Kibana, powered by **LangGraph JS (`@langchain/langgraph`)** in TypeScript with **Google Gemini** and **Elasticsearch v8 Dense Vector (kNN)**.

## 📖 Architecture & Decisions (ADRs)
This project follows the [Architecture Decision Record (ADR)](https://github.com/addyosmani/agent-skills/blob/main/skills/documentation-and-adrs/SKILL.md) standard to document the *why* behind technical decisions:

- **[ADR-001: Event-Driven Kibana Alert Webhook with On-Demand Correlated Retrieval](docs/decisions/ADR-001-architecture-event-driven-kibana-rca.md)**  
  Why we trigger RCA via alert webhooks instead of streaming all logs to LLM continuously.
- **[ADR-002: Technology Stack Evolution](docs/decisions/ADR-002-technology-stack-golang.md)**  
  Evolution of the technology stack from initial prototypes to production-grade agents.
- **[ADR-003: LLM Integration and Structured Prompt Pipeline](docs/decisions/ADR-003-llm-integration-and-prompting.md)**  
  Choice of Google Gemini, deterministic pipeline, data sanitization, and structured JSON outputs.
- **[ADR-004: Event Loop Processing & Vector Semantic Memory (kNN RAG)](docs/decisions/ADR-004-event-loop-and-vector-memory.md)**  
  Alert debouncing/deduplication and native Elasticsearch v8 kNN vector search for institutional memory.
- **[ADR-005: Multi-Agent War Room Pattern & Role-Based Knowledge Context](docs/decisions/ADR-005-multi-agent-war-room-architecture.md)**  
  Decoupling triage into specialized roles (Tech Lead, SRE, Product Lead, Commander) with domain document citations and per-role token usage tracking.
- **[ADR-006: Knowledge Document Loader, Vector kNN Chunking & On-Premise Strategy](docs/decisions/ADR-006-document-loader-knn-and-local-strategy.md)**  
  Semantic document chunking by markdown headings, Elasticsearch `docs-knowledge-knn` 768 dims, and local Ollama alternatives.
- **[ADR-007: Migration to LangGraph JS for Cyclic Multi-Agent Loops](docs/decisions/ADR-007-migration-to-langgraph-js.md)**  
  Adopting `@langchain/langgraph` in TypeScript for stateful cyclic agentic graphs and deep dive investigation loops.
- **[Multi-Agent War Room Architecture Guide](docs/architecture/multi-agent-war-room.md)**  
  Detailed multi-role personas, guardrails, response schemas, and token metrics.
- **[System Architecture & Workflow Overview](docs/architecture/system-overview.md)**  
  Complete system overview, Mermaid sequence diagrams, and webhook/RCA data contracts.

---

## 🏗️ Project Layout

```text
.
├── agent/                   # LangGraph JS Multi-Agent Orchestrator (TypeScript)
│   ├── src/
│   │   ├── state.ts         # WarRoomStateAnnotation & channel reducers
│   │   ├── graph.ts         # StateGraph (Fan-out, Fan-in, Cyclic Edge)
│   │   ├── geminiClient.ts  # Gemini API with exponential backoff & embeddings
│   │   ├── index.ts         # Main log watcher & LangGraph invocation loop
│   │   └── nodes/           # Tech Lead, SRE Lead, Product Lead, Commander, Discord Notifier
│   ├── package.json
│   └── tsconfig.json
├── docs/
│   ├── architecture/        # System diagrams & Multi-Agent War Room guide
│   ├── decisions/           # ADR-001 through ADR-007
│   └── knowledge/           # Architecture specs, runbooks & business policies
├── cmd/
│   └── app/                 # Go Demo Web API with simulated failure scenarios
├── testdata/
│   ├── sample_kibana_alert.json
│   └── trigger_errors.sh    # Incident simulator script
├── docker-compose.yml       # Elasticsearch 8.17.0 + Kibana 8.17.0
└── README.md
```
