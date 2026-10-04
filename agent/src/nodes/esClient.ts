import { Client } from "@elastic/elasticsearch";
import * as dotenv from "dotenv";
import { generateEmbedding768 } from "../geminiClient";
import { KnowledgeChunk, LogEntry, IncidentContext } from "../state";

dotenv.config({ path: "../.env" });

const esUrl = process.env.ELASTICSEARCH_URL || "http://localhost:9200";
const esIndex = process.env.ELASTICSEARCH_INDEX || "logs-app-*";

export const es = new Client({ node: esUrl });

export async function searchKNNChunks(
  queryText: string,
  targetRole: string,
  k: number = 1
): Promise<KnowledgeChunk[]> {
  try {
    const vector = await generateEmbedding768(queryText);
    const response = await es.search({
      index: "docs-knowledge-knn",
      knn: {
        field: "embedding",
        query_vector: vector,
        k,
        num_candidates: 20,
        filter: {
          term: {
            target_role: targetRole,
          },
        },
      },
    });

    const hits = response.hits.hits || [];
    return hits.map((hit: any) => ({
      chunk_id: hit._source.chunk_id,
      doc_title: hit._source.doc_title,
      category: hit._source.category,
      target_role: hit._source.target_role,
      heading: hit._source.heading,
      content: hit._source.content,
    }));
  } catch (err) {
    console.warn(`[kNN Search Fallback for ${targetRole}]:`, err);
    return [];
  }
}

export async function checkErrorSpike(
  timeWindowMin: number = 1,
  threshold: number = 3
): Promise<{ triggered: boolean; count: number }> {
  const gteTime = `now-${timeWindowMin}m`;

  const response = await es.count({
    index: esIndex,
    query: {
      bool: {
        filter: [
          { range: { "@timestamp": { gte: gteTime } } },
          { term: { level: "ERROR" } },
        ],
      },
    },
  });

  const count = response.count || 0;
  return {
    triggered: count >= threshold,
    count,
  };
}

export async function fetchCorrelatedLogs(size: number = 25): Promise<IncidentContext> {
  const response = await es.search({
    index: esIndex,
    size,
    sort: [{ "@timestamp": { order: "desc" } }],
    query: {
      range: {
        "@timestamp": { gte: "now-5m" },
      },
    },
  });

  const hits = response.hits.hits || [];
  const logs: LogEntry[] = hits.map((h: any) => ({
    timestamp: h._source["@timestamp"],
    level: h._source.level || "INFO",
    service: h._source.service || "unknown",
    message: h._source.message || "",
    trace_id: h._source.trace_id,
    http_method: h._source.http_method,
    http_path: h._source.http_path,
    http_status: h._source.http_status,
    stack_trace: h._source.stack_trace,
    extra: h._source.extra,
  }));

  const errorLogs = logs.filter((l) => l.level === "ERROR");
  const signatures = Array.from(new Set(errorLogs.map((e) => e.message))).slice(0, 5);
  const sampleTrace = errorLogs.find((e) => e.trace_id)?.trace_id || "";
  const serviceName = logs[0]?.service || "order-service";

  return {
    serviceName,
    totalErrors: errorLogs.length,
    triggerTime: new Date().toISOString(),
    sampleTraceId: sampleTrace,
    errorSignatures: signatures,
    correlatedLogs: logs,
  };
}
