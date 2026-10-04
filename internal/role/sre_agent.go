package role

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/genai"

	"github.com/t-kiattisak/kibana-rca-agent/internal/model"
)

type SREAgent struct {
	client *genai.Client
	model  string
}

func NewSREAgent(client *genai.Client, model string) *SREAgent {
	return &SREAgent{client: client, model: model}
}

func (a *SREAgent) Name() string      { return "sre_specialist" }
func (a *SREAgent) RoleTitle() string { return "Principal SRE & Platform Engineer" }

func (a *SREAgent) Analyze(ctx context.Context, incident *model.IncidentContext, docs []KnowledgeDoc) (*model.RolePerspective, error) {
	var relevantDocs []KnowledgeDoc
	for _, d := range docs {
		if d.Role == "sre" || d.Category == "infrastructure" {
			relevantDocs = append(relevantDocs, d)
		}
	}

	prompt := a.buildPrompt(incident, relevantDocs)
	schema := &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"assessment": {
				Type:        genai.TypeString,
				Description: "SRE perspective on container health, pool starvation, connection limits, and infra stability",
			},
			"action_proposal": {
				Type:        genai.TypeString,
				Description: "Immediate operational steps: pod rollout, pool size adjustments, and runbook triage",
			},
			"referenced_docs": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeString,
				},
			},
		},
		Required: []string{"assessment", "action_proposal", "referenced_docs"},
	}

	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: "You are the Principal SRE (Site Reliability Engineer) and Platform Architect. " +
					"Your core responsibilities are infrastructure availability, container health, database connection pool limits (HikariCP, PgBouncer), " +
					"and immediate operational mitigations (pod restarts, ConfigMap adjustments). Cite relevant infrastructure runbooks."},
			},
		},
		ResponseMIMEType: "application/json",
		ResponseSchema:   schema,
		Temperature:      genai.Ptr(float32(0.2)),
	}

	result, err := GenerateWithRetry(ctx, a.client, a.model, prompt, config)
	if err != nil {
		return nil, fmt.Errorf("SRE analysis failed: %w", err)
	}

	var res struct {
		Assessment     string   `json:"assessment"`
		ActionProposal string   `json:"action_proposal"`
		ReferencedDocs []string `json:"referenced_docs"`
	}
	if err := json.Unmarshal([]byte(result.Text()), &res); err != nil {
		return nil, err
	}

	var tokens model.TokenUsage
	if result.UsageMetadata != nil {
		tokens.PromptTokens = int(result.UsageMetadata.PromptTokenCount)
		tokens.CandidatesTokens = int(result.UsageMetadata.CandidatesTokenCount)
		tokens.TotalTokens = int(result.UsageMetadata.TotalTokenCount)
	}

	return &model.RolePerspective{
		Role:           a.RoleTitle(),
		Assessment:     res.Assessment,
		ActionProposal: res.ActionProposal,
		ReferencedDocs: res.ReferencedDocs,
		Tokens:         tokens,
	}, nil
}

func (a *SREAgent) buildPrompt(incident *model.IncidentContext, docs []KnowledgeDoc) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# SRE Analysis for Service: %s\n", incident.ServiceName))
	sb.WriteString(fmt.Sprintf("- Total Errors: %d\n", incident.TotalErrors))
	sb.WriteString("## Error Signatures:\n")
	for _, sig := range incident.ErrorSignatures {
		sb.WriteString(fmt.Sprintf("- %s\n", sig))
	}
	sb.WriteString("\n## Infrastructure Runbooks:\n")
	for _, d := range docs {
		sb.WriteString(fmt.Sprintf("### %s\n%s\n", d.Title, d.Content))
	}
	return sb.String()
}
