package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/t-kiattisak/kibana-rca-agent/internal/model"
)

type Notifier struct {
	slackURL   string
	discordURL string
	httpClient *http.Client
}

func New(slackURL, discordURL string) *Notifier {
	return &Notifier{
		slackURL:   slackURL,
		discordURL: discordURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// PrintConsole outputs a rich, formatted incident summary directly to STDOUT
func (n *Notifier) PrintConsole(incident *model.IncidentContext, rca *model.RCAResult) {
	badge := "🟡 MEDIUM"
	if rca.Severity == "CRITICAL" {
		badge = "🚨 CRITICAL"
	} else if rca.Severity == "HIGH" {
		badge = "🔴 HIGH"
	}

	fmt.Println("\n" + strings.Repeat("═", 75))
	fmt.Printf("%s PRODUCTION INCIDENT DETECTED [%s]\n", badge, incident.ServiceName)
	fmt.Println(strings.Repeat("═", 75))
	fmt.Printf("• Timestamp:       %s\n", incident.TriggerTime.Format(time.RFC3339))
	fmt.Printf("• Service:         %s\n", incident.ServiceName)
	fmt.Printf("• Error Spike:     %d errors detected\n", incident.TotalErrors)
	if incident.SampleTraceID != "" {
		fmt.Printf("• Sample Trace ID: %s\n", incident.SampleTraceID)
	}

	fmt.Println("\n🔍 [EXECUTIVE SUMMARY]:")
	fmt.Printf("  %s\n", rca.IncidentSummary)

	fmt.Println("\n💥 [PROBABLE ROOT CAUSE]:")
	fmt.Printf("  %s\n", rca.ProbableRootCause)

	if len(rca.ImpactedComponents) > 0 {
		fmt.Println("\n🎯 [IMPACTED COMPONENTS]:")
		for _, comp := range rca.ImpactedComponents {
			fmt.Printf("  - %s\n", comp)
		}
	}

	fmt.Println("\n🛠️ [RECOMMENDED ACTIONABLE RUNBOOK]:")
	fmt.Printf("  1. [Immediate Workaround] : %s\n", rca.RecommendedActions.ImmediateWorkaround)
	fmt.Printf("  2. [Config Tuning]        : %s\n", rca.RecommendedActions.ConfigurationTuning)
	fmt.Printf("  3. [Permanent Code Fix]   : %s\n", rca.RecommendedActions.PermanentFix)
	fmt.Println(strings.Repeat("═", 75) + "\n")
}

// SendDiscord dispatches a rich embed card to a Discord webhook if configured
func (n *Notifier) SendDiscord(incident *model.IncidentContext, rca *model.RCAResult) error {
	if n.discordURL == "" {
		return nil
	}

	// Discord decimal color codes
	color := 15844367 // Yellow (Medium)
	badge := "🟡 MEDIUM"
	if rca.Severity == "CRITICAL" {
		color = 15158332 // Red
		badge = "🚨 CRITICAL"
	} else if rca.Severity == "HIGH" {
		color = 15105570 // Orange
		badge = "🔴 HIGH"
	}

	runbook := fmt.Sprintf(
		"**1. Immediate Workaround:** %s\n**2. Config Tuning:** %s\n**3. Permanent Fix:** %s",
		rca.RecommendedActions.ImmediateWorkaround,
		rca.RecommendedActions.ConfigurationTuning,
		rca.RecommendedActions.PermanentFix,
	)

	impacted := "None detected"
	if len(rca.ImpactedComponents) > 0 {
		impacted = strings.Join(rca.ImpactedComponents, ", ")
	}

	payload := map[string]interface{}{
		"username":   "Kibana AI RCA Agent",
		"avatar_url": "https://cdn-icons-png.flaticon.com/512/8649/8649595.png",
		"embeds": []map[string]interface{}{
			{
				"title":       fmt.Sprintf("%s Production Incident: %s", badge, incident.ServiceName),
				"description": fmt.Sprintf("**Incident Detected at:** `%s`\n**Error Spike Count:** `%d errors`", incident.TriggerTime.Format(time.RFC3339), incident.TotalErrors),
				"color":       color,
				"fields": []map[string]interface{}{
					{
						"name":   "🔍 Incident Summary",
						"value":  rca.IncidentSummary,
						"inline": false,
					},
					{
						"name":   "💥 Probable Root Cause",
						"value":  rca.ProbableRootCause,
						"inline": false,
					},
					{
						"name":   "🎯 Impacted Components",
						"value":  impacted,
						"inline": false,
					},
					{
						"name":   "🛠️ Actionable Runbook",
						"value":  runbook,
						"inline": false,
					},
				},
				"footer": map[string]interface{}{
					"text": "Kibana AI RCA Agent • Powered by Gemini",
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
	return nil
}

// SendSlack dispatches an incident card to a Slack webhook if configured
func (n *Notifier) SendSlack(incident *model.IncidentContext, rca *model.RCAResult) error {
	if n.slackURL == "" {
		return nil
	}

	color := "#ffcc00"
	if rca.Severity == "CRITICAL" {
		color = "#cc0000"
	} else if rca.Severity == "HIGH" {
		color = "#ff6600"
	}

	payload := map[string]interface{}{
		"attachments": []map[string]interface{}{
			{
				"color": color,
				"title": fmt.Sprintf("[%s] Incident RCA: %s", rca.Severity, incident.ServiceName),
				"fields": []map[string]interface{}{
					{
						"title": "Incident Summary",
						"value": rca.IncidentSummary,
						"short": false,
					},
					{
						"title": "Probable Root Cause",
						"value": rca.ProbableRootCause,
						"short": false,
					},
					{
						"title": "Immediate Action",
						"value": rca.RecommendedActions.ImmediateWorkaround,
						"short": false,
					},
					{
						"title": "Permanent Fix",
						"value": rca.RecommendedActions.PermanentFix,
						"short": false,
					},
				},
				"footer": "Kibana AI RCA Agent",
				"ts":     time.Now().Unix(),
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := n.httpClient.Post(n.slackURL, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

