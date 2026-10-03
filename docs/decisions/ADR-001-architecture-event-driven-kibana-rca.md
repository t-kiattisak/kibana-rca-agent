# ADR-001: Event-Driven Kibana Alert Webhook with On-Demand Correlated Log Retrieval

## Status
Accepted

## Date
2026-10-03

## Context
When monitoring applications using Elasticsearch and Kibana, logging volume is typically very high (gigabytes to terabytes per day). Directly streaming all raw logs into a Large Language Model (LLM) is cost-prohibitive, introduces severe latency, and risks token exhaustion.

However, incident responders need contextual information (e.g., logs preceding the incident, related trace spans, error distribution across instances) to determine the root cause, rather than just inspecting an isolated error alert line.

Key requirements:
- Cost efficiency: Only trigger LLM calls when an anomaly or incident threshold is met.
- Actionable Context: The LLM must receive correlated logs across the incident time window ($T - 5\text{m}$ to $T + 1\text{m}$) and grouped by `trace_id` or service.
- Low operational complexity: Use native Kibana and Elasticsearch alerting capabilities.

## Decision
Adopt an **Event-Driven, On-Demand Correlated Retrieval** architecture:
1. **Kibana Alerting / Watcher** evaluates error rules (e.g., error rate spike > threshold in 1 minute).
2. Kibana fires an HTTP Webhook containing basic alert metadata (`rule_name`, `timestamp`, `service_name`, initial matching error sample).
3. The **AI RCA Agent** receives the webhook and queries Elasticsearch via the Elasticsearch REST API to gather correlated logs around the time window and matching trace IDs.
4. The RCA Agent sanitizes logs (masks PII / credentials), formats the context into an incident prompt, and passes it to the LLM.
5. Actionable RCA and remediation suggestions are delivered via chat webhook (Slack/Teams/Console).

```text
[Kibana Alert Rule / Threshold]
           │ Webhook (timestamp, service, alert context)
           ▼
[AI RCA Agent Service]
           │ Query Correlated Logs ($T - 5m$ to $T + 1m$, trace_id)
           ▼
[Elasticsearch API] ──(Logs returned)──> [Agent Sanitizer]
                                                │
                                                ▼
                                        [LLM Analysis (Gemini)]
                                                │
                                                ▼
                                    [Actionable Incident Report]
```

## Alternatives Considered

### 1. Continuous Log Stream Ingestion to LLM (Logstash / Vector directly to LLM)
- **Pros:** Real-time stream processing, can catch subtle non-threshold anomalies.
- **Cons:** Astronomical API token costs, high latency, huge context window constraints, rate-limit throttling.
- **Rejected:** Economically and technically impractical for production log volumes.

### 2. Alert-Only Payload Analysis (LLM analyzes only the single log attached to Kibana Webhook)
- **Pros:** Simplest implementation, no need for the Agent to call Elasticsearch back.
- **Cons:** Missing upstream/downstream causal logs, previous warning logs, and trace spans. LLM frequently hallucinates or produces generic advice without preceding context.
- **Rejected:** Fails the primary goal of providing high-confidence Root Cause Analysis.

## Consequences
- **Positive:**
  - Token costs are reduced by over 99% compared to streaming models.
  - LLM receives rich, correlated context ($T - 5\text{m}$ window, trace traces), leading to high-quality root cause identification.
  - Decoupled: Kibana handles log aggregation & threshold detection, while the Agent focuses purely on investigation & synthesis.
- **Negative / Trade-offs:**
  - The AI RCA Agent requires read credentials and network access back to the Elasticsearch cluster.
  - Alert time-to-deliver includes the Elasticsearch query latency (~100-300ms) + LLM inference latency (~1-3s).
