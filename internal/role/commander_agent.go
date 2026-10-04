package role

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/genai"

	"github.com/t-kiattisak/kibana-rca-agent/internal/model"
)

type CommanderAgent struct {
	client *genai.Client
	model  string
}

func NewCommanderAgent(client *genai.Client, model string) *CommanderAgent {
	return &CommanderAgent{client: client, model: model}
}

// SynthesizeConsensus evaluates all expert opinions and produces final triaged action steps
func (a *CommanderAgent) SynthesizeConsensus(ctx context.Context, incident *model.IncidentContext, perspectives []model.RolePerspective) (string, []string, string, string, model.TokenUsage, error) {
	prompt := a.buildPrompt(incident, perspectives)

	schema := &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"severity": {
				Type: genai.TypeString,
				Enum: []string{"CRITICAL", "HIGH", "MEDIUM", "LOW"},
			},
			"incident_summary": {
				Type:        genai.TypeString,
				Description: "Unified high-level summary of the incident",
			},
			"probable_root_cause": {
				Type:        genai.TypeString,
				Description: "Unified technical root cause across code, database, and infra",
			},
			"commander_consensus": {
				Type:        genai.TypeString,
				Description: "Incident Commander synthesized consensus balancing Infra, Code, and Business",
			},
			"action_steps": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeString,
				},
				Description: "Final prioritized action steps: [1. Immediate, 2. Short-term, 3. Permanent]",
			},
		},
		Required: []string{"severity", "incident_summary", "probable_root_cause", "commander_consensus", "action_steps"},
	}

	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: "You are the Incident Commander (Chair of the Incident War Room). " +
					"Review the expert assessments from the Software Tech Lead, Principal SRE, and Product Lead. " +
					"Synthesize a clear, authoritative consensus that balances immediate business continuity with infra stability and code fixes."},
			},
		},
		ResponseMIMEType: "application/json",
		ResponseSchema:   schema,
		Temperature:      genai.Ptr(float32(0.2)),
	}

	result, err := GenerateWithRetry(ctx, a.client, a.model, prompt, config)
	if err != nil {
		return "", nil, "", "", model.TokenUsage{}, fmt.Errorf("Commander synthesis failed: %w", err)
	}

	var res struct {
		Severity           string   `json:"severity"`
		IncidentSummary    string   `json:"incident_summary"`
		ProbableRootCause  string   `json:"probable_root_cause"`
		CommanderConsensus string   `json:"commander_consensus"`
		ActionSteps        []string `json:"action_steps"`
	}
	if err := json.Unmarshal([]byte(result.Text()), &res); err != nil {
		return "", nil, "", "", model.TokenUsage{}, err
	}

	var tokens model.TokenUsage
	if result.UsageMetadata != nil {
		tokens.PromptTokens = int(result.UsageMetadata.PromptTokenCount)
		tokens.CandidatesTokens = int(result.UsageMetadata.CandidatesTokenCount)
		tokens.TotalTokens = int(result.UsageMetadata.TotalTokenCount)
	}

	return res.CommanderConsensus, res.ActionSteps, res.Severity, res.ProbableRootCause, tokens, nil
}

func (a *CommanderAgent) buildPrompt(incident *model.IncidentContext, perspectives []model.RolePerspective) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Incident Commander Review for: %s\n", incident.ServiceName))
	sb.WriteString(fmt.Sprintf("- Total Errors: %d\n\n", incident.TotalErrors))

	sb.WriteString("## Expert Panel Testimonies:\n")
	for _, p := range perspectives {
		sb.WriteString(fmt.Sprintf("### %s\n", p.Role))
		sb.WriteString(fmt.Sprintf("- **Assessment:** %s\n", p.Assessment))
		sb.WriteString(fmt.Sprintf("- **Proposal:** %s\n", p.ActionProposal))
		if len(p.ReferencedDocs) > 0 {
			sb.WriteString(fmt.Sprintf("- **Citations:** %s\n", strings.Join(p.ReferencedDocs, ", ")))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Review these expert viewpoints, resolve conflicts, and decide the prioritized action plan.")
	return sb.String()
}
