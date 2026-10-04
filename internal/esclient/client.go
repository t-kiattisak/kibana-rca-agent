package esclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/t-kiattisak/kibana-rca-agent/internal/model"
)

type Client struct {
	baseURL    string
	index      string
	username   string
	password   string
	httpClient *http.Client
}

func New(baseURL, index, username, password string) *Client {
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		index:    index,
		username: username,
		password: password,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// CheckErrorSpike inspects the last timeWindow for error count
func (c *Client) CheckErrorSpike(ctx context.Context, timeWindow time.Duration, threshold int) (bool, int, error) {
	fromTime := time.Now().UTC().Add(-timeWindow).Format(time.RFC3339Nano)

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"filter": []map[string]interface{}{
					{
						"range": map[string]interface{}{
							"@timestamp": map[string]interface{}{
								"gte": fromTime,
							},
						},
					},
					{
						"bool": map[string]interface{}{
							"should": []map[string]interface{}{
								{"term": map[string]interface{}{"level": "ERROR"}},
								{"range": map[string]interface{}{"http_status": map[string]interface{}{"gte": 500}}},
							},
						},
					},
				},
			},
		},
	}

	data, err := json.Marshal(query)
	if err != nil {
		return false, 0, err
	}

	url := fmt.Sprintf("%s/%s/_count", c.baseURL, c.index)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return false, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return false, 0, fmt.Errorf("ES count error (status %d): %s", resp.StatusCode, string(body))
	}

	var countRes struct {
		Count int `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&countRes); err != nil {
		return false, 0, err
	}

	return countRes.Count >= threshold, countRes.Count, nil
}

// FetchCorrelatedLogs retrieves context logs around the incident window (e.g. -5m to +1m)
func (c *Client) FetchCorrelatedLogs(ctx context.Context, incidentTime time.Time, limit int) (*model.IncidentContext, error) {
	from := incidentTime.Add(-5 * time.Minute).Format(time.RFC3339Nano)
	to := incidentTime.Add(1 * time.Minute).Format(time.RFC3339Nano)

	// Fetch up to limit most relevant logs (both ERRORS and preceding warnings/infos)
	query := map[string]interface{}{
		"size": limit,
		"sort": []map[string]interface{}{
			{"@timestamp": map[string]interface{}{"order": "desc"}},
		},
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"filter": []map[string]interface{}{
					{
						"range": map[string]interface{}{
							"@timestamp": map[string]interface{}{
								"gte": from,
								"lte": to,
							},
						},
					},
				},
			},
		},
	}

	data, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/%s/_search", c.baseURL, c.index)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ES search error (status %d): %s", resp.StatusCode, string(body))
	}

	var searchRes struct {
		Hits struct {
			Total struct {
				Value int `json:"value"`
			} `json:"total"`
			Hits []struct {
				Source model.LogDoc `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&searchRes); err != nil {
		return nil, err
	}

	var (
		serviceName     = "unknown-service"
		sampleTraceID   = ""
		errorSignatures = []string{}
		logs            = make([]model.LogDoc, 0, len(searchRes.Hits.Hits))
		seenErrors      = make(map[string]bool)
	)

	for _, hit := range searchRes.Hits.Hits {
		doc := hit.Source
		logs = append(logs, doc)

		if doc.Service != "" {
			serviceName = doc.Service
		}

		if doc.Level == "ERROR" || doc.HTTPStatus >= 500 {
			if sampleTraceID == "" && doc.TraceID != "" {
				sampleTraceID = doc.TraceID
			}
			sig := doc.Message
			if !seenErrors[sig] {
				seenErrors[sig] = true
				errorSignatures = append(errorSignatures, sig)
			}
		}
	}

	return &model.IncidentContext{
		ServiceName:     serviceName,
		TriggerTime:     incidentTime,
		TotalErrors:     searchRes.Hits.Total.Value,
		SampleTraceID:   sampleTraceID,
		ErrorSignatures: errorSignatures,
		CorrelatedLogs:  logs,
	}, nil
}
