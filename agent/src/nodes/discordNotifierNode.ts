import { WarRoomStateType } from "../state";
import * as dotenv from "dotenv";

dotenv.config({ path: "../.env" });

const discordUrl = process.env.DISCORD_WEBHOOK_URL || "";

function truncate(str: string, maxLen: number): string {
  if (str.length <= maxLen) return str;
  return str.slice(0, maxLen - 3) + "...";
}

export async function discordNotifierNode(state: WarRoomStateType): Promise<Partial<WarRoomStateType>> {
  console.log("   📢 [LangGraph Node: Discord Notifier] Dispatching Rich Embed card...");

  if (!discordUrl) {
    console.warn("DISCORD_WEBHOOK_URL not configured. Skipping Discord delivery.");
    return { isDelivered: true };
  }

  const incident = state.incident;
  const synthesis = state.synthesis;
  if (!synthesis) {
    return { isDelivered: false };
  }

  let color = 15158332; // Red for Critical
  if (synthesis.severity === "HIGH") color = 15105570;
  else if (synthesis.severity === "MEDIUM") color = 15844367;

  const perspectives = Object.values(state.perspectives);

  let totalPrompt = synthesis.promptTokens;
  let totalCandidates = synthesis.responseTokens;
  let totalTokens = synthesis.totalTokens;

  perspectives.forEach((p) => {
    totalPrompt += p.promptTokens;
    totalCandidates += p.responseTokens;
    totalTokens += p.totalTokens;
  });

  const fields: any[] = [
    {
      name: "🔍 Incident Summary & Root Cause",
      value: truncate(
        `**Service:** \`${incident.serviceName}\`\n**Root Cause:** ${synthesis.probableRootCause}`,
        1020
      ),
      inline: false,
    },
  ];

  perspectives.forEach((p) => {
    let icon = "🧑‍💻";
    if (p.role.includes("SRE")) icon = "🛠️";
    else if (p.role.includes("Product") || p.role.includes("Business")) icon = "👔";

    const citations = p.referencedDocs.length > 0 ? `\n*📄 Citing: ${p.referencedDocs.join(", ")}*` : "";
    const badge = `\n*(Tokens: in:${p.promptTokens}, out:${p.responseTokens}, total:${p.totalTokens})*`;

    fields.push({
      name: `${icon} ${p.role}`,
      value: truncate(`**Assessment:** ${p.assessment}\n**Action Proposal:** ${p.actionProposal}${citations}${badge}`, 1020),
      inline: false,
    });
  });

  fields.push({
    name: "👑 Incident Commander Final Consensus",
    value: truncate(synthesis.consensus, 1020),
    inline: false,
  });

  const actionStepsText = synthesis.actionSteps.map((s, idx) => `**${idx + 1}.** ${s}`).join("\n");
  fields.push({
    name: "📋 Prioritized Action Plan",
    value: truncate(actionStepsText, 1020),
    inline: false,
  });

  fields.push({
    name: "💰 Token Consumption & Cost Tracker (LangGraph Cycle)",
    value: `\`\`\`\nPrompt/Input : ${totalPrompt} tokens\nOutput/Reason: ${totalCandidates} tokens\nTotal Count  : ${totalTokens} tokens\nCycles / Loops: ${state.loopCount}\n\`\`\``,
    inline: false,
  });

  const payload = {
    username: "LangGraph War Room Commander",
    avatar_url: "https://cdn-icons-png.flaticon.com/512/8649/8649595.png",
    embeds: [
      {
        title: `🏛️ LangGraph Incident War Room: ${incident.serviceName} [${synthesis.severity}]`,
        description: `**Trigger Time:** \`${incident.triggerTime}\` • **Total Errors:** \`${incident.totalErrors}\` • **LangGraph Engine:** \`TypeScript / Cyclic Graph\``,
        color,
        fields,
        footer: {
          text: `LangGraph Multi-Agent • Input: ${totalPrompt} | Output: ${totalCandidates} | Total: ${totalTokens} tokens`,
        },
        timestamp: new Date().toISOString(),
      },
    ],
  };

  try {
    const res = await fetch(discordUrl, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });

    if (!res.ok) {
      const errText = await res.text();
      console.error(`Failed to post to Discord (HTTP ${res.status}): ${errText}`);
      return { isDelivered: false };
    }

    console.log("   ✅ Successfully delivered LangGraph War Room card to Discord!");

    // Save resolution into Long-Term Cross-Incident Store
    const { saveIncidentToStore } = await import("../memoryStore");
    await saveIncidentToStore(incident.serviceName, {
      serviceName: incident.serviceName,
      timestamp: incident.triggerTime,
      probableRootCause: synthesis.probableRootCause,
      consensus: synthesis.consensus,
      effectiveActionSteps: synthesis.actionSteps,
    });

    return { isDelivered: true };
  } catch (err) {
    console.error("Discord delivery exception:", err);
    return { isDelivered: false };
  }
}
