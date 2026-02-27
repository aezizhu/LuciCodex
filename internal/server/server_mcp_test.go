package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aezizhu/LuciCodex/internal/config"
)

func TestRateLimiter_Allow(t *testing.T) {
	rl := newRateLimiter(3, 1)

	// Should allow first 3 requests (burst)
	for i := 0; i < 3; i++ {
		if !rl.allow() {
			t.Errorf("expected request %d to be allowed", i+1)
		}
	}

	// 4th request should be denied (exhausted burst)
	if rl.allow() {
		t.Error("expected request to be rate limited")
	}
}

func TestRateLimiter_Refill(t *testing.T) {
	rl := newRateLimiter(2, 10)

	// Exhaust all tokens
	rl.allow()
	rl.allow()
	if rl.allow() {
		t.Error("expected rate limit after exhausting tokens")
	}

	// Simulate time passing for refill
	rl.mu.Lock()
	rl.lastTime = time.Now().Add(-1 * time.Second)
	rl.mu.Unlock()

	// Should have refilled
	if !rl.allow() {
		t.Error("expected token after refill")
	}
}

func TestRateLimiter_MaxCap(t *testing.T) {
	rl := newRateLimiter(5, 100)

	// Simulate long time passing
	rl.mu.Lock()
	rl.lastTime = time.Now().Add(-10 * time.Second)
	rl.tokens = 0
	rl.mu.Unlock()

	// Refill should cap at max
	rl.allow() // triggers refill
	rl.mu.Lock()
	remaining := rl.tokens
	rl.mu.Unlock()

	if remaining > 5 {
		t.Errorf("tokens %d exceeded max cap 5", remaining)
	}
}

func TestMergeConfig(t *testing.T) {
	baseCfg := config.Config{
		Provider: "gemini",
		Model:    "gemini-3-flash",
		APIKey:   "base-gemini-key",
	}
	s := &Server{cfg: baseCfg}

	// Test provider override
	merged := s.mergeConfig("openai", "gpt-4", map[string]string{
		"openai_key": "new-openai-key",
	})

	if merged.Provider != "openai" {
		t.Errorf("expected provider openai, got %s", merged.Provider)
	}
	if merged.OpenAIAPIKey != "new-openai-key" {
		t.Errorf("expected openai key to be set")
	}

	// Test empty overrides preserve base
	merged2 := s.mergeConfig("", "", nil)
	if merged2.Provider != "gemini" {
		t.Errorf("expected provider gemini preserved, got %s", merged2.Provider)
	}
	if merged2.APIKey != "base-gemini-key" {
		t.Errorf("expected gemini key preserved")
	}
}

func TestMergeConfig_AllProviderKeys(t *testing.T) {
	s := &Server{cfg: config.Config{Provider: "gemini"}}

	merged := s.mergeConfig("anthropic", "claude-3", map[string]string{
		"gemini_key":    "g-key",
		"openai_key":    "o-key",
		"anthropic_key": "a-key",
	})

	if merged.APIKey != "g-key" {
		t.Errorf("expected gemini key set, got %q", merged.APIKey)
	}
	if merged.OpenAIAPIKey != "o-key" {
		t.Errorf("expected openai key set, got %q", merged.OpenAIAPIKey)
	}
	if merged.AnthropicAPIKey != "a-key" {
		t.Errorf("expected anthropic key set, got %q", merged.AnthropicAPIKey)
	}
}

func TestSanitizeConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
		excludes string
	}{
		{
			name:     "redacts password",
			input:    "\toption password 'mysecretpass'",
			contains: "<REDACTED>",
			excludes: "mysecretpass",
		},
		{
			name:     "redacts psk",
			input:    "\toption psk 'wifipassword123'",
			contains: "<REDACTED>",
			excludes: "wifipassword123",
		},
		{
			name:     "redacts key",
			input:    "\toption key 'some_secret_key'",
			contains: "<REDACTED>",
			excludes: "some_secret_key",
		},
		{
			name:     "preserves non-sensitive",
			input:    "\toption proto 'dhcp'",
			contains: "dhcp",
		},
		{
			name:     "preserves config structure",
			input:    "config interface 'lan'\n\toption proto 'static'\n\toption password 'secret123'",
			contains: "config interface",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeConfig(tt.input)
			if tt.contains != "" {
				if !bytes.Contains([]byte(result), []byte(tt.contains)) {
					t.Errorf("expected output to contain %q, got %q", tt.contains, result)
				}
			}
			if tt.excludes != "" {
				if bytes.Contains([]byte(result), []byte(tt.excludes)) {
					t.Errorf("expected output to not contain %q, got %q", tt.excludes, result)
				}
			}
		})
	}
}

