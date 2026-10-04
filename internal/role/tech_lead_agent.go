package role

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/genai"

	"github.com/t-kiattisak/kibana-rca-agent/internal/model"
)

type TechLeadAgent struct {
	client *genai.Client
	model  string
}

func NewTechLeadAgent(client *genai.Client, model string) *TechLeadAgent {
	return &TechLeadAgent{client: client, model: model}
}

func (a *TechLeadAgent) Name() string      { return "tech_lead" }
func (a *TechLeadAgent) RoleTitle() string { return "Software Tech Lead & Code Architect" }

func (a *TechLeadAgent) Analyze(ctx context.Context, incident *model.IncidentContext, docs []KnowledgeDoc) (*model.RolePerspective, error) {
	var relevantDocs []KnowledgeDoc
	for _, d := range docs {
		if d.Role == "developer" || d.Category == "architecture" {
			relevantDocs = append(relevantDocs, d)
		}
	}

	prompt := a.buildPrompt(incident, relevantDocs)
	schema := &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"assessment": {
				Type:        genai.TypeString,
				Description: "Tech Lead analysis on stack traces, context timeout omission, transaction leaks, and test gaps",
			},
			"action_proposal": {
				Type:        genai.TypeString,
				Description: "Code-level remedies: PR hotfix, defer cancel/rollback, defensive nil checks, and integration tests",
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
				{Text: "You are the Software Tech Lead and Application Architect. " +
					"Your focus is on code-level root causes: stack traces, database transaction lifecycles, " +
					"context deadline propagation (context.WithTimeout), nil-pointer panics, and unit/integration test gaps. " +
					"Propose concrete code hotfixes and cite architecture specs."},
			},
		},
		ResponseMIMEType: "application/json",
		ResponseSchema:   schema,
		Temperature:      genai.Ptr(float32(0.2)),
	}

	result, err := GenerateWithRetry(ctx, a.client, a.model, prompt, config)
	if err != nil {
		return nil, fmt.Errorf("Tech Lead analysis failed: %w", err)
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

func (a *TechLeadAgent) buildPrompt(incident *model.IncidentContext, docs []KnowledgeDoc) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Software Tech Lead Code Inspection for: %s\n", incident.ServiceName))
	sb.WriteString("## Correlated Stack Traces & Error Logs:\n")
	for _, l := range incident.CorrelatedLogs {
		if l.StackTrace != "" {
			sb.WriteString(fmt.Sprintf("```\n%s\n```\n", l.StackTrace))
		} else if l.Level == "ERROR" {
			sb.WriteString(fmt.Sprintf("- %s\n", l.Message))
		}
	}
	sb.WriteString("\n## Architecture & Testing Specs:\n")
	for _, d := range docs {
		sb.WriteString(fmt.Sprintf("### %s\n%s\n", d.Title, d.Content))
	}
	return sb.String()
}
