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

    "github.com/aezizhu/g/internal/config"
    "github.com/aezizhu/g/internal/plan"
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


