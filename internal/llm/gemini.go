package llm

import (
    "bytes"
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "net/http"
    "time"

    "github.com/aezizhu/LuciCodex/internal/config"
    "github.com/aezizhu/LuciCodex/internal/plan"
)

type GeminiClient struct {
    httpClient *http.Client
    cfg        config.Config
}

func NewGeminiClient(cfg config.Config) *GeminiClient {
    return &GeminiClient{
        httpClient: &http.Client{Timeout: 30 * time.Second},
        cfg:        cfg,
    }
}

// API request/response shapes (minimal for our use)
type generateContentRequest struct {
    Contents []content               `json:"contents"`
    Config   *generationConfig       `json:"generationConfig,omitempty"`
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
        return zero, errors.New("missing API key")
    }
    model := c.cfg.Model
    if model == "" {
        model = "gemini-1.5-flash"
    }
    url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", c.cfg.Endpoint, model, c.cfg.APIKey)

    reqBody := generateContentRequest{
        Contents: []content{{
            Role:  "user",
            Parts: []part{{Text: prompt}},
        }},
        Config: &generationConfig{ResponseMimeType: "application/json"},
    }
    b, _ := json.Marshal(reqBody)

    httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
    if err != nil {
        return zero, err
    }
    httpReq.Header.Set("Content-Type", "application/json")

    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return zero, err
    }
    defer resp.Body.Close()
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        data, _ := io.ReadAll(resp.Body)
        return zero, fmt.Errorf("gemini http %d: %s", resp.StatusCode, string(data))
    }

    var gcr generateContentResponse
    if err := json.NewDecoder(resp.Body).Decode(&gcr); err != nil {
        return zero, err
    }
    if len(gcr.Candidates) == 0 || len(gcr.Candidates[0].Content.Parts) == 0 {
        return zero, errors.New("empty response")
    }
    text := gcr.Candidates[0].Content.Parts[0].Text
    p, err := plan.TryUnmarshalPlan(text)
    if err != nil {
        // try to extract JSON if wrapped in text
        var p2 plan.Plan
        if json.Unmarshal([]byte(extractJSON(text)), &p2) == nil && len(p2.Commands) > 0 {
            return p2, nil
        }
        return zero, fmt.Errorf("failed to parse plan: %w", err)
    }
    return p, nil
}

func (c *GeminiClient) GenerateErrorFix(ctx context.Context, originalCommand string, errorOutput string, attempt int) (plan.Plan, error) {
    prompt := fmt.Sprintf(`You are a router command error fixer for OpenWrt systems.

The following command failed:
Command: %s
Error output: %s
Attempt: %d

Analyze the error and provide a corrected plan to fix the issue. Output strict JSON:
{
  "summary": "brief explanation of the fix",
  "commands": [ { "command": [string, ...], "description": string, "needs_root": bool } ],
  "warnings": [string]
}

Rules:
- Analyze the error carefully (file not found, permission denied, syntax error, etc.)
- Provide alternative commands or fixes
- Use OpenWrt tools: uci, ubus, fw4, opkg, logread, wifi, /etc/init.d/*
- For permission errors, set needs_root to true
- For file not found, check alternative paths or suggest installation
- For syntax errors, correct the command syntax
- Keep the fix minimal and directly actionable
- Common OpenWrt paths: /etc/config/, /var/log/, /sys/class/net/`, originalCommand, errorOutput, attempt)

    return c.GeneratePlan(ctx, prompt)
}

func extractJSON(s string) string {
    start := -1
    depth := 0
    for i, ch := range s {
        if ch == '{' {
            if depth == 0 {
                start = i
            }
            depth++
        } else if ch == '}' {
            depth--
            if depth == 0 && start >= 0 {
                return s[start : i+1]
            }
        }
    }
    return s
}


