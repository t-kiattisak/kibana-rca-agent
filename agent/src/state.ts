import { Annotation } from "@langchain/langgraph";

export interface LogEntry {
  timestamp: string;
  level: string;
  service: string;
  message: string;
  trace_id?: string;
  http_method?: string;
  http_path?: string;
  http_status?: number;
  stack_trace?: string;
  extra?: Record<string, any>;
}

export interface IncidentContext {
  serviceName: string;
  totalErrors: number;
  triggerTime: string;
  sampleTraceId: string;
  errorSignatures: string[];
  correlatedLogs: LogEntry[];
}

export interface KnowledgeChunk {
  chunk_id: string;
  doc_title: string;
  category: string;
  target_role: string;
  heading: string;
  content: string;
}

export interface RolePerspective {
  role: string;
  assessment: string;
  actionProposal: string;
  referencedDocs: string[];
  promptTokens: number;
  responseTokens: number;
  totalTokens: number;
}

export interface CommanderSynthesis {
  consensus: string;
  actionSteps: string[];
  severity: "CRITICAL" | "HIGH" | "MEDIUM" | "LOW";
  probableRootCause: string;
  promptTokens: number;
  responseTokens: number;
  totalTokens: number;
}

// LangGraph Shared State Annotation
export const WarRoomStateAnnotation = Annotation.Root({
  // The detected incident context
  incident: Annotation<IncidentContext>(),

  // Retrieved knowledge chunks for all roles
  retrievedDocs: Annotation<KnowledgeChunk[]>({
    reducer: (curr, update) => update ?? curr,
    default: () => [],
  }),

  // Individual role perspectives
  perspectives: Annotation<Record<string, RolePerspective>>({
    reducer: (curr, update) => ({ ...curr, ...update }),
    default: () => ({}),
  }),

  // Commander final consensus
  synthesis: Annotation<CommanderSynthesis | null>({
    reducer: (curr, update) => update ?? curr,
    default: () => null,
  }),

  // Loop & verification counter for agentic loops
  loopCount: Annotation<number>({
    reducer: (curr, update) => curr + update,
    default: () => 0,
  }),

  // Flag if Commander needs more evidence / investigation
  needsDeepDive: Annotation<boolean>({
    reducer: (curr, update) => update ?? curr,
    default: () => false,
  }),

  // Deep dive instruction for the next cycle
  deepDiveInstruction: Annotation<string>({
    reducer: (curr, update) => update ?? curr,
    default: () => "",
  }),

  // Delivery status
  isDelivered: Annotation<boolean>({
    reducer: (curr, update) => update ?? curr,
    default: () => false,
  }),
});

export type WarRoomStateType = typeof WarRoomStateAnnotation.State;
