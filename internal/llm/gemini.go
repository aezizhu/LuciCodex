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


