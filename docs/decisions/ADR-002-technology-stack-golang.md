# ADR-002: Use Go (Golang) for RCA Agent and Demo Service

## Status
Accepted

## Date
2026-10-03

## Context
The AI RCA Agent system consists of:
1. A **Demo / Sample Service** that generates traffic and emits structured error logs.
2. An **RCA Agent Service** that serves an HTTP webhook endpoint, communicates with Elasticsearch, calls LLM APIs, masks sensitive data, and delivers incident notifications.

Elasticsearch and Kibana already consume significant host memory (often 2GB to 4GB+ in local/staging environments). We needed to select a programming language and runtime for the Agent and Demo application.

Key evaluation criteria:
- Memory footprint and resource overhead.
- Execution speed and concurrency handling (simultaneous incident alerts, multiple ES queries).
- Production-readiness (single static binary deployment, container image size).
- Ecosystem and official SDK support (Elasticsearch, LLM clients).

## Decision
Use **Go (Golang)** as the primary implementation language for both the `rca-agent` and the `demo-app`.

## Alternatives Considered

### 1. Python (FastAPI + LangChain / LlamaIndex)
- **Pros:** Vast ecosystem for LLM experimentation, quick prototyping.
- **Cons:**
  - High baseline RAM consumption (~100-200MB+ per container vs ~15-25MB in Go).
  - Heavy image size (Python base images typically 300MB - 1GB vs ~20MB scratch/alpine in Go).
  - Heavyweight abstractions like LangChain add unnecessary overhead for deterministic prompt/retrieval pipelines.
- **Rejected:** Excessive resource consumption when co-located with Elasticsearch/Kibana, and dependency sprawl.

### 2. Node.js / TypeScript (Express / Fastify)
- **Pros:** Good async I/O, familiar web server model.
- **Cons:** Moderate memory footprint (Node runtime ~60-120MB), lacks the single static binary simplicity and compile-time concurrency guarantees of Go.
- **Rejected:** Go offers superior performance, smaller memory footprint, and cleaner static typing for Elasticsearch/LLM data structures.

## Consequences
- **Positive:**
  - **Ultra-low memory footprint:** Go processes run comfortably within 15–35MB RAM, preserving host resources for the Elastic stack.
  - **High Concurrency:** Goroutines effortlessly handle parallel Elasticsearch log extraction and concurrent alert processing.
  - **Minimal Docker Images:** Multi-stage builds produce tiny production container images (~15–25MB).
  - **Official SDK Support:** Utilizes official Elastic Go client (`github.com/elastic/go-elasticsearch/v8`) and Google GenAI Go SDK (`google.golang.org/genai`).
- **Negative / Trade-offs:**
  - Slightly more verbose boilerplate for JSON parsing and HTTP client handling compared to dynamic Python scripts, mitigated by Go's strong type safety and explicit error handling.
