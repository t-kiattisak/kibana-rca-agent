# ADR-004: Event Loop Processing & Vector Semantic Memory (kNN RAG)

## Status
Accepted

## Date
2026-10-03

## Context
In ADR-001 through ADR-003, we established a synchronous webhook receiver architecture where Kibana triggers the RCA Agent, which queries correlated logs and invokes the LLM.

However, in realistic high-traffic environments and severe production incidents, this initial design presents two critical architectural limitations:

1. **Alert Storms & Synchronous Bottlenecks:**
   - A single root cause (e.g., database network partition) can trigger dozens of simultaneous alert notifications within seconds.
   - Synchronous HTTP webhook handlers will block waiting on LLM inference (2–4 seconds per call), leading to connection timeouts, excessive resource spikes, and LLM rate-limit exhaustion.
   - Duplicate alerts for the same underlying failure cause duplicate LLM token consumption and spam incident chat rooms.

2. **Absence of Historical Context & Institutional Knowledge:**
   - Stateless analysis treats every incident as if it were brand new.
   - Engineering teams frequently encounter recurring issues or patterns with established internal runbooks and historical post-mortems.
   - Without long-term semantic memory, the LLM cannot reference past team decisions, resolution steps, or known architectural workarounds.

## Decision

We adopt two foundational patterns:

### 1. Asynchronous Ingestion with an In-Memory Event Loop & Worker Pool
- **Immediate Ingestion:** The HTTP webhook handler immediately validates the alert payload, pushes the event onto a buffered Go channel (`chan AlertEvent`), and returns `HTTP 202 Accepted` to Kibana within milliseconds (< 10ms).
- **Debouncing & Deduplication:** An in-process Event Loop aggregates incoming alerts over a sliding debounce window (e.g., 30 seconds). Alerts matching the same incident fingerprint (`hash(service, alert_rule, error_signature)`) are batched into a single unified incident task.
- **Worker Pool:** A pool of concurrent worker Goroutines consumes deduplicated incident tasks, enforcing concurrency boundaries and LLM rate-limiting.

### 2. Long-Term Semantic Memory using Elasticsearch v8 Vector Search (kNN / Dense Vector)
- **Leveraging Existing Infrastructure:** Rather than introducing a separate dedicated vector database (like Pinecone, Milvus, or Qdrant), we utilize Elasticsearch v8's native Dense Vector and approximate kNN search capabilities (`index: incident-memory-knn`).
- **Embedding Generation:** Incident error signatures, stack traces, and human post-mortems are converted into dense vector embeddings using text-embedding models (e.g., Google `text-embedding-004` or Gemini Embeddings).
- **RAG Retrieval:** During analysis, the worker executes a hybrid search:
  1. Correlated recent logs from the log indices (`logs-app-*`).
  2. The top $K$ most similar historical incidents or runbooks from `incident-memory-knn`.
- **Knowledge Augmentation:** Retrieved past resolutions are injected into the LLM prompt as contextual few-shot examples, producing organization-specific, highly accurate remediation steps.
- **Feedback Loop / Memory Write-back:** When an incident is resolved and verified, its summary and root cause can be indexed back into the vector store for future incidents.

```text
[Kibana Alert]
       │ (HTTP POST Webhook)
       ▼
[Webhook Receiver] ──(HTTP 202 Accepted in <10ms)──> [Kibana]
       │
       ▼ (push)
[Buffered Go Channel Queue]
       │
       ▼ (read & debounce)
[Event Loop / Deduplicator]
       │ (dispatch unique incident)
       ▼
[Worker Pool (Goroutines)]
       ├── 1. Query Correlated Logs (logs-app-*)
       ├── 2. Generate Embedding & Query Similar Incidents (incident-memory-knn kNN search)
       ├── 3. Sanitize PII & Secrets
       ├── 4. Invoke LLM with Context + Past Solutions (Gemini)
       └── 5. Dispatch Actionable Report (Slack / Teams)
```

## Alternatives Considered

### 1. Standalone External Queue (Kafka / RabbitMQ / Redis Streams) for Phase 1
- **Pros:** Distributed durability across multiple nodes, survive service restart.
- **Cons:** Significantly increases deployment footprint and operational overhead for the demo and initial versions.
- **Decision:** Use native Go channels and Goroutine worker pools for the embedded event loop. The interface will be decoupled so an external queue (e.g., Redis Stream) can be swapped in if horizontal scaling is required in the future.

### 2. Standalone Vector Database (Pinecone / Chroma / Weaviate)
- **Pros:** Specialized vector features.
- **Cons:** Elasticsearch v8 already provides high-performance HNSW kNN vector search natively. Introducing a separate vector database creates redundant infrastructure and complicates Docker Compose setup.
- **Decision:** Keep storage unified inside Elasticsearch v8.

## Consequences
- **Positive:**
  - **Resilient to Alert Storms:** Webhook ingestion never blocks; duplicate alerts are automatically debounced.
  - **Smarter Recommendations:** RCA recommendations leverage previous team post-mortems and internal runbooks.
  - **Unified Infrastructure:** Elasticsearch handles both raw log storage and vector semantic memory without needing extra databases.
- **Negative / Trade-offs:**
  - In-flight alerts in an in-memory Go channel buffer could be lost if the agent crashes during a restart (mitigated by keep-alive and graceful shutdown, or an external queue in a distributed setup).
  - Requires generating embeddings before vector queries, adding an embedding API roundtrip (~50-150ms).
