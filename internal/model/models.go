package model

import "time"

// LogDoc represents a single log entry retrieved from Elasticsearch
type LogDoc struct {
	Timestamp  time.Time              `json:"@timestamp"`
	Service    string                 `json:"service"`
	TraceID    string                 `json:"trace_id"`
	Level      string                 `json:"level"`
	Message    string                 `json:"message"`
	HTTPMethod string                 `json:"http_method,omitempty"`
	HTTPPath   string                 `json:"http_path,omitempty"`
	HTTPStatus int                    `json:"http_status,omitempty"`
	DurationMs int64                  `json:"duration_ms,omitempty"`
	StackTrace string                 `json:"stack_trace,omitempty"`
	Extra      map[string]interface{} `json:"extra,omitempty"`
}

// IncidentContext collects correlated evidence for the LLM
type IncidentContext struct {
	ServiceName     string    `json:"service_name"`
	TriggerTime     time.Time `json:"trigger_time"`
	TotalErrors     int       `json:"total_errors"`
	SampleTraceID   string    `json:"sample_trace_id,omitempty"`
	ErrorSignatures []string  `json:"error_signatures"`
	CorrelatedLogs  []LogDoc  `json:"correlated_logs"`
}

// RecommendedActions defines clear triaged action steps
type RecommendedActions struct {
	ImmediateWorkaround string `json:"immediate_workaround"`
	ConfigurationTuning string `json:"configuration_tuning"`
	PermanentFix        string `json:"permanent_fix"`
}

// RCAResult is the structured output returned by the LLM
type RCAResult struct {
	Severity           string             `json:"severity"` // CRITICAL, HIGH, MEDIUM, LOW
	IncidentSummary    string             `json:"incident_summary"`
	ProbableRootCause  string             `json:"probable_root_cause"`
	ImpactedComponents []string           `json:"impacted_components"`
	RecommendedActions RecommendedActions `json:"recommended_actions"`
	AnalyzedAt         time.Time          `json:"analyzed_at"`
}
