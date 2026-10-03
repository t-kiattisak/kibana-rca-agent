# ADR-003: LLM Integration Strategy and Structured Prompt Pipeline

## Status
Accepted

## Date
2026-10-03

## Context
The AI RCA Agent must inspect log streams, trace spans, error counts, and incident metadata to generate structured Root Cause Analysis (RCA) and mitigation runbooks.

We need to decide:
1. Which LLM provider and SDK approach to use.
2. Whether to use an Autonomous Multi-turn Agent framework (e.g., ReAct / LangGraph) or a Deterministic Multi-stage Pipeline (Correlated Query -> Sanitization -> LLM Analysis).
3. The schema and structure of the LLM output to guarantee consistent notifications.

## Decision
1. **Model Provider:** Use **Google Gemini (Gemini 2.5/Flash)** via the official Go SDK (`google.golang.org/genai`), taking advantage of its large context window, fast time-to-first-token (TTFT), and cost-effective inference.
2. **Architecture:** Use a **Deterministic Prompt Pipeline** with JSON Structured Output:
   - Step 1: Webhook Parser extracts alert criteria and timestamps.
   - Step 2: Correlated Log Query extracts log lines within the incident window ($T - 5\text{m}$ to $T + 1\text{m}$).
   - Step 3: Sanitizer cleanses secrets/PII (passwords, auth tokens, emails, card numbers).
   - Step 4: System Prompt enforces an SRE/Incident Response Persona with structured JSON schema.
3. **Structured Response Schema:** Enforce response contract:
   - `severity`: "CRITICAL" | "HIGH" | "MEDIUM" | "LOW"
   - `incident_summary`: Concise summary of what happened.
   - `probable_root_cause`: Detailed evidence-backed explanation.
   - `affected_components`: List of affected services/endpoints.
   - `recommended_actions`: Categorized into Immediate Workaround, Configuration Change, and Permanent Bug Fix.

## Alternatives Considered

### 1. Fully Autonomous ReAct Agent (LLM calls Elasticsearch dynamically via Function Calling)
- **Pros:** LLM can decide which indices or specific queries to run adaptively.
- **Cons:**
  - High latency (multiple roundtrips of tool calls add 5–15 seconds to incident reporting).
  - Unpredictable token usage and cost variance.
  - Risk of recursive tool loops or inefficient wildcard queries hitting Elasticsearch under production load.
- **Rejected:** Incidents require immediate, sub-5-second alerts. A deterministic correlation fetcher provides 95% of required context in a single roundtrip.

### 2. Unstructured Free-Text Output
- **Pros:** Simpler prompt design.
- **Cons:** Chat webhook formatters (Slack Blocks, Teams Adaptive Cards) break easily when output format is unpredictable.
- **Rejected:** Operational response demands predictable, structured fields.

## Consequences
- **Positive:**
  - Fast, predictable latency (< 3-5 seconds end-to-end).
  - Reliable parsing into rich Slack / Microsoft Teams card UI.
  - High reproducibility and straightforward unit testing of prompt and extraction components.
- **Negative / Trade-offs:**
  - If an incident spans multiple disparate services that are not captured in the default correlation query, the agent will only see the predetermined window of logs. (Can be extended in future ADRs to add secondary hops).
