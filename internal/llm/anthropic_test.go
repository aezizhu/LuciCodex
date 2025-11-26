package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aezizhu/LuciCodex/internal/config"
	"github.com/aezizhu/LuciCodex/internal/testutil"
)

func TestAnthropicClient_GeneratePlan_Success(t *testing.T) {
	mockResponse := anthropicResp{
		Content: []struct {
			Text string `json:"text"`
		}{
			{Text: `{"summary": "test plan", "commands": [{"command": ["echo", "hello"]}]}`},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/messages" {
			t.Errorf("expected path /messages, got %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("expected x-api-key header")
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Errorf("expected anthropic-version header")
		}

		var req anthropicReq
		json.NewDecoder(r.Body).Decode(&req)
		if req.Model != "claude-3-haiku-20240307" {
			t.Errorf("expected model claude-3-haiku-20240307, got %s", req.Model)
		}

		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	cfg := config.Config{
		AnthropicAPIKey: "test-key",
		Model:           "claude-3-haiku-20240307",
		Endpoint:        server.URL,
	}

	client := NewAnthropicClient(cfg)
	plan, err := client.GeneratePlan(context.Background(), "test prompt")

	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, plan.Summary, "test plan")
	testutil.AssertEqual(t, len(plan.Commands), 1)
}

func TestAnthropicClient_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": {"message": "invalid key"}}`))
	}))
	defer server.Close()

	cfg := config.Config{
		AnthropicAPIKey: "test-key",
		Endpoint:        server.URL,
	}

	client := NewAnthropicClient(cfg)
	_, err := client.GeneratePlan(context.Background(), "test")

	testutil.AssertError(t, err)
	testutil.AssertContains(t, err.Error(), "anthropic http 401")
	testutil.AssertContains(t, err.Error(), "invalid key")
}

func TestAnthropicClient_GenerateErrorFix(t *testing.T) {
	mockResponse := anthropicResp{
		Content: []struct {
			Text string `json:"text"`
		}{
			{Text: `{"summary": "fix plan", "commands": [{"command": ["fix", "it"]}]}`},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	cfg := config.Config{
		AnthropicAPIKey: "test-key",
		Endpoint:        server.URL,
	}

	client := NewAnthropicClient(cfg)
	plan, err := client.GenerateErrorFix(context.Background(), "cmd", "error", 1)

	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, plan.Summary, "fix plan")
}
