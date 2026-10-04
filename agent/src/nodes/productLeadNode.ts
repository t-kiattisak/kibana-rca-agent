import { WarRoomStateType, RolePerspective } from "../state";
import { generateJSONWithRetry } from "../geminiClient";
import { searchKNNChunks } from "./esClient";

export async function productLeadNode(state: WarRoomStateType): Promise<Partial<WarRoomStateType>> {
  console.log("   👔 [LangGraph Node: Product Lead] Analyzing SLA breach & GMV conversion impact...");

  const incident = state.incident;
  const query = `${incident.serviceName} ${incident.errorSignatures.join(" ")}`;

  // Retrieve role-filtered Top-1 chunk from Elasticsearch kNN
  const chunks = await searchKNNChunks(query, "product", 1);
  const docContext = chunks.map((c) => `### ${c.doc_title} (${c.heading})\n${c.content}`).join("\n\n");

  const systemInstruction = `You are the Product Lead and Business Operations Manager.
Your focus is customer conversion, checkout SLA breaches (order success rate drop),
estimated GMV losses during promotion campaigns, cart abandonment, and customer communications.
Enforce ADR-005: Degraded Mode & Asynchronous Intake policy. Cite business rules.`;

  const prompt = `Analyze this incident:
Service: ${incident.serviceName}
Errors: ${incident.totalErrors}
Error Signatures: ${JSON.stringify(incident.errorSignatures)}
${state.deepDiveInstruction ? `Commander Deep Dive Instruction: ${state.deepDiveInstruction}` : ""}

Referenced Business Specs:
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
    role: "Product & Business Operations Lead",
    assessment: data.assessment,
    actionProposal: data.action_proposal,
    referencedDocs: data.referenced_docs,
    promptTokens,
    responseTokens,
    totalTokens,
  };

  return {
    perspectives: {
      product_lead: perspective,
    },
  };
}
