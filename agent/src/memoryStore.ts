import { InMemoryStore } from "@langchain/langgraph";

// Long-Term Cross-Incident Memory Store
// Stores past resolution summaries, root causes, and action plans indexed by serviceName
export const incidentMemoryStore = new InMemoryStore();

export interface HistoricalResolution {
  serviceName: string;
  timestamp: string;
  probableRootCause: string;
  consensus: string;
  effectiveActionSteps: string[];
}

export async function saveIncidentToStore(
  serviceName: string,
  resolution: HistoricalResolution
): Promise<void> {
  const namespace = ["incident_history", serviceName];
  const key = `res_${Date.now()}`;

  await incidentMemoryStore.put(namespace, key, resolution);
  console.log(`   🧠 [Long-Term Memory] Persisted resolution into store for [${serviceName}]`);
}

export async function getHistoricalContext(serviceName: string): Promise<string> {
  const namespace = ["incident_history", serviceName];
  const items = await incidentMemoryStore.search(namespace, { limit: 2 });

  if (!items || items.length === 0) {
    return "";
  }

  const summaries = items.map((it: any) => {
    const val = it.value as HistoricalResolution;
    return `- Previous Incident at ${val.timestamp}:\n  Root Cause: ${val.probableRootCause}\n  Resolution Consensus: ${val.consensus}`;
  });

  return `### Known Historical Precedents for Service [${serviceName}]:\n${summaries.join("\n\n")}`;
}
