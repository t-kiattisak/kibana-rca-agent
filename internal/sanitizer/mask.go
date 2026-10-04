package sanitizer

import (
	"regexp"
	"strings"

	"github.com/t-kiattisak/kibana-rca-agent/internal/model"
)

var (
	// Regex patterns for masking sensitive data
	bearerRegex   = regexp.MustCompile(`(?i)(bearer\s+)[a-zA-Z0-9_\-\.]{15,}`)
	passwordRegex = regexp.MustCompile(`(?i)("?(password|secret|token|api_key)"?\s*[:=]\s*["']?)([^"',\s]{4,})(["']?)`)
	cardRegex     = regexp.MustCompile(`\b(?:\d{4}[-\s]?){3}\d{4}\b`)
	emailRegex    = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
)

// MaskString sanitizes credentials, tokens, and PII from raw string
func MaskString(input string) string {
	if input == "" {
		return ""
	}
	s := bearerRegex.ReplaceAllString(input, "${1}[REDACTED_TOKEN]")
	s = passwordRegex.ReplaceAllString(s, "${1}[REDACTED_SECRET]${4}")
	s = cardRegex.ReplaceAllString(s, "[REDACTED_CARD]")
	s = emailRegex.ReplaceAllString(s, "[REDACTED_EMAIL]")
	return s
}

// SanitizeIncident cleanses all log messages, stack traces and signatures before LLM inference
func SanitizeIncident(incident *model.IncidentContext) {
	for i := range incident.ErrorSignatures {
		incident.ErrorSignatures[i] = MaskString(incident.ErrorSignatures[i])
	}

	for i := range incident.CorrelatedLogs {
		log := &incident.CorrelatedLogs[i]
		log.Message = MaskString(log.Message)
		if log.StackTrace != "" {
			log.StackTrace = MaskString(log.StackTrace)
		}
		// Sanitize extra map values if strings
		for k, v := range log.Extra {
			if strVal, ok := v.(string); ok {
				if strings.Contains(strings.ToLower(k), "pass") || strings.Contains(strings.ToLower(k), "token") {
					log.Extra[k] = "[REDACTED]"
				} else {
					log.Extra[k] = MaskString(strVal)
				}
			}
		}
	}
}
