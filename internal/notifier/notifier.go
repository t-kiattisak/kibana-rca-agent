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
	httpClient *http.Client
}

func New(slackURL string) *Notifier {
	return &Notifier{
		slackURL: slackURL,
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
