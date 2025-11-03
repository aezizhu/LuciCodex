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

type AnthropicClient struct {
    httpClient *http.Client
    cfg        config.Config
}

func NewAnthropicClient(cfg config.Config) *AnthropicClient {
    return &AnthropicClient{httpClient: &http.Client{Timeout: 30 * time.Second}, cfg: cfg}
}

type anthropicMessage struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

type anthropicReq struct {
    Model     string              `json:"model"`
    Messages  []anthropicMessage  `json:"messages"`
    MaxTokens int                 `json:"max_tokens"`
}

type anthropicResp struct { Content []struct{ Text string `json:"text"` } `json:"content"` }

func (c *AnthropicClient) GeneratePlan(ctx context.Context, prompt string) (plan.Plan, error) {
    var zero plan.Plan
    if c.cfg.AnthropicAPIKey == "" {
        return zero, errors.New("missing ANTHROPIC_API_KEY")
    }
    model := c.cfg.Model
    if model == "" {
        model = "claude-3-5-sonnet-20240620"
    }
    body := anthropicReq{Model: model, MaxTokens: 2048}
    body.Messages = []anthropicMessage{{Role: "user", Content: prompt}}
    b, _ := json.Marshal(body)
    req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(b))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("x-api-key", c.cfg.AnthropicAPIKey)
    req.Header.Set("anthropic-version", "2023-06-01")
    resp, err := c.httpClient.Do(req)
    if err != nil { return zero, err }
    defer resp.Body.Close()
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        data, _ := io.ReadAll(resp.Body)
        return zero, fmt.Errorf("anthropic http %d: %s", resp.StatusCode, string(data))
    }
    var ar anthropicResp
    if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil { return zero, err }
    if len(ar.Content) == 0 { return zero, errors.New("empty response") }
    text := ar.Content[0].Text
    return plan.TryUnmarshalPlan(text)
}

func (c *AnthropicClient) GenerateErrorFix(ctx context.Context, originalCommand string, errorOutput string, attempt int) (plan.Plan, error) {
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


