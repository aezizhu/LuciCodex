package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/aezizhu/LuciCodex/internal/config"
	"github.com/aezizhu/LuciCodex/internal/llm/prompts"
	"github.com/aezizhu/LuciCodex/internal/plan"
)

type GeminiClient struct {
	httpClient *http.Client
	cfg        config.Config
}

func NewGeminiClient(cfg config.Config) *GeminiClient {
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	// Ensure minimum 60 seconds for Gemini calls (complex prompts can take time)
	if timeout < 60*time.Second {
		timeout = 60 * time.Second
	}
	return &GeminiClient{
		httpClient: newHTTPClient(cfg, timeout),
		cfg:        cfg,
	}
}

// API request/response shapes (minimal for our use)
type generateContentRequest struct {
	Contents []content         `json:"contents"`
	Config   *generationConfig `json:"generationConfig,omitempty"`
}

type generationConfig struct {
	ResponseMimeType string `json:"response_mime_type,omitempty"`
}

type content struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}

type part struct {
	Text string `json:"text,omitempty"`
}

type generateContentResponse struct {
	Candidates []struct {
		Content content `json:"content"`
	} `json:"candidates"`
	PromptFeedback any `json:"promptFeedback,omitempty"`
}

func (c *GeminiClient) GeneratePlan(ctx context.Context, prompt string) (plan.Plan, error) {
	var zero plan.Plan
	if c.cfg.APIKey == "" {
		return zero, NewAPIError("gemini", 0, "missing API key - configure in LuCI or set GEMINI_API_KEY", ErrNoAPIKey)
	}
	model := c.cfg.Model
	if model == "" {
		model = "gemini-3-flash"
	}
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", c.cfg.Endpoint, model, c.cfg.APIKey)

	reqBody := generateContentRequest{
		Contents: []content{{
			Role:  "user",
			Parts: []part{{Text: prompt}},
		}},
		Config: &generationConfig{ResponseMimeType: "application/json"},
	}
	b, err := json.Marshal(reqBody)
	if err != nil {
		return zero, NewAPIError("gemini", 0, "failed to marshal request", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return zero, NewAPIError("gemini", 0, "failed to create request", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return zero, NewAPIError("gemini", 0, "request cancelled", ErrContextCancelled)
		}
		return zero, NewAPIError("gemini", 0, "request failed", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data := readErrorBody(resp.Body)
		return zero, NewAPIError("gemini", resp.StatusCode, string(data), ErrRequestFailed)
	}

	var gcr generateContentResponse
	if err := json.NewDecoder(resp.Body).Decode(&gcr); err != nil {
		return zero, NewParseError("gemini", "response decoding", "", err)
	}
	if len(gcr.Candidates) == 0 || len(gcr.Candidates[0].Content.Parts) == 0 {
		return zero, NewAPIError("gemini", 0, "empty response from API", ErrInvalidResponse)
	}
	text := gcr.Candidates[0].Content.Parts[0].Text
	p, err := plan.TryUnmarshalPlan(text)
	if err != nil {
		return zero, NewParseError("gemini", "plan extraction", text, err)
	}
	return p, nil
}

func (c *GeminiClient) GenerateErrorFix(ctx context.Context, originalCommand string, errorOutput string, attempt int) (plan.Plan, error) {
	prompt := prompts.GenerateErrorFixPrompt(originalCommand, errorOutput, attempt)
	return c.GeneratePlan(ctx, prompt)
}

// Summarize returns summary/details using the active Gemini model.
func (c *GeminiClient) Summarize(ctx context.Context, prompt string) (string, []string, error) {
	if c.cfg.APIKey == "" {
		return "", nil, NewAPIError("gemini", 0, "missing API key - configure in LuCI or set GEMINI_API_KEY", ErrNoAPIKey)
	}
	model := c.cfg.Model
	if model == "" {
		model = "gemini-3-flash"
	}
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", c.cfg.Endpoint, model, c.cfg.APIKey)

	reqBody := generateContentRequest{
		Contents: []content{{
			Role:  "user",
			Parts: []part{{Text: prompt}},
		}},
		Config: &generationConfig{ResponseMimeType: "application/json"},
	}
	b, err := json.Marshal(reqBody)
	if err != nil {
		return "", nil, NewAPIError("gemini", 0, "failed to marshal request", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return "", nil, NewAPIError("gemini", 0, "failed to create request", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return "", nil, NewAPIError("gemini", 0, "request cancelled", ErrContextCancelled)
		}
		return "", nil, NewAPIError("gemini", 0, "request failed", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data := readErrorBody(resp.Body)
		return "", nil, NewAPIError("gemini", resp.StatusCode, string(data), ErrRequestFailed)
	}

	var gcr generateContentResponse
	if err := json.NewDecoder(resp.Body).Decode(&gcr); err != nil {
		return "", nil, NewParseError("gemini", "response decoding", "", err)
	}
	if len(gcr.Candidates) == 0 || len(gcr.Candidates[0].Content.Parts) == 0 {
		return "", nil, NewAPIError("gemini", 0, "empty response from API", ErrInvalidResponse)
	}
	text := gcr.Candidates[0].Content.Parts[0].Text
	summary, details := parseSummary(text)
	return summary, details, nil
}
