package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/genai"

	"github.com/t-kiattisak/kibana-rca-agent/internal/role"
)

type DocumentChunk struct {
	ChunkID    string    `json:"chunk_id"`
	DocTitle   string    `json:"doc_title"`
	Category   string    `json:"category"`
	TargetRole string    `json:"target_role"`
	Heading    string    `json:"heading"`
	Content    string    `json:"content"`
	Embedding  []float32 `json:"embedding"`
}

type KnowledgeStore struct {
	esURL      string
	index      string
	genaiCli   *genai.Client
	httpClient *http.Client
}

func NewStore(esURL, index string, genaiCli *genai.Client) *KnowledgeStore {
	return &KnowledgeStore{
		esURL:    strings.TrimRight(esURL, "/"),
		index:    index,
		genaiCli: genaiCli,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GenerateEmbedding calls Gemini Embedding API to generate dense vector (768 dims)
func (s *KnowledgeStore) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	dims := int32(768)
	cfg := &genai.EmbedContentConfig{
		OutputDimensionality: &dims,
	}
	result, err := s.genaiCli.Models.EmbedContent(ctx, "gemini-embedding-001", genai.Text(text), cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	if len(result.Embeddings) == 0 || len(result.Embeddings[0].Values) == 0 {
		return nil, fmt.Errorf("empty embedding returned")
	}

	return result.Embeddings[0].Values, nil
}

// IngestKnowledgeDirectory reads Markdown files, chunks them by headings, and indexes them with embeddings
func (s *KnowledgeStore) IngestKnowledgeDirectory(ctx context.Context, dir string) error {
	files, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
			continue
		}

		filePath := filepath.Join(dir, f.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		chunks := chunkMarkdown(f.Name(), string(data))
		for _, chunk := range chunks {
			// Generate dense vector embedding
			emb, err := s.GenerateEmbedding(ctx, chunk.Heading+": "+chunk.Content)
			if err != nil {
				log.Printf("⚠️ Warning: Failed to embed chunk %s: %v", chunk.ChunkID, err)
				continue
			}
			chunk.Embedding = emb

			// Index into Elasticsearch
			if err := s.indexChunk(ctx, chunk); err != nil {
				log.Printf("⚠️ Warning: Failed to index chunk %s: %v", chunk.ChunkID, err)
			}
		}
	}

	log.Printf("✅ Knowledge base ingestion completed successfully into index [%s]", s.index)
	return nil
}

func (s *KnowledgeStore) indexChunk(ctx context.Context, chunk DocumentChunk) error {
	url := fmt.Sprintf("%s/%s/_doc/%s", s.esURL, s.index, chunk.ChunkID)
	data, err := json.Marshal(chunk)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ES index error (%d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// RetrieveRelevantChunks performs Role-Filtered kNN Search
func (s *KnowledgeStore) RetrieveRelevantChunks(ctx context.Context, queryText string, targetRole string, k int) ([]role.KnowledgeDoc, error) {
	emb, err := s.GenerateEmbedding(ctx, queryText)
	if err != nil {
		return nil, err
	}

	query := map[string]interface{}{
		"knn": map[string]interface{}{
			"field":         "embedding",
			"query_vector":  emb,
			"k":             k,
			"num_candidates": 20,
			"filter": map[string]interface{}{
				"term": map[string]interface{}{
					"target_role": targetRole,
				},
			},
		},
	}

	data, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/%s/_search", s.esURL, s.index)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var searchRes struct {
		Hits struct {
			Hits []struct {
				Source DocumentChunk `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&searchRes); err != nil {
		return nil, err
	}

	var results []role.KnowledgeDoc
	for _, hit := range searchRes.Hits.Hits {
		results = append(results, role.KnowledgeDoc{
			Title:    fmt.Sprintf("%s (%s)", hit.Source.DocTitle, hit.Source.Heading),
			Category: hit.Source.Category,
			Role:     hit.Source.TargetRole,
			Content:  hit.Source.Content,
		})
	}

	return results, nil
}

// chunkMarkdown divides a document into semantic chunks based on markdown headings
func chunkMarkdown(docTitle, content string) []DocumentChunk {
	var chunks []DocumentChunk
	lines := strings.Split(content, "\n")

	var currentHeading = "Overview"
	var currentSection strings.Builder
	var targetRole = "developer"
	var category = "architecture"

	// Parse header metadata if present
	for _, line := range lines {
		if strings.Contains(line, "Target Role: sre") {
			targetRole = "sre"
			category = "infrastructure"
		} else if strings.Contains(line, "Target Role: product") {
			targetRole = "product"
			category = "business_policy"
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "# ") {
			if currentSection.Len() > 0 {
				chunks = append(chunks, DocumentChunk{
					ChunkID:    fmt.Sprintf("%s-%s", docTitle, sanitizeHeading(currentHeading)),
					DocTitle:   docTitle,
					Category:   category,
					TargetRole: targetRole,
					Heading:    currentHeading,
					Content:    strings.TrimSpace(currentSection.String()),
				})
				currentSection.Reset()
			}
			currentHeading = strings.TrimPrefix(trimmed, "## ")
			currentHeading = strings.TrimPrefix(currentHeading, "# ")
			continue
		}
		currentSection.WriteString(line + "\n")
	}

	if currentSection.Len() > 0 {
		chunks = append(chunks, DocumentChunk{
			ChunkID:    fmt.Sprintf("%s-%s", docTitle, sanitizeHeading(currentHeading)),
			DocTitle:   docTitle,
			Category:   category,
			TargetRole: targetRole,
			Heading:    currentHeading,
			Content:    strings.TrimSpace(currentSection.String()),
		})
	}

	return chunks
}

func sanitizeHeading(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "&", "and")
	s = strings.ReplaceAll(s, "/", "-")
	return s
}
