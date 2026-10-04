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

// RunModularWarRoom coordinates independent expert agents with role-filtered vector kNN retrieval
func (a *Analyzer) RunModularWarRoom(ctx context.Context, incident *model.IncidentContext, store interface{
	RetrieveRelevantChunks(ctx context.Context, queryText string, targetRole string, k int) ([]role.KnowledgeDoc, error)
}) (*model.WarRoomResult, error) {
	// Instantiate specialized role agents
	techLead := role.NewTechLeadAgent(a.client, a.model)
	sreLead := role.NewSREAgent(a.client, a.model)
	productLead := role.NewProductLeadAgent(a.client, a.model)
	commander := role.NewCommanderAgent(a.client, a.model)

	var perspectives []model.RolePerspective
	var totalTokens model.TokenUsage

	// Build compact query string from incident signatures
	incidentQuery := fmt.Sprintf("%s %s", incident.ServiceName, strings.Join(incident.ErrorSignatures, " "))

	// 1. Tech Lead: Retrieve code/architecture vector chunks (Top-1 most relevant chunk)
	log.Println("   🧑‍💻 Searching Vector DB (kNN) for Tech Lead architecture chunks...")
	techDocs, err := store.RetrieveRelevantChunks(ctx, incidentQuery, "developer", 1)
	if err != nil || len(techDocs) == 0 {
		log.Printf("      ⚠️ Vector search returned fallback for developer")
	} else {
		log.Printf("      🎯 Retrieved chunk: %s", techDocs[0].Title)
	}
	techView, err := techLead.Analyze(ctx, incident, techDocs)
	if err != nil {
		return nil, fmt.Errorf("tech lead analysis failed: %w", err)
	}
	perspectives = append(perspectives, *techView)
	accumulateTokens(&totalTokens, techView.Tokens)

	// 2. SRE: Retrieve infrastructure/runbook vector chunks (Top-1 most relevant chunk)
	log.Println("   🛠️ Searching Vector DB (kNN) for SRE runbook chunks...")
	sreDocs, err := store.RetrieveRelevantChunks(ctx, incidentQuery, "sre", 1)
	if err != nil || len(sreDocs) == 0 {
		log.Printf("      ⚠️ Vector search returned fallback for SRE")
	} else {
		log.Printf("      🎯 Retrieved chunk: %s", sreDocs[0].Title)
	}
	sreView, err := sreLead.Analyze(ctx, incident, sreDocs)
	if err != nil {
		return nil, fmt.Errorf("SRE analysis failed: %w", err)
	}
	perspectives = append(perspectives, *sreView)
	accumulateTokens(&totalTokens, sreView.Tokens)

	// 3. Product Lead: Retrieve business policy vector chunks (Top-1 most relevant chunk)
	log.Println("   👔 Searching Vector DB (kNN) for Product Lead business policy chunks...")
	prodDocs, err := store.RetrieveRelevantChunks(ctx, incidentQuery, "product", 1)
	if err != nil || len(prodDocs) == 0 {
		log.Printf("      ⚠️ Vector search returned fallback for product")
	} else {
		log.Printf("      🎯 Retrieved chunk: %s", prodDocs[0].Title)
	}
	prodView, err := productLead.Analyze(ctx, incident, prodDocs)
	if err != nil {
		return nil, fmt.Errorf("product lead analysis failed: %w", err)
	}
	perspectives = append(perspectives, *prodView)
	accumulateTokens(&totalTokens, prodView.Tokens)

	// 4. Incident Commander: Synthesizes final prioritized runbook
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
