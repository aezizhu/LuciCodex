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

func TestOpenAIClient_GeneratePlan_Success(t *testing.T) {
	mockResponse := openaiResp{
		Choices: []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}{
			{
				Message: struct {
					Content string `json:"content"`
				}{
					Content: `{"summary": "test plan", "commands": [{"command": ["echo", "hello"]}]}`,
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("expected path /chat/completions, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Authorization header")
		}

		var req openaiReq
		json.NewDecoder(r.Body).Decode(&req)
		if req.Model != "gpt-4o-mini" {
			t.Errorf("expected model gpt-4o-mini, got %s", req.Model)
		}

		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	cfg := config.Config{
		OpenAIAPIKey: "test-key",
		Model:        "gpt-4o-mini",
		Endpoint:     server.URL,
	}

	client := NewOpenAIClient(cfg)
	plan, err := client.GeneratePlan(context.Background(), "test prompt")

	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, plan.Summary, "test plan")
	testutil.AssertEqual(t, len(plan.Commands), 1)
}

func TestOpenAIClient_GenerateErrorFix(t *testing.T) {
	mockResponse := openaiResp{
		Choices: []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}{
			{
				Message: struct {
					Content string `json:"content"`
				}{
					Content: `{"summary": "fix plan", "commands": [{"command": ["fix", "it"]}]}`,
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	cfg := config.Config{
		OpenAIAPIKey: "test-key",
		Endpoint:     server.URL,
	}

	client := NewOpenAIClient(cfg)
	plan, err := client.GenerateErrorFix(context.Background(), "cmd", "error", 1)

	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, plan.Summary, "fix plan")
}

func TestOpenAIClient_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": {"message": "invalid key"}}`))
	}))
	defer server.Close()

	cfg := config.Config{
		OpenAIAPIKey: "test-key",
		Endpoint:     server.URL,
	}

	client := NewOpenAIClient(cfg)
	_, err := client.GeneratePlan(context.Background(), "test")

	testutil.AssertError(t, err)
	testutil.AssertContains(t, err.Error(), "openai http 400")
	testutil.AssertContains(t, err.Error(), "invalid key")
}

func TestOpenAIClient_EdgeCases(t *testing.T) {
	// 1. Missing API Key
	client := NewOpenAIClient(config.Config{})
	_, err := client.GeneratePlan(context.Background(), "test")
	testutil.AssertError(t, err)
	testutil.AssertContains(t, err.Error(), "missing OpenAI API key")

	// 2. HTTP Client Error (Connection Refused)
	client = NewOpenAIClient(config.Config{
		OpenAIAPIKey: "key",
		Endpoint:     "http://127.0.0.1:1", // Should fail
	})
	_, err = client.GeneratePlan(context.Background(), "test")
	testutil.AssertError(t, err)

	// 3. Invalid JSON Response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	client = NewOpenAIClient(config.Config{
		OpenAIAPIKey: "key",
		Endpoint:     server.URL,
	})
	_, err = client.GeneratePlan(context.Background(), "test")
	testutil.AssertError(t, err)

	// 4. Empty Response Content
	serverEmpty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"choices": []}`))
	}))
	defer serverEmpty.Close()

	client = NewOpenAIClient(config.Config{
		OpenAIAPIKey: "key",
		Endpoint:     serverEmpty.URL,
	})
	_, err = client.GeneratePlan(context.Background(), "test")
	testutil.AssertError(t, err)
	testutil.AssertContains(t, err.Error(), "empty response")
}
