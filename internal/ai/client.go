package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

type Result struct {
	DocID   string  `json:"doc_id"`
	Title   string  `json:"title"`
	Type    string  `json:"type"`
	Text    string  `json:"text"`
	Score   float64 `json:"score"`
	Vector  float64 `json:"vector"`
	Keyword float64 `json:"keyword"`
}

type RetrieveResponse struct {
	Grounded bool     `json:"grounded"`
	TopScore float64  `json:"top_score"`
	Results  []Result `json:"results"`
}

type AnswerResponse struct {
	Grounded  bool     `json:"grounded"`
	Answer    string   `json:"answer"`
	TopScore  float64  `json:"top_score"`
	Citations []string `json:"citations"`
}

type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func NewFromEnv() *Client {
	base := os.Getenv("INFERENCE_URL")
	if base == "" {
		return nil
	}
	return &Client{BaseURL: base, HTTP: &http.Client{Timeout: 20 * time.Second}}
}

func (c *Client) Retrieve(ctx context.Context, tenant, query string, k int, types []string) (RetrieveResponse, error) {
	body, err := json.Marshal(map[string]any{
		"tenant": tenant,
		"query":  query,
		"k":      k,
		"types":  types,
	})
	if err != nil {
		return RetrieveResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/retrieve", bytes.NewReader(body))
	if err != nil {
		return RetrieveResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return RetrieveResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return RetrieveResponse{}, fmt.Errorf("retrieve: %s", resp.Status)
	}
	var out RetrieveResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return RetrieveResponse{}, err
	}
	return out, nil
}

func (c *Client) Answer(ctx context.Context, tenant, query string, k int, lang string) (AnswerResponse, error) {
	body, err := json.Marshal(map[string]any{
		"tenant": tenant,
		"query":  query,
		"k":      k,
		"lang":   lang,
	})
	if err != nil {
		return AnswerResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/answer", bytes.NewReader(body))
	if err != nil {
		return AnswerResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return AnswerResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return AnswerResponse{}, fmt.Errorf("answer: %s", resp.Status)
	}
	var out AnswerResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return AnswerResponse{}, err
	}
	return out, nil
}
