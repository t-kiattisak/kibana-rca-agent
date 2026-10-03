# Kibana AI Root Cause Analysis (RCA) Agent

AI-powered automated Incident Triage and Root Cause Analysis for systems monitored with Elasticsearch and Kibana, built with **Go (Golang)** and **Google Gemini**.

## 📖 Architecture & Decisions (ADRs)
This project follows the [Architecture Decision Record (ADR)](https://github.com/addyosmani/agent-skills/blob/main/skills/documentation-and-adrs/SKILL.md) standard to document the *why* behind technical decisions:

- **[ADR-001: Event-Driven Kibana Alert Webhook with On-Demand Correlated Retrieval](docs/decisions/ADR-001-architecture-event-driven-kibana-rca.md)**  
  Why we trigger RCA via Kibana alert webhooks instead of streaming all logs to LLM continuously.
- **[ADR-002: Technology Stack - Go (Golang)](docs/decisions/ADR-002-technology-stack-golang.md)**  
  Why Go was chosen over Python/FastAPI for low memory footprint, high concurrency, and production-grade binaries.
- **[ADR-003: LLM Integration and Structured Prompt Pipeline](docs/decisions/ADR-003-llm-integration-and-prompting.md)**  
  Choice of Google Gemini, deterministic pipeline, data sanitization, and structured JSON outputs.
- **[ADR-004: Event Loop Processing & Vector Semantic Memory (kNN RAG)](docs/decisions/ADR-004-event-loop-and-vector-memory.md)**  
  Asynchronous ingestion via Go buffered channel, alert debouncing/deduplication, and native Elasticsearch v8 kNN vector search for institutional memory.
- **[System Architecture & Workflow Overview](docs/architecture/system-overview.md)**  
  Complete system overview, Mermaid sequence diagrams, and webhook/RCA data contracts.

---

## 🏗️ Project Layout

```text
.
├── docs/
│   ├── architecture/
│   │   └── system-overview.md
│   └── decisions/
│       ├── ADR-001-architecture-event-driven-kibana-rca.md
│       ├── ADR-002-technology-stack-golang.md
│       └── ADR-003-llm-integration-and-prompting.md
├── cmd/
│   ├── demo-app/            # Mock Web API with error-injection triggers
│   └── rca-agent/           # Go AI RCA Webhook & Analysis Engine
├── internal/
│   ├── analyzer/            # Gemini LLM integration & RCA Prompts
│   ├── config/              # Configuration loader
│   ├── esclient/            # Correlated log fetcher via Elasticsearch API
│   ├── model/               # Alert & RCA data contracts
│   ├── notifier/            # Slack / Console alert dispatchers
│   └── sanitizer/           # PII and credentials masking
├── testdata/
│   ├── sample_kibana_alert.json
│   └── trigger_errors.sh
├── docker-compose.yml
├── Makefile
└── README.md
```
