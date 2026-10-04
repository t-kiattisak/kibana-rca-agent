package role

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/genai"

	"github.com/t-kiattisak/kibana-rca-agent/internal/model"
)

type ProductLeadAgent struct {
	client *genai.Client
	model  string
}

func NewProductLeadAgent(client *genai.Client, model string) *ProductLeadAgent {
	return &ProductLeadAgent{client: client, model: model}
}

func (a *ProductLeadAgent) Name() string      { return "product_lead" }
func (a *ProductLeadAgent) RoleTitle() string { return "Product & Business Lead" }

func (a *ProductLeadAgent) Analyze(ctx context.Context, incident *model.IncidentContext, docs []KnowledgeDoc) (*model.RolePerspective, error) {
	var relevantDocs []KnowledgeDoc
	for _, d := range docs {
		if d.Role == "product" || d.Category == "business_policy" {
			relevantDocs = append(relevantDocs, d)
		}
	}

	prompt := a.buildPrompt(incident, relevantDocs)
	schema := &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"assessment": {
				Type:        genai.TypeString,
				Description: "Business & Product analysis on customer conversion, GMV revenue loss, SLA thresholds, and cart drops",
			},
			"action_proposal": {
				Type:        genai.TypeString,
				Description: "Business continuity actions: Degraded mode invocation (ADR-005), marketing alerts, customer comms",
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
				{Text: "You are the Product and Business Operations Lead. " +
					"Your focus is customer conversion, SLA breach risks, business revenue impact ($ GMV losses), " +
					"and triggering Degraded Mode / Asynchronous intake policies per Business ADRs to minimize business damage."},
			},
		},
		ResponseMIMEType: "application/json",
		ResponseSchema:   schema,
		Temperature:      genai.Ptr(float32(0.2)),
	}

	result, err := GenerateWithRetry(ctx, a.client, a.model, prompt, config)
	if err != nil {
		return nil, fmt.Errorf("Product Lead analysis failed: %w", err)
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

func (a *ProductLeadAgent) buildPrompt(incident *model.IncidentContext, docs []KnowledgeDoc) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Product & Business Impact Analysis for: %s\n", incident.ServiceName))
	sb.WriteString(fmt.Sprintf("- Incident Timestamp: %s\n", incident.TriggerTime.Format("2006-01-02 15:04:05 MST")))
	sb.WriteString(fmt.Sprintf("- Total Error Spike: %d errors\n", incident.TotalErrors))
	sb.WriteString("\n## Business Policies & SLA Specs:\n")
	for _, d := range docs {
		sb.WriteString(fmt.Sprintf("### %s\n%s\n", d.Title, d.Content))
	}
	return sb.String()
}