func TestServer_Execute_InvalidMethod(t *testing.T) {
	cfg := config.Config{}
	s := New(cfg)

	req, _ := http.NewRequest("GET", "/v1/execute", nil)
	req.Header.Set("X-Auth-Token", s.GetToken())
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestServer_Execute_EmptyBody(t *testing.T) {
	cfg := config.Config{}
	s := New(cfg)

	req, _ := http.NewRequest("POST", "/v1/execute", bytes.NewReader([]byte{}))
	req.Header.Set("X-Auth-Token", s.GetToken())
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestServer_Summarize_InvalidMethod(t *testing.T) {
	cfg := config.Config{}
	s := New(cfg)

	req, _ := http.NewRequest("GET", "/v1/summarize", nil)
	req.Header.Set("X-Auth-Token", s.GetToken())
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestServer_Summarize_MissingCommands(t *testing.T) {
	cfg := config.Config{}
	s := New(cfg)

	body := []byte(`{"prompt": "test", "commands": []}`)
	req, _ := http.NewRequest("POST", "/v1/summarize", bytes.NewReader(body))
	req.Header.Set("X-Auth-Token", s.GetToken())
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestServer_BearerAuth(t *testing.T) {
	cfg := config.Config{}
	s := New(cfg)

	body := []byte(`{"prompt": "test"}`)
	req, _ := http.NewRequest("POST", "/v1/plan", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+s.GetToken())
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	// Should not be 401 — the Bearer auth should work
	if rr.Code == http.StatusUnauthorized {
		t.Error("Bearer auth should have been accepted")
	}
}

func TestServer_InvalidBearerAuth(t *testing.T) {
	cfg := config.Config{}
	s := New(cfg)

	body := []byte(`{"prompt": "test"}`)
	req, _ := http.NewRequest("POST", "/v1/plan", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer wrong-token")
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestServer_MCP_InvalidMethod(t *testing.T) {
	cfg := config.Config{}
	s := New(cfg)

	req, _ := http.NewRequest("GET", "/v1/mcp", nil)
	req.Header.Set("X-Auth-Token", s.GetToken())
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestServer_MCP_Initialize(t *testing.T) {
	cfg := config.Config{}
	s := New(cfg)

	mcpReq := MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	}
	body, _ := json.Marshal(mcpReq)

	req, _ := http.NewRequest("POST", "/v1/mcp", bytes.NewReader(body))
	req.Header.Set("X-Auth-Token", s.GetToken())
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	var resp MCPResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error != nil {
		t.Errorf("unexpected error: %s", resp.Error.Message)
	}
	if resp.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %s", resp.JSONRPC)
	}
}

func TestServer_MCP_Ping(t *testing.T) {
	cfg := config.Config{}
	s := New(cfg)

	mcpReq := MCPRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "ping",
	}
	body, _ := json.Marshal(mcpReq)

	req, _ := http.NewRequest("POST", "/v1/mcp", bytes.NewReader(body))
	req.Header.Set("X-Auth-Token", s.GetToken())
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	var resp MCPResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Error != nil {
		t.Errorf("ping should succeed, got error: %s", resp.Error.Message)
	}
}

func TestServer_MCP_ListTools(t *testing.T) {
	cfg := config.Config{}
	s := New(cfg)

	mcpReq := MCPRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/list",
	}
	body, _ := json.Marshal(mcpReq)

	req, _ := http.NewRequest("POST", "/v1/mcp", bytes.NewReader(body))
	req.Header.Set("X-Auth-Token", s.GetToken())
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	var resp MCPResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Error != nil {
		t.Errorf("list tools should succeed, got error: %s", resp.Error.Message)
	}

	// Result should contain tools
	resultMap, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("expected result to be a map")
	}
	tools, ok := resultMap["tools"]
	if !ok {
		t.Fatal("expected tools key in result")
	}
	toolList, ok := tools.([]interface{})
	if !ok {
		t.Fatal("expected tools to be an array")
	}
	if len(toolList) == 0 {
		t.Error("expected at least one tool")
	}
}

func TestServer_MCP_ListResources(t *testing.T) {
	cfg := config.Config{}
	s := New(cfg)

	mcpReq := MCPRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "resources/list",
	}
	body, _ := json.Marshal(mcpReq)

	req, _ := http.NewRequest("POST", "/v1/mcp", bytes.NewReader(body))
	req.Header.Set("X-Auth-Token", s.GetToken())
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	var resp MCPResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Error != nil {
		t.Errorf("list resources should succeed, got error: %s", resp.Error.Message)
	}
}

func TestServer_MCP_UnknownMethod(t *testing.T) {
	cfg := config.Config{}
	s := New(cfg)

	mcpReq := MCPRequest{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "nonexistent/method",
	}
	body, _ := json.Marshal(mcpReq)

	req, _ := http.NewRequest("POST", "/v1/mcp", bytes.NewReader(body))
	req.Header.Set("X-Auth-Token", s.GetToken())
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	var resp MCPResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Error == nil {
		t.Error("expected error for unknown method")
	}
	if resp.Error.Code != MCPMethodNotFound {
		t.Errorf("expected method not found error code, got %d", resp.Error.Code)
	}
}

func TestServer_MCP_InvalidJSONRPCVersion(t *testing.T) {
	cfg := config.Config{}
	s := New(cfg)

	mcpReq := MCPRequest{
		JSONRPC: "1.0",
		ID:      6,
		Method:  "ping",
	}
	body, _ := json.Marshal(mcpReq)

	req, _ := http.NewRequest("POST", "/v1/mcp", bytes.NewReader(body))
	req.Header.Set("X-Auth-Token", s.GetToken())
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	var resp MCPResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Error == nil {
		t.Error("expected error for invalid JSON-RPC version")
	}
	if resp.Error.Code != MCPInvalidRequest {
		t.Errorf("expected invalid request error code, got %d", resp.Error.Code)
	}
}

func TestServer_MCP_InvalidJSON(t *testing.T) {
	cfg := config.Config{}
	s := New(cfg)

	req, _ := http.NewRequest("POST", "/v1/mcp", bytes.NewReader([]byte("not json")))
	req.Header.Set("X-Auth-Token", s.GetToken())
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	var resp MCPResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Error == nil {
		t.Error("expected parse error for invalid JSON")
	}
	if resp.Error.Code != MCPParseError {
		t.Errorf("expected parse error code, got %d", resp.Error.Code)
	}
}

func TestServer_MCP_ToolsCall_UnknownTool(t *testing.T) {
	cfg := config.Config{}
	s := New(cfg)

	params, _ := json.Marshal(map[string]interface{}{
		"name":      "unknown_tool",
		"arguments": map[string]string{},
	})

	mcpReq := MCPRequest{
		JSONRPC: "2.0",
		ID:      7,
		Method:  "tools/call",
		Params:  params,
	}
	body, _ := json.Marshal(mcpReq)

	req, _ := http.NewRequest("POST", "/v1/mcp", bytes.NewReader(body))
	req.Header.Set("X-Auth-Token", s.GetToken())
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	var resp MCPResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Error == nil {
		t.Error("expected error for unknown tool")
	}
}

func TestServer_MCP_ToolsCall_InvalidParams(t *testing.T) {
	cfg := config.Config{}
	s := New(cfg)

	mcpReq := MCPRequest{
		JSONRPC: "2.0",
		ID:      8,
		Method:  "tools/call",
		Params:  json.RawMessage("invalid"),
	}
	body, _ := json.Marshal(mcpReq)

	req, _ := http.NewRequest("POST", "/v1/mcp", bytes.NewReader(body))
	req.Header.Set("X-Auth-Token", s.GetToken())
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	var resp MCPResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Error == nil {
		t.Error("expected error for invalid params")
	}
}

func TestServer_MCP_ResourcesRead_UnknownURI(t *testing.T) {
	cfg := config.Config{}
	s := New(cfg)

	params, _ := json.Marshal(map[string]string{"uri": "unknown://something"})
	mcpReq := MCPRequest{
		JSONRPC: "2.0",
		ID:      9,
		Method:  "resources/read",
		Params:  params,
	}
	body, _ := json.Marshal(mcpReq)

	req, _ := http.NewRequest("POST", "/v1/mcp", bytes.NewReader(body))
	req.Header.Set("X-Auth-Token", s.GetToken())
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	var resp MCPResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Error == nil {
		t.Error("expected error for unknown resource URI")
	}
}

func TestGenerateToken(t *testing.T) {
	token1, err := generateToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(token1) != 64 { // 32 bytes = 64 hex chars
		t.Errorf("expected 64 char token, got %d", len(token1))
	}

	// Generate another, should be different
	token2, _ := generateToken()
	if token1 == token2 {
		t.Error("expected unique tokens")
	}
}
