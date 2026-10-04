package sanitizer

import (
	"strings"
	"testing"

	"github.com/t-kiattisak/kibana-rca-agent/internal/model"
)

func TestMaskString(t *testing.T) {
	input := "User logged in with password=SuperSecretPass123! and Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.xyz"
	masked := MaskString(input)

	if strings.Contains(masked, "SuperSecretPass123!") {
		t.Errorf("Expected password to be masked, got: %s", masked)
	}

	if !strings.Contains(masked, "[REDACTED_SECRET]") {
		t.Errorf("Expected [REDACTED_SECRET], got: %s", masked)
	}

	if !strings.Contains(masked, "[REDACTED_TOKEN]") {
		t.Errorf("Expected [REDACTED_TOKEN], got: %s", masked)
	}
}

func TestSanitizeIncident(t *testing.T) {
	incident := &model.IncidentContext{
		ServiceName: "order-service",
		ErrorSignatures: []string{
			"Failed auth: token=my_secret_token_12345",
		},
		CorrelatedLogs: []model.LogDoc{
			{
				Message: "Payment failed for card 4111 2222 3333 4444",
				Extra: map[string]interface{}{
					"api_key": "secret_key_12345",
				},
			},
		},
	}

	SanitizeIncident(incident)

	if strings.Contains(incident.CorrelatedLogs[0].Message, "4111 2222 3333 4444") {
		t.Errorf("Expected credit card to be redacted")
	}

	if strings.Contains(incident.ErrorSignatures[0], "my_secret_token_12345") {
		t.Errorf("Expected token to be redacted in signature")
	}
}
