package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/t-kiattisak/kibana-rca-agent/internal/model"
)

// PrintWarRoomConsole displays rich multi-agent discussion on STDOUT
func (n *Notifier) PrintWarRoomConsole(incident *model.IncidentContext, warRoom *model.WarRoomResult) {
	fmt.Println("\n" + strings.Repeat("═", 80))
	fmt.Printf("🏛️ MULTI-AGENT INCIDENT WAR ROOM DEBATE: [%s]\n", incident.ServiceName)
	fmt.Println(strings.Repeat("═", 80))
	fmt.Printf("• Detected At:      %s\n", incident.TriggerTime.Format(time.RFC3339))
	fmt.Printf("• Error Spike:      %d errors\n", incident.TotalErrors)
	fmt.Printf("• Severity:         %s\n", warRoom.Severity)
	fmt.Printf("• Token Usage:      Prompt: %d | Response: %d | Total: %d tokens\n",
		warRoom.TotalTokens.PromptTokens, warRoom.TotalTokens.CandidatesTokens, warRoom.TotalTokens.TotalTokens)

	fmt.Println("\n🔍 [SUMMARY & ROOT CAUSE]:")
	fmt.Printf("  %s\n", warRoom.ProbableRootCause)

	fmt.Println("\n🗣️ [EXPERT PANEL DISCUSSIONS]:")
	for _, p := range warRoom.Perspectives {
		icon := "🧑‍💻"
		if strings.Contains(p.Role, "SRE") {
			icon = "🛠️"
		} else if strings.Contains(p.Role, "Product") || strings.Contains(p.Role, "Business") {
			icon = "👔"
		}
		fmt.Printf("\n%s Role: %s\n", icon, p.Role)
		fmt.Printf("   Assessment : %s\n", p.Assessment)
		fmt.Printf("   Proposal   : %s\n", p.ActionProposal)
		if len(p.ReferencedDocs) > 0 {
			fmt.Printf("   Citations  : %s\n", strings.Join(p.ReferencedDocs, ", "))
		}
	}

	fmt.Println("\n👑 [INCIDENT COMMANDER CONSENSUS]:")
	fmt.Printf("  %s\n", warRoom.CommanderConsensus)

	fmt.Println("\n📋 [PRIORITIZED ACTION PLAN]:")
	for i, step := range warRoom.ActionSteps {
		fmt.Printf("  %d. %s\n", i+1, step)
	}
	fmt.Println(strings.Repeat("═", 80) + "\n")
}

// SendWarRoomDiscord dispatches comprehensive War Room card with Token usage to Discord
func (n *Notifier) SendWarRoomDiscord(incident *model.IncidentContext, warRoom *model.WarRoomResult) error {
	if n.discordURL == "" {
		return nil
	}

	color := 15158332 // Red for Critical
	if warRoom.Severity == "HIGH" {
		color = 15105570
	} else if warRoom.Severity == "MEDIUM" {
		color = 15844367
	}

	fields := []map[string]interface{}{
		{
			"name":   "🔍 Incident Summary & Root Cause",
			"value":  truncate(fmt.Sprintf("**Summary:** %s\n**Root Cause:** %s", warRoom.IncidentSummary, warRoom.ProbableRootCause), 1020),
			"inline": false,
		},
	}

	// Add each role's perspective with individual token consumption
	for _, p := range warRoom.Perspectives {
		icon := "🧑‍💻"
		if strings.Contains(p.Role, "SRE") {
			icon = "🛠️"
		} else if strings.Contains(p.Role, "Product") || strings.Contains(p.Role, "Business") {
			icon = "👔"
		}
		docCitations := ""
		if len(p.ReferencedDocs) > 0 {
			docCitations = fmt.Sprintf("\n*📄 Citing: %s*", strings.Join(p.ReferencedDocs, ", "))
		}
		tokenBadge := fmt.Sprintf("\n*(Tokens: in:%d, out:%d, total:%d)*", p.Tokens.PromptTokens, p.Tokens.CandidatesTokens, p.Tokens.TotalTokens)
		fields = append(fields, map[string]interface{}{
			"name":   fmt.Sprintf("%s %s", icon, p.Role),
			"value":  truncate(fmt.Sprintf("**Assessment:** %s\n**Proposed Action:** %s%s%s", p.Assessment, p.ActionProposal, docCitations, tokenBadge), 1020),
			"inline": false,
		})
	}

	// Commander Consensus
	fields = append(fields, map[string]interface{}{
		"name":   "👑 Incident Commander Final Consensus",
		"value":  truncate(warRoom.CommanderConsensus, 1020),
		"inline": false,
	})

	// Prioritized Action Steps
	var actionStepsStr strings.Builder
	for i, step := range warRoom.ActionSteps {
		actionStepsStr.WriteString(fmt.Sprintf("**%d.** %s\n", i+1, step))
	}
	fields = append(fields, map[string]interface{}{
		"name":   "📋 Prioritized Action Plan",
		"value":  truncate(actionStepsStr.String(), 1020),
		"inline": false,
	})

	// Token Usage Breakdown Field
	tokenFooter := fmt.Sprintf("📊 Input Tokens: %d | Output Tokens: %d | Total: %d tokens",
		warRoom.TotalTokens.PromptTokens, warRoom.TotalTokens.CandidatesTokens, warRoom.TotalTokens.TotalTokens)
	fields = append(fields, map[string]interface{}{
		"name":   "💰 Token Consumption & Cost Tracker",
		"value":  fmt.Sprintf("```\nPrompt/Input : %d tokens\nOutput/Reason: %d tokens\nTotal Count  : %d tokens\n```", warRoom.TotalTokens.PromptTokens, warRoom.TotalTokens.CandidatesTokens, warRoom.TotalTokens.TotalTokens),
		"inline": false,
	})

	payload := map[string]interface{}{
		"username":   "Kibana War Room Commander",
		"avatar_url": "https://cdn-icons-png.flaticon.com/512/8649/8649595.png",
		"embeds": []map[string]interface{}{
			{
				"title":       fmt.Sprintf("🏛️ Multi-Agent Incident War Room: %s [%s]", incident.ServiceName, warRoom.Severity),
				"description": fmt.Sprintf("**Trigger Time:** `%s` • **Errors:** `%d`", incident.TriggerTime.Format(time.RFC3339), incident.TotalErrors),
				"color":       color,
				"fields":      fields,
				"footer": map[string]interface{}{
					"text": fmt.Sprintf("Kibana AI RCA Agent • %s", tokenFooter),
				},
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := n.httpClient.Post(n.discordURL, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		return fmt.Errorf("discord returned status %d: %s", resp.StatusCode, buf.String())
	}
	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
