import { WarRoomStateType, RolePerspective } from "../state";
import { generateJSONWithRetry } from "../geminiClient";
import { searchKNNChunks } from "./esClient";

export async function techLeadNode(state: WarRoomStateType): Promise<Partial<WarRoomStateType>> {
  console.log("   🧑‍💻 [LangGraph Node: Tech Lead] Analyzing code stack traces & transaction leaks...");

  const incident = state.incident;
  const query = `${incident.serviceName} ${incident.errorSignatures.join(" ")}`;

  // Retrieve role-filtered Top-1 chunk from Elasticsearch kNN
  const chunks = await searchKNNChunks(query, "developer", 1);
  const docContext = chunks.map((c) => `### ${c.doc_title} (${c.heading})\n${c.content}`).join("\n\n");

  const systemInstruction = `You are the Software Tech Lead and Application Architect.
Your focus is on code-level root causes: stack traces, database transaction lifecycles (tx.Begin, defer tx.Rollback),
context deadline propagation (context.WithTimeout), nil-pointer panics, and unit/integration test gaps.
Propose concrete code hotfixes and cite architecture specs.`;

  const prompt = `Analyze this incident:
Service: ${incident.serviceName}
Errors: ${incident.totalErrors}
Error Signatures: ${JSON.stringify(incident.errorSignatures)}
${state.deepDiveInstruction ? `Commander Deep Dive Instruction: ${state.deepDiveInstruction}` : ""}

Sample Logs & Traces:
${incident.correlatedLogs
  .slice(0, 10)
  .map((l) => `[${l.level}] ${l.message} ${l.stack_trace ? `\nStack:\n${l.stack_trace}` : ""}`)
  .join("\n")}

Referenced Architecture Knowledge:
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
    role: "Software Tech Lead & Code Architect",
    assessment: data.assessment,
    actionProposal: data.action_proposal,
    referencedDocs: data.referenced_docs,
    promptTokens,
    responseTokens,
    totalTokens,
  };

  return {
    perspectives: {
      tech_lead: perspective,
    },
  };
}
