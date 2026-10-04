package analyzer

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/t-kiattisak/kibana-rca-agent/internal/model"
	"github.com/t-kiattisak/kibana-rca-agent/internal/role"
)

// LoadKnowledgeDocs reads domain markdown files from docs/knowledge
func LoadKnowledgeDocs(dir string) []role.KnowledgeDoc {
	var docs []role.KnowledgeDoc
	files, err := os.ReadDir(dir)
	if err != nil {
		return docs
	}

	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".md") {
			content, err := os.ReadFile(filepath.Join(dir, f.Name()))
			if err != nil {
				continue
			}
			text := string(content)
			targetRole := "developer"
			category := "architecture"

			if strings.Contains(text, "Target Role: sre") {
				targetRole = "sre"
				category = "infrastructure"
			} else if strings.Contains(text, "Target Role: product") {
				targetRole = "product"
				category = "business_policy"
			}

			docs = append(docs, role.KnowledgeDoc{
				Title:    f.Name(),
				Category: category,
				Role:     targetRole,
				Content:  text,
			})
		}
	}
	return docs
}

// RunModularWarRoom coordinates independent expert agents and calculates per-role token breakdown
func (a *Analyzer) RunModularWarRoom(ctx context.Context, incident *model.IncidentContext, docs []role.KnowledgeDoc) (*model.WarRoomResult, error) {
	// Instantiate specialized role agents
	techLead := role.NewTechLeadAgent(a.client, a.model)
	sreLead := role.NewSREAgent(a.client, a.model)
	productLead := role.NewProductLeadAgent(a.client, a.model)
	commander := role.NewCommanderAgent(a.client, a.model)

	var perspectives []model.RolePerspective
	var totalTokens model.TokenUsage

	// 1. Tech Lead analyzes code, traces, and tests
	log.Println("   🧑‍💻 Consulting Software Tech Lead...")
	techView, err := techLead.Analyze(ctx, incident, docs)
	if err != nil {
		return nil, fmt.Errorf("tech lead analysis failed: %w", err)
	}
	perspectives = append(perspectives, *techView)
	accumulateTokens(&totalTokens, techView.Tokens)

	// 2. SRE analyzes infrastructure, connection pools, and container health
	log.Println("   🛠️ Consulting Principal SRE...")
	sreView, err := sreLead.Analyze(ctx, incident, docs)
	if err != nil {
		return nil, fmt.Errorf("SRE analysis failed: %w", err)
	}
	perspectives = append(perspectives, *sreView)
	accumulateTokens(&totalTokens, sreView.Tokens)

	// 3. Product Lead analyzes business policy, GMV revenue impact, and degraded modes
	log.Println("   👔 Consulting Product & Business Lead...")
	prodView, err := productLead.Analyze(ctx, incident, docs)
	if err != nil {
		return nil, fmt.Errorf("product lead analysis failed: %w", err)
	}
	perspectives = append(perspectives, *prodView)
	accumulateTokens(&totalTokens, prodView.Tokens)

	// 4. Incident Commander synthesizes consensus and final prioritized runbook
	log.Println("   👑 Incident Commander synthesizing final consensus...")
	consensus, actionSteps, severity, rootCause, cmdTokens, err := commander.SynthesizeConsensus(ctx, incident, perspectives)
	if err != nil {
		return nil, fmt.Errorf("commander consensus failed: %w", err)
	}
	accumulateTokens(&totalTokens, cmdTokens)

	return &model.WarRoomResult{
		Severity:           severity,
		IncidentSummary:    fmt.Sprintf("Availability incident on %s investigated by 3 specialized experts.", incident.ServiceName),
		ProbableRootCause:  rootCause,
		Perspectives:       perspectives,
		CommanderConsensus: consensus,
		ActionSteps:        actionSteps,
		TotalTokens:        totalTokens,
		AnalyzedAt:         time.Now().UTC(),
	}, nil
}

func accumulateTokens(total *model.TokenUsage, addition model.TokenUsage) {
	total.PromptTokens += addition.PromptTokens
	total.CandidatesTokens += addition.CandidatesTokens
	total.TotalTokens += addition.TotalTokens
}
