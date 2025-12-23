package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aezizhu/LuciCodex/internal/config"
	"github.com/aezizhu/LuciCodex/internal/llm/prompts"
	"github.com/aezizhu/LuciCodex/internal/plan"
)

type OpenAIClient struct {
	httpClient *http.Client
	cfg        config.Config
}

func NewOpenAIClient(cfg config.Config) *OpenAIClient {
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &OpenAIClient{httpClient: newHTTPClient(cfg, timeout), cfg: cfg}
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiReq struct {
	Model          string            `json:"model"`
	Messages       []openaiMessage   `json:"messages"`
	ResponseFormat map[string]string `json:"response_format,omitempty"`
}

type openaiResp struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type openaiSummary struct {
	Summary string   `json:"summary"`
	Details []string `json:"details,omitempty"`
	Status  string   `json:"status,omitempty"`
}

func (c *OpenAIClient) GeneratePlan(ctx context.Context, prompt string) (plan.Plan, error) {
	var zero plan.Plan
	if c.cfg.OpenAIAPIKey == "" {
		return zero, errors.New("missing OpenAI API key - configure it in LuCI or set OPENAI_API_KEY environment variable")
	}
	model := c.cfg.Model
	if model == "" {
		model = "gpt-4o-mini"
	}
	// Use configured endpoint or default
	endpoint := c.cfg.Endpoint
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1"
	}
	// Ensure endpoint ends properly for chat completions
	url := strings.TrimSuffix(endpoint, "/") + "/chat/completions"

	body := openaiReq{Model: model}
	body.Messages = []openaiMessage{{Role: "user", Content: prompt}}
	body.ResponseFormat = map[string]string{"type": "json_object"}
	b, err := json.Marshal(body)
	if err != nil {
		return zero, fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return zero, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.OpenAIAPIKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data := readErrorBody(resp.Body)
		return zero, fmt.Errorf("openai http %d: %s", resp.StatusCode, string(data))
	}
	var or openaiResp
	if err := json.NewDecoder(resp.Body).Decode(&or); err != nil {
		return zero, err
	}
	if len(or.Choices) == 0 {
		return zero, errors.New("empty response")
	}
	text := or.Choices[0].Message.Content
	return plan.TryUnmarshalPlan(text)
}

func (c *OpenAIClient) GenerateErrorFix(ctx context.Context, originalCommand string, errorOutput string, attempt int) (plan.Plan, error) {
	prompt := prompts.GenerateErrorFixPrompt(originalCommand, errorOutput, attempt)
	return c.GeneratePlan(ctx, prompt)
}

// Summarize sends a summarization prompt and returns the summary plus optional detail bullets.
func (c *OpenAIClient) Summarize(ctx context.Context, prompt string) (string, []string, error) {
	if c.cfg.OpenAIAPIKey == "" {
		return "", nil, errors.New("missing OpenAI API key - configure it in LuCI or set OPENAI_API_KEY environment variable")
	}

	model := c.cfg.Model
	if model == "" {
		model = "gpt-4o-mini"
	}

	endpoint := c.cfg.Endpoint
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1"
	}
	url := strings.TrimSuffix(endpoint, "/") + "/chat/completions"

	body := openaiReq{
		Model:          model,
		Messages:       []openaiMessage{{Role: "user", Content: prompt}},
		ResponseFormat: map[string]string{"type": "json_object"},
	}

	b, err := json.Marshal(body)
	if err != nil {
		return "", nil, fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.OpenAIAPIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data := readErrorBody(resp.Body)
		return "", nil, fmt.Errorf("openai http %d: %s", resp.StatusCode, string(data))
	}

	var or openaiResp
	if err := json.NewDecoder(resp.Body).Decode(&or); err != nil {
		return "", nil, err
	}
	if len(or.Choices) == 0 {
		return "", nil, errors.New("empty response")
	}

	text := or.Choices[0].Message.Content
	var parsed openaiSummary
	if err := json.Unmarshal([]byte(text), &parsed); err == nil && parsed.Summary != "" {
		return parsed.Summary, parsed.Details, nil
	}

	// Fallback: return raw text if JSON parsing failed
	return text, nil, nil
}
