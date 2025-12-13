package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aezizhu/LuciCodex/internal/config"
	"github.com/aezizhu/LuciCodex/internal/llm/prompts"
	"github.com/aezizhu/LuciCodex/internal/plan"
)

type AnthropicClient struct {
	httpClient *http.Client
	cfg        config.Config
}

func NewAnthropicClient(cfg config.Config) *AnthropicClient {
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &AnthropicClient{httpClient: newHTTPClient(cfg, timeout), cfg: cfg}
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicReq struct {
	Model     string             `json:"model"`
	Messages  []anthropicMessage `json:"messages"`
	MaxTokens int                `json:"max_tokens"`
}

type anthropicResp struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
}

func (c *AnthropicClient) GeneratePlan(ctx context.Context, prompt string) (plan.Plan, error) {
	var zero plan.Plan
	if c.cfg.AnthropicAPIKey == "" {
		return zero, errors.New("missing Anthropic API key - configure it in LuCI or set ANTHROPIC_API_KEY environment variable")
	}
	model := c.cfg.Model
	if model == "" {
		model = "claude-haiku-4-5-20251001"
	}
	// Use configured endpoint or default
	endpoint := c.cfg.Endpoint
	if endpoint == "" {
		endpoint = "https://api.anthropic.com/v1"
	}
	// Ensure endpoint ends properly for messages
	url := strings.TrimSuffix(endpoint, "/") + "/messages"

	body := anthropicReq{Model: model, MaxTokens: 2048}
	body.Messages = []anthropicMessage{{Role: "user", Content: prompt}}
	b, err := json.Marshal(body)
	if err != nil {
		return zero, fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return zero, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.cfg.AnthropicAPIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return zero, fmt.Errorf("anthropic http %d: %s", resp.StatusCode, string(data))
	}
	var ar anthropicResp
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return zero, err
	}
	if len(ar.Content) == 0 {
		return zero, errors.New("empty response")
	}
	text := ar.Content[0].Text
	return plan.TryUnmarshalPlan(text)
}

func (c *AnthropicClient) GenerateErrorFix(ctx context.Context, originalCommand string, errorOutput string, attempt int) (plan.Plan, error) {
	prompt := prompts.GenerateErrorFixPrompt(originalCommand, errorOutput, attempt)
	return c.GeneratePlan(ctx, prompt)
}

// Summarize returns summary/details using Anthropic messages API.
func (c *AnthropicClient) Summarize(ctx context.Context, prompt string) (string, []string, error) {
	if c.cfg.AnthropicAPIKey == "" {
		return "", nil, errors.New("missing Anthropic API key - configure it in LuCI or set ANTHROPIC_API_KEY environment variable")
	}
	model := c.cfg.Model
	if model == "" {
		model = "claude-haiku-4-5-20251001"
	}
	endpoint := c.cfg.Endpoint
	if endpoint == "" {
		endpoint = "https://api.anthropic.com/v1"
	}
	url := strings.TrimSuffix(endpoint, "/") + "/messages"

	body := anthropicReq{Model: model, MaxTokens: 1024}
	body.Messages = []anthropicMessage{{Role: "user", Content: prompt}}
	b, err := json.Marshal(body)
	if err != nil {
		return "", nil, fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.cfg.AnthropicAPIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return "", nil, fmt.Errorf("anthropic http %d: %s", resp.StatusCode, string(data))
	}
	var ar anthropicResp
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return "", nil, err
	}
	if len(ar.Content) == 0 {
		return "", nil, errors.New("empty response")
	}
	text := ar.Content[0].Text
	summary, details := parseSummary(text)
	return summary, details, nil
}
