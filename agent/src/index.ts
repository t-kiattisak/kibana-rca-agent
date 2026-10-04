import * as dotenv from "dotenv";
import { checkErrorSpike, fetchCorrelatedLogs } from "./nodes/esClient";
import { buildWarRoomGraph } from "./graph";

dotenv.config({ path: "../.env" });

const pollIntervalSeconds = parseInt(process.env.POLL_INTERVAL_SECONDS || "15", 10);
const timeWindowMinutes = parseInt(process.env.TIME_WINDOW_MINUTES || "1", 10);
const errorThreshold = parseInt(process.env.ERROR_THRESHOLD_COUNT || "3", 10);
const cooldownMs = 3 * 60 * 1000; // 3 minutes cooldown

let lastIncidentTime = 0;

async function main() {
  console.log("============================================================");
  console.log("🚀 Starting LangGraph Multi-Agent Kibana RCA Agent");
  console.log("============================================================");
  console.log(`• Elasticsearch URL  : ${process.env.ELASTICSEARCH_URL || "http://localhost:9200"}`);
  console.log(`• Error Threshold    : ${errorThreshold} errors in ${timeWindowMinutes}m`);
  console.log(`• Poll Interval      : ${pollIntervalSeconds}s`);
  console.log(`• Gemini Model       : ${process.env.GEMINI_MODEL || "gemini-3.5-flash"}`);
  console.log(`• Discord Webhook    : ${process.env.DISCORD_WEBHOOK_URL ? "Configured" : "None"}`);
  console.log(`• Framework          : @langchain/langgraph (Cyclic StateGraph)`);
  console.log("============================================================\n");

  const warRoomApp = buildWarRoomGraph();

  console.log("👀 Watching Elasticsearch logs for incidents...");

  setInterval(async () => {
    try {
      if (Date.now() - lastIncidentTime < cooldownMs) {
        return;
      }

      const { triggered, count } = await checkErrorSpike(timeWindowMinutes, errorThreshold);
      if (!triggered) {
        return;
      }

      lastIncidentTime = Date.now();
      console.log(`\n🚨 [INCIDENT DETECTED] Error spike of ${count} errors in the last ${timeWindowMinutes}m!`);
      console.log("🧠 Convening LangGraph Multi-Agent War Room...");

      const incident = await fetchCorrelatedLogs(25);

      const initialInput = {
        incident,
        retrievedDocs: [],
        perspectives: {},
        synthesis: null,
        loopCount: 0,
        needsDeepDive: false,
        deepDiveInstruction: "",
        isDelivered: false,
      };

      // Execute LangGraph StateGraph
      const finalState = await warRoomApp.invoke(initialInput);

      console.log("\n🏛️ [LangGraph War Room Completed]");
      console.log(`• Severity: ${finalState.synthesis?.severity}`);
      console.log(`• Cycles  : ${finalState.loopCount} loop(s) executed`);
      console.log(`• Status  : Delivered to Discord = ${finalState.isDelivered}`);
    } catch (err) {
      console.error("Error in LangGraph watcher tick:", err);
    }
  }, pollIntervalSeconds * 1000);
}

main().catch(console.error);
