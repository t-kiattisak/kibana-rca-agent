package role

import (
	"context"
	"strings"
	"time"

	"google.golang.org/genai"

	"github.com/t-kiattisak/kibana-rca-agent/internal/model"
)

// ExpertAgent defines the common contract for specialized incident response roles
type ExpertAgent interface {
	Name() string
	RoleTitle() string
	Analyze(ctx context.Context, incident *model.IncidentContext, docs []KnowledgeDoc) (*model.RolePerspective, error)
}

// KnowledgeDoc represents domain-specific technical or business documentation
type KnowledgeDoc struct {
	Title    string `json:"title"`
	Category string `json:"category"`
	Role     string `json:"role"`
	Content  string `json:"content"`
}

// GenerateWithRetry executes Gemini API calls with exponential backoff on 503 high demand spikes
func GenerateWithRetry(ctx context.Context, client *genai.Client, model string, prompt string, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
	var result *genai.GenerateContentResponse
	var err error
	for attempt := 1; attempt <= 4; attempt++ {
		result, err = client.Models.GenerateContent(ctx, model, genai.Text(prompt), config)
		if err == nil {
			return result, nil
		}
		if strings.Contains(err.Error(), "503") || strings.Contains(err.Error(), "high demand") {
			time.Sleep(time.Duration(attempt*2) * time.Second)
			continue
		}
		return nil, err
	}
	return nil, err
}
