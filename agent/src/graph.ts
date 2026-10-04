import { StateGraph, START, END } from "@langchain/langgraph";
import { WarRoomStateAnnotation, WarRoomStateType } from "./state";
import { techLeadNode } from "./nodes/techLeadNode";
import { sreLeadNode } from "./nodes/sreLeadNode";
import { productLeadNode } from "./nodes/productLeadNode";
import { commanderNode } from "./nodes/commanderNode";
import { discordNotifierNode } from "./nodes/discordNotifierNode";

// Conditional Edge Router: Evaluate whether to loop back or notify
export function routeAfterCommander(state: WarRoomStateType): "tech_lead" | "discord_notifier" {
  if (state.needsDeepDive && state.loopCount < 2) {
    console.log(`   🔄 [LangGraph Cyclic Loop triggered] Deep dive requested by Commander. Re-evaluating...`);
    return "tech_lead";
  }
  return "discord_notifier";
}

export function buildWarRoomGraph() {
  const workflow = new StateGraph(WarRoomStateAnnotation)
    // 1. Add all functional nodes
    .addNode("tech_lead", techLeadNode)
    .addNode("sre_lead", sreLeadNode)
    .addNode("product_lead", productLeadNode)
    .addNode("commander", commanderNode)
    .addNode("discord_notifier", discordNotifierNode)

    // 2. Multi-Agent Fan-out from START
    .addEdge(START, "tech_lead")
    .addEdge(START, "sre_lead")
    .addEdge(START, "product_lead")

    // 3. Fan-in to Commander
    .addEdge("tech_lead", "commander")
    .addEdge("sre_lead", "commander")
    .addEdge("product_lead", "commander")

    // 4. Conditional Edge: Agentic Loop or Conclude
    .addConditionalEdges("commander", routeAfterCommander, {
      tech_lead: "tech_lead",
      discord_notifier: "discord_notifier",
    })

    // 5. End of graph
    .addEdge("discord_notifier", END);

  return workflow.compile();
}
