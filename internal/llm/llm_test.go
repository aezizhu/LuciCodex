package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aezizhu/LuciCodex/internal/config"
	"github.com/aezizhu/LuciCodex/internal/plan"
)

func TestNewGeminiClient(t *testing.T) {
	cfg := config.Config{
		APIKey:   "test-key",
		Model:    "gemini-1.5-flash",
		Endpoint: "https://test.example.com",
	}

	client := NewGeminiClient(cfg)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.cfg.APIKey != "test-key" {
		t.Errorf("expected API key 'test-key', got %q", client.cfg.APIKey)
	}
	if client.httpClient == nil {
		t.Error("expected non-nil http client")
	}
	
	var _ plan.Plan
}

func TestGeminiClient_GeneratePlan_Success(t *testing.T) {
	mockResponse := generateContentResponse{
		Candidates: []struct {
			Content content `json:"content"`
		}{
			{
				Content: content{
					Parts: []part{
						{
							Text: `{"summary":"Test plan","commands":[{"command":["uci","show","network"],"description":"Show network"}],"warnings":[]}`,
						},
					},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST request, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		var req generateContentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	cfg := config.Config{
		APIKey:   "test-key",
		Model:    "gemini-1.5-flash",
		Endpoint: server.URL,
	}

	client := NewGeminiClient(cfg)
	ctx := context.Background()

	p, err := client.GeneratePlan(ctx, "test prompt")
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}

	if p.Summary != "Test plan" {
		t.Errorf("expected summary 'Test plan', got %q", p.Summary)
	}
	if len(p.Commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(p.Commands))
	}
	if p.Commands[0].Command[0] != "uci" {
		t.Errorf("expected command 'uci', got %q", p.Commands[0].Command[0])
	}
}

func TestGeminiClient_GeneratePlan_MissingAPIKey(t *testing.T) {
	cfg := config.Config{
		APIKey:   "",
		Model:    "gemini-1.5-flash",
		Endpoint: "https://test.example.com",
	}

	client := NewGeminiClient(cfg)
	ctx := context.Background()

	_, err := client.GeneratePlan(ctx, "test prompt")
	if err == nil {
		t.Error("expected error for missing API key")
	}
	if err.Error() != "missing API key" {
		t.Errorf("expected 'missing API key' error, got %q", err.Error())
	}
}

func TestGeminiClient_GeneratePlan_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Invalid API key"))
	}))
	defer server.Close()

	cfg := config.Config{
		APIKey:   "invalid-key",
		Model:    "gemini-1.5-flash",
		Endpoint: server.URL,
	}

	client := NewGeminiClient(cfg)
	ctx := context.Background()

	_, err := client.GeneratePlan(ctx, "test prompt")
	if err == nil {
		t.Error("expected error for HTTP 401")
	}
}

func TestGeminiClient_GeneratePlan_EmptyResponse(t *testing.T) {
	mockResponse := generateContentResponse{
		Candidates: []struct {
			Content content `json:"content"`
		}{},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	cfg := config.Config{
		APIKey:   "test-key",
		Model:    "gemini-1.5-flash",
		Endpoint: server.URL,
	}

	client := NewGeminiClient(cfg)
	ctx := context.Background()

	_, err := client.GeneratePlan(ctx, "test prompt")
	if err == nil {
		t.Error("expected error for empty response")
	}
	if err.Error() != "empty response" {
		t.Errorf("expected 'empty response' error, got %q", err.Error())
	}
}

func TestGeminiClient_GeneratePlan_InvalidJSON(t *testing.T) {
	mockResponse := generateContentResponse{
		Candidates: []struct {
			Content content `json:"content"`
		}{
			{
				Content: content{
					Parts: []part{
						{
							Text: "This is not valid JSON",
						},
					},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	cfg := config.Config{
		APIKey:   "test-key",
		Model:    "gemini-1.5-flash",
		Endpoint: server.URL,
	}

	client := NewGeminiClient(cfg)
	ctx := context.Background()

	_, err := client.GeneratePlan(ctx, "test prompt")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestGeminiClient_GeneratePlan_WrappedJSON(t *testing.T) {
	mockResponse := generateContentResponse{
		Candidates: []struct {
			Content content `json:"content"`
		}{
			{
				Content: content{
					Parts: []part{
						{
							Text: `Here is the plan: {"summary":"Wrapped plan","commands":[{"command":["echo","test"]}]} - that's it!`,
						},
					},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	cfg := config.Config{
		APIKey:   "test-key",
		Model:    "gemini-1.5-flash",
		Endpoint: server.URL,
	}

	client := NewGeminiClient(cfg)
	ctx := context.Background()

	p, err := client.GeneratePlan(ctx, "test prompt")
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}

	if p.Summary != "Wrapped plan" {
		t.Errorf("expected summary 'Wrapped plan', got %q", p.Summary)
	}
	if len(p.Commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(p.Commands))
	}
}

func TestGeminiClient_GeneratePlan_DefaultModel(t *testing.T) {
	mockResponse := generateContentResponse{
		Candidates: []struct {
			Content content `json:"content"`
		}{
			{
				Content: content{
					Parts: []part{
						{
							Text: `{"commands":[{"command":["test"]}]}`,
						},
					},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !contains(r.URL.String(), "gemini-1.5-flash") {
			t.Error("expected default model 'gemini-1.5-flash' in URL")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	cfg := config.Config{
		APIKey:   "test-key",
		Model:    "",
		Endpoint: server.URL,
	}

	client := NewGeminiClient(cfg)
	ctx := context.Background()

	_, err := client.GeneratePlan(ctx, "test prompt")
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}
}

func TestGeminiClient_GeneratePlan_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config.Config{
		APIKey:   "test-key",
		Model:    "gemini-1.5-flash",
		Endpoint: server.URL,
	}

	client := NewGeminiClient(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := client.GeneratePlan(ctx, "test prompt")
	if err == nil {
		t.Error("expected error for context cancellation")
	}
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "pure JSON",
			input:    `{"key":"value"}`,
			expected: `{"key":"value"}`,
		},
		{
			name:     "JSON with prefix",
			input:    `Here is the JSON: {"key":"value"}`,
			expected: `{"key":"value"}`,
		},
		{
			name:     "JSON with suffix",
			input:    `{"key":"value"} - that's it`,
			expected: `{"key":"value"}`,
		},
		{
			name:     "JSON with both",
			input:    `Prefix {"key":"value"} suffix`,
			expected: `{"key":"value"}`,
		},
		{
			name:     "nested JSON",
			input:    `{"outer":{"inner":"value"}}`,
			expected: `{"outer":{"inner":"value"}}`,
		},
		{
			name:     "no JSON",
			input:    `no json here`,
			expected: `no json here`,
		},
		{
			name:     "multiple JSON objects",
			input:    `{"first":"obj"} and {"second":"obj"}`,
			expected: `{"first":"obj"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractJSON(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestNewOpenAIClient(t *testing.T) {
	cfg := config.Config{
		OpenAIAPIKey: "test-key",
		Model:        "gpt-4",
	}

	client := NewOpenAIClient(cfg)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.cfg.OpenAIAPIKey != "test-key" {
		t.Errorf("expected API key 'test-key', got %q", client.cfg.OpenAIAPIKey)
	}
}

func TestNewAnthropicClient(t *testing.T) {
	cfg := config.Config{
		AnthropicAPIKey: "test-key",
		Model:           "claude-3",
	}

	client := NewAnthropicClient(cfg)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.cfg.AnthropicAPIKey != "test-key" {
		t.Errorf("expected API key 'test-key', got %q", client.cfg.AnthropicAPIKey)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
