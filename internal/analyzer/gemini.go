package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"google.golang.org/genai"

	"github.com/t-kiattisak/kibana-rca-agent/internal/model"
)

type Analyzer struct {
	client *genai.Client
	model  string
}

func New(ctx context.Context, apiKey, modelName string) (*Analyzer, error) {
	if modelName == "" {
		modelName = "gemini-2.5-flash"
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	return &Analyzer{
		client: client,
		model:  modelName,
	}, nil
}

// AnalyzeIncident builds structured prompt and queries Gemini LLM
func (a *Analyzer) AnalyzeIncident(ctx context.Context, incident *model.IncidentContext) (*model.RCAResult, error) {
	prompt := buildPrompt(incident)

	schema := &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"severity": {
				Type:        genai.TypeString,
				Description: "Incident severity: CRITICAL, HIGH, MEDIUM, or LOW",
				Enum:        []string{"CRITICAL", "HIGH", "MEDIUM", "LOW"},
			},
			"incident_summary": {
				Type:        genai.TypeString,
				Description: "High-level executive summary of what went wrong and user impact",
			},
			"probable_root_cause": {
				Type:        genai.TypeString,
				Description: "Technical root cause explanation citing specific error messages and stack trace context",
			},
			"impacted_components": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeString,
				},
				Description: "List of services, database tables, or third-party endpoints impacted",
			},
			"recommended_actions": {
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"immediate_workaround": {
						Type:        genai.TypeString,
						Description: "Fast mitigation steps for on-call responder to stop the bleeding immediately",
					},
					"configuration_tuning": {
						Type:        genai.TypeString,
						Description: "Infrastructure or environment configuration adjustments",
					},
					"permanent_fix": {
						Type:        genai.TypeString,
						Description: "Long-term architectural or code fix required",
					},
				},
				Required: []string{"immediate_workaround", "configuration_tuning", "permanent_fix"},
			},
		},
		Required: []string{"severity", "incident_summary", "probable_root_cause", "impacted_components", "recommended_actions"},
	}

	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: "You are an Elite Principal SRE (Site Reliability Engineer) and Incident Commander. " +
					"Your job is to analyze log streams, stack traces, and error patterns to pinpoint the exact root cause " +
					"and provide actionable runbook recommendations. Be precise, avoid fluff, and cite concrete code lines and error codes."},
			},
		},
		ResponseMIMEType: "application/json",
		ResponseSchema:   schema,
		Temperature:      genai.Ptr(float32(0.2)), // Low temperature for deterministic analysis
	}

	result, err := a.client.Models.GenerateContent(ctx, a.model, genai.Text(prompt), config)
	if err != nil {
		return nil, fmt.Errorf("LLM analysis failed: %w", err)
	}

	responseText := result.Text()
	if responseText == "" {
		return nil, fmt.Errorf("empty response received from Gemini")
	}

	var rca model.RCAResult
	if err := json.Unmarshal([]byte(responseText), &rca); err != nil {
		return nil, fmt.Errorf("failed to parse structured RCA response: %w (raw: %s)", err, responseText)
	}

	rca.AnalyzedAt = time.Now().UTC()
	return &rca, nil
}

func buildPrompt(incident *model.IncidentContext) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Production Incident Detected on Service: %s\n", incident.ServiceName))
	sb.WriteString(fmt.Sprintf("- Incident Timestamp: %s\n", incident.TriggerTime.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("- Approximate Error Count: %d\n", incident.TotalErrors))
	if incident.SampleTraceID != "" {
		sb.WriteString(fmt.Sprintf("- Sample Trace ID: %s\n", incident.SampleTraceID))
	}

	sb.WriteString("\n## Error Signatures Captured:\n")
	for i, sig := range incident.ErrorSignatures {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, sig))
	}

	sb.WriteString("\n## Correlated Logs & Stack Traces (Chronological Context):\n```json\n")
	logsJSON, _ := json.MarshalIndent(incident.CorrelatedLogs, "", "  ")
	sb.WriteString(string(logsJSON))
	sb.WriteString("\n```\n")

	sb.WriteString("\nPerform a comprehensive Root Cause Analysis (RCA) and return JSON adhering to the specified schema.")
	return sb.String()
}
