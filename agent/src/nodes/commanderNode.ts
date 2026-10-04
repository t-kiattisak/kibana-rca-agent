import { WarRoomStateType, CommanderSynthesis } from "../state";
import { generateJSONWithRetry } from "../geminiClient";

export async function commanderNode(state: WarRoomStateType): Promise<Partial<WarRoomStateType>> {
  console.log("   👑 [LangGraph Node: Incident Commander] Synthesizing consensus & evaluating loop condition...");

  const incident = state.incident;
  const perspectives = Object.values(state.perspectives);

  const systemInstruction = `You are the Incident Commander arbitrating an active War Room.
You receive independent perspectives from:
1. Software Tech Lead (Code, Transactions, Stack Traces)
2. SRE Lead (Infra, Connection Pool, Restarts, Configs)
3. Product Lead (Business, GMV impact, Degraded Mode)

Your job is to synthesize a decisive, prioritized action plan:
1. Immediate Workaround (< 5 mins)
2. Short-term Fix (< 2 hrs)
3. Permanent Fix (< 2 sprints)

Determine if we have enough clarity to resolve the incident, or if we need a deep dive verification loop.`;

  const perspectivesText = perspectives
    .map(
      (p) => `### Role: ${p.role}
Assessment: ${p.assessment}
Proposal: ${p.actionProposal}
Citations: ${p.referencedDocs.join(", ")}`
    )
    .join("\n\n");

  const prompt = `Synthesize this Incident War Room debate:
Service: ${incident.serviceName}
Total Errors: ${incident.totalErrors}
Loop Count: ${state.loopCount}

Panel Perspectives:
${perspectivesText}

Return JSON with:
- consensus (string): Unified summary of the decision
- action_steps (array of strings): 3 to 5 chronological, numbered steps
- severity ("CRITICAL", "HIGH", "MEDIUM", "LOW")
- probable_root_cause (string): Root cause explanation
- needs_deep_dive (boolean): Set to true only if root cause is ambiguous and loopCount < 1
- deep_dive_instruction (string): If needs_deep_dive is true, specify questions for experts`;

  const schema = {
    type: "OBJECT",
    properties: {
      consensus: { type: "STRING" },
      action_steps: {
        type: "ARRAY",
        items: { type: "STRING" },
      },
      severity: {
        type: "STRING",
        enum: ["CRITICAL", "HIGH", "MEDIUM", "LOW"],
      },
      probable_root_cause: { type: "STRING" },
      needs_deep_dive: { type: "BOOLEAN" },
      deep_dive_instruction: { type: "STRING" },
    },
    required: ["consensus", "action_steps", "severity", "probable_root_cause", "needs_deep_dive"],
  };

  const { data, promptTokens, responseTokens, totalTokens } = await generateJSONWithRetry<{
    consensus: string;
    action_steps: string[];
    severity: "CRITICAL" | "HIGH" | "MEDIUM" | "LOW";
    probable_root_cause: string;
    needs_deep_dive: boolean;
    deep_dive_instruction?: string;
  }>(prompt, systemInstruction, schema);

  const synthesis: CommanderSynthesis = {
    consensus: data.consensus,
    actionSteps: data.action_steps,
    severity: data.severity,
    probableRootCause: data.probable_root_cause,
    promptTokens,
    responseTokens,
    totalTokens,
  };

  // Only allow deep dive if loopCount < 1 to prevent endless loops
  const shouldDeepDive = data.needs_deep_dive && state.loopCount < 1;

  return {
    synthesis,
    loopCount: 1, // Reducer adds +1
    needsDeepDive: shouldDeepDive,
    deepDiveInstruction: shouldDeepDive ? (data.deep_dive_instruction || "") : "",
  };
}
