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

type OpenAIClient struct {
    httpClient *http.Client
    cfg        config.Config
}

func NewOpenAIClient(cfg config.Config) *OpenAIClient {
    return &OpenAIClient{httpClient: &http.Client{Timeout: 30 * time.Second}, cfg: cfg}
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
    Choices []struct{ Message struct{ Content string `json:"content"` } `json:"message"` } `json:"choices"`
}

func (c *OpenAIClient) GeneratePlan(ctx context.Context, prompt string) (plan.Plan, error) {
    var zero plan.Plan
    if c.cfg.OpenAIAPIKey == "" {
        return zero, errors.New("missing OPENAI_API_KEY")
    }
    model := c.cfg.Model
    if model == "" {
        model = "gpt-4o-mini"
    }
    body := openaiReq{Model: model}
    body.Messages = []openaiMessage{{Role: "user", Content: prompt}}
    body.ResponseFormat = map[string]string{"type": "json_object"}
    b, _ := json.Marshal(body)
    req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(b))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+c.cfg.OpenAIAPIKey)
    resp, err := c.httpClient.Do(req)
    if err != nil { return zero, err }
    defer resp.Body.Close()
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        data, _ := io.ReadAll(resp.Body)
        return zero, fmt.Errorf("openai http %d: %s", resp.StatusCode, string(data))
    }
    var or openaiResp
    if err := json.NewDecoder(resp.Body).Decode(&or); err != nil { return zero, err }
    if len(or.Choices) == 0 { return zero, errors.New("empty response") }
    text := or.Choices[0].Message.Content
    return plan.TryUnmarshalPlan(text)
}

func (c *OpenAIClient) GenerateErrorFix(ctx context.Context, originalCommand string, errorOutput string, attempt int) (plan.Plan, error) {
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


