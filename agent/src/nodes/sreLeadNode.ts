import { WarRoomStateType, RolePerspective } from "../state";
import { generateJSONWithRetry } from "../geminiClient";
import { searchKNNChunks } from "./esClient";

export async function sreLeadNode(state: WarRoomStateType): Promise<Partial<WarRoomStateType>> {
  console.log("   🛠️ [LangGraph Node: SRE Lead] Analyzing connection starvation & container health...");

  const incident = state.incident;
  const query = `${incident.serviceName} ${incident.errorSignatures.join(" ")}`;

  // Retrieve role-filtered Top-1 chunk from Elasticsearch kNN
  const chunks = await searchKNNChunks(query, "sre", 1);
  const docContext = chunks.map((c) => `### ${c.doc_title} (${c.heading})\n${c.content}`).join("\n\n");

  const systemInstruction = `You are the Principal Site Reliability Engineer (SRE) and Platform Lead.
Your focus is on infrastructure reliability: HikariCP database connection pool exhaustion,
network latency, upstream 3rd-party timeouts (504 Gateway Timeout), Kubernetes pod health,
and immediate operational mitigation (rolling restart, pool size tuning, circuit breakers).
Propose actionable platform mitigation and cite relevant runbooks.`;

  const prompt = `Analyze this incident:
Service: ${incident.serviceName}
Errors: ${incident.totalErrors}
Error Signatures: ${JSON.stringify(incident.errorSignatures)}
${state.deepDiveInstruction ? `Commander Deep Dive Instruction: ${state.deepDiveInstruction}` : ""}

Sample Logs & Traces:
${incident.correlatedLogs
  .slice(0, 10)
  .map((l) => `[${l.level}] ${l.message} (HTTP ${l.http_status || "-"})`)
  .join("\n")}

Referenced SRE Runbooks:
${docContext}

Return JSON with:
- assessment (string)
- action_proposal (string)
- referenced_docs (array of strings)`;

  const schema = {
    type: "OBJECT",
    properties: {
      assessment: { type: "STRING" },
      action_proposal: { type: "STRING" },
      referenced_docs: {
        type: "ARRAY",
        items: { type: "STRING" },
      },
    },
    required: ["assessment", "action_proposal", "referenced_docs"],
  };

  const { data, promptTokens, responseTokens, totalTokens } = await generateJSONWithRetry<{
    assessment: string;
    action_proposal: string;
    referenced_docs: string[];
  }>(prompt, systemInstruction, schema);

  const perspective: RolePerspective = {
    role: "Principal SRE & Platform Engineer",
    assessment: data.assessment,
    actionProposal: data.action_proposal,
    referencedDocs: data.referenced_docs,
    promptTokens,
    responseTokens,
    totalTokens,
  };

  return {
    perspectives: {
      sre_lead: perspective,
    },
  };
}
