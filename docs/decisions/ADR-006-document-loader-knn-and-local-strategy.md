# ADR-006: Knowledge Document Loader, Vector kNN Chunking & On-Premise Local Strategy

## Status
Accepted

## Date
2026-10-04

## Context
In ADR-005, we implemented the Multi-Agent War Room pattern where four specialized agents (Tech Lead, SRE Lead, Product Lead, and Incident Commander) collaborate to analyze incidents. 

However, during practical deployment and testing, two critical operational challenges emerged:

1. **Token Bloat & Rate Limit Exhaustion (Free Tier / High Demand):**
   - Injecting entire markdown knowledge documents (`architecture-order-repo.md`, `infra-runbook-db.md`, `business-campaign-rules.md`) directly into LLM prompts consumed upwards of 10,000–12,000 tokens per incident.
   - In cloud-based free tiers (e.g., Google AI Studio Gemini API), strict rate limits exist (e.g., 20 Requests Per Day [RPD], 5 Requests Per Minute [RPM]). A single 4-agent War Room run rapidly triggers `HTTP 429 Resource Exhausted` or `HTTP 503 High Demand` errors.

2. **Enterprise Data Privacy & Offline Autonomy Requirements:**
   - Organizations frequently manage proprietary runbooks, internal architectural secrets, and confidential SLAs that cannot be transmitted over public internet endpoints.
   - Relying solely on external Cloud APIs introduces latency and external dependencies during critical network infrastructure outages.

---

## Decision

We establish a comprehensive two-tier architecture: **Smart Semantic Document Chunking with Role-Filtered Vector kNN** in Elasticsearch, complemented by a **Zero-Cloud Local Deployment Strategy (Ollama)**.

### 1. Document Chunking & Elasticsearch Vector Index (`docs-knowledge-knn`)
- **Semantic Chunking by Heading:** Rather than arbitrary token slicing, documents in `docs/knowledge/` are split semantically by Markdown headings (`## `). Each chunk retains its metadata:
  - `chunk_id`: E.g., `infra-runbook-db.md-symptoms-and-triage`
  - `doc_title`: Origin document filename
  - `target_role`: E.g., `developer`, `sre`, `product`
  - `heading` & `content`: Section-specific markdown text
  - `embedding`: 768-dimensional dense vector
- **Elasticsearch Native Index Mapping:**
  ```json
  {
    "mappings": {
      "properties": {
        "chunk_id": { "type": "keyword" },
        "doc_title": { "type": "keyword" },
        "category": { "type": "keyword" },
        "target_role": { "type": "keyword" },
        "heading": { "type": "keyword" },
        "content": { "type": "text" },
        "embedding": {
          "type": "dense_vector",
          "dims": 768,
          "index": true,
          "similarity": "cosine",
          "index_options": {
            "type": "int8_hnsw",
            "m": 16,
            "ef_construction": 100
          }
        }
      }
    }
  }
  ```

### 2. Role-Filtered Vector kNN Search
When an incident is detected:
1. The error signatures and service context are converted to a query vector.
2. Each agent executes a role-filtered vector search with $k=1$ (Top-1 most relevant chunk):
   ```json
   {
     "knn": {
       "field": "embedding",
       "query_vector": [...],
       "k": 1,
       "num_candidates": 20,
       "filter": {
         "term": { "target_role": "sre" }
       }
     }
   }
   ```
3. **Outcome:** Each agent receives *only* the specific runbook or architectural section directly relevant to the failure, cutting prompt token consumption down to minimal bounds (~1,500–2,500 tokens).

### 3. Cloud vs. Local Strategy Comparison

To mitigate quota exhaustion and support air-gapped on-premise environments, the architecture supports both Cloud and Local engines:

| Dimension | Option A: Gemini Cloud (Current) | Option B: Ollama Local (On-Premise) |
| :--- | :--- | :--- |
| **Embedding Engine** | `gemini-embedding-001` (768 dims) | `nomic-embed-text` (768 dims) via Ollama |
| **LLM Inference** | `gemini-3.5-flash` / `gemini-3.8-flash` | `qwen2.5:7b` / `llama3.2:3b` via Ollama |
| **API Endpoint** | `generativelanguage.googleapis.com` | `http://localhost:11434/v1` (OpenAI-compatible) |
| **Daily Quota (RPD)** | Limited (20 RPD on Free Tier) | **Unlimited (No Rate Limits)** |
| **Cost** | Free Tier or Pay-per-token | **$0 (Uses local hardware)** |
| **Data Privacy** | Payloads sent to Google AI Studio | **100% In-house / Air-gapped compliance** |
| **Network Dependency**| Requires outbound internet connection | **Runs 100% offline inside internal VPC** |

---

## Architecture Diagram

```text
       ┌──────────────────────────────────────────────────────────────┐
       │                Kibana / Elasticsearch Logs                   │
       └──────────────────────────────┬───────────────────────────────┘
                                      │ Polling / Event Spike
                                      ▼
                        ┌───────────────────────────┐
                        │       RCA Watcher         │
                        └─────────────┬─────────────┘
                                      │ Trigger Incident
                                      ▼
                 ┌─────────────────────────────────────────┐
                 │    Role-Filtered kNN Document Store     │
                 │      (ES: docs-knowledge-knn)           │
                 └───────┬────────────┬────────────┬───────┘
          Query k=1      │            │            │
      for "developer"    │   for "sre"│   for "product"
                         ▼            ▼            ▼
                    ┌─────────┐  ┌─────────┐  ┌─────────┐
                    │Tech Lead│  │SRE Lead │  │ Product │
                    │  Agent  │  │  Agent  │  │  Lead   │
                    └────┬────┘  └────┬────┘  └────┬────┘
                         │            │            │
                         └────────────┼────────────┘
                                      ▼
                       ┌─────────────────────────────┐
                       │ Incident Commander Synthesis │
                       └──────────────┬──────────────┘
                                      │
                     ┌────────────────┴────────────────┐
                     ▼                                 ▼
         ┌─────────────────────────┐      ┌─────────────────────────┐
         │ Discord Webhook Notifier │      │ STDOUT Terminal Console │
         │   (Rich Embeds Cards)   │      │ (Token Breakdown Table) │
         └─────────────────────────┘      └─────────────────────────┘
```

---

## Consequences

### Positive
- **Dramatic Token Reduction:** Token usage per agent is reduced from entire files (~10,000 tokens) to focused sections (~1,500 tokens).
- **Zero Additional Database Overhead:** Native Elasticsearch v8 vector search means no secondary vector databases (Pinecone, Qdrant) are needed.
- **Local Fallback Ready:** Seamless path to switch to local Ollama embeddings (`nomic-embed-text`) with identical 768 dimensions without altering index mappings.
- **Role Isolation:** SRE agents never see irrelevant marketing policy text, and Product agents never process raw database connection pool sizing configs.

### Negative & Mitigations
- **Free Tier Cloud Bottlenecks:** When using Google Free Tier, rapid testing triggers `429 Quota Exceeded` (20 RPD).
  - *Mitigation:* Implement debouncing windows (3 minutes) and provide seamless local execution options.
- **Discord Payload Size Limits:** Rich War Room debates with 4 role perspectives can exceed Discord's 1024-character embed field limit.
  - *Mitigation:* Added string truncation (`truncate(text, 1020)`) in `warroom_notifier.go`.
