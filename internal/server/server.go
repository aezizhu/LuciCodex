package server

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/aezizhu/LuciCodex/internal/config"
	"github.com/aezizhu/LuciCodex/internal/executor"
	"github.com/aezizhu/LuciCodex/internal/llm"
	"github.com/aezizhu/LuciCodex/internal/llm/prompts"
	"github.com/aezizhu/LuciCodex/internal/openwrt"
	"github.com/aezizhu/LuciCodex/internal/plan"
	"github.com/aezizhu/LuciCodex/internal/policy"
)

// TokenFile is the path where the authentication token is stored
const TokenFile = "/tmp/.lucicodex.token"

// rateLimiter implements a simple token bucket rate limiter
type rateLimiter struct {
	mu       sync.Mutex
	tokens   int
	max      int
	refill   int
	lastTime time.Time
}

func newRateLimiter(max, refillPerSecond int) *rateLimiter {
	return &rateLimiter{
		tokens:   max,
		max:      max,
		refill:   refillPerSecond,
		lastTime: time.Now(),
	}
}

func (rl *rateLimiter) allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(rl.lastTime).Seconds()
	rl.tokens += int(elapsed * float64(rl.refill))
	if rl.tokens > rl.max {
		rl.tokens = rl.max
	}
	rl.lastTime = now

	if rl.tokens > 0 {
		rl.tokens--
		return true
	}
	return false
}

type Server struct {
	cfg     config.Config
	mux     *http.ServeMux
	token   string       // Authentication token
	limiter *rateLimiter // Rate limiter
}

// generateToken creates a cryptographically secure random token
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func New(cfg config.Config) *Server {
	// Generate authentication token
	token, err := generateToken()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to generate auth token: %v\n", err)
		token = "" // Disable auth if token generation fails
	}

	// Write token to file for LuCI to read
	if token != "" {
		if err := os.WriteFile(TokenFile, []byte(token), 0600); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to write token file: %v\n", err)
		}
	}

	s := &Server{
		cfg:     cfg,
		mux:     http.NewServeMux(),
		token:   token,
		limiter: newRateLimiter(30, 2), // 30 requests burst, 2 per second refill
	}

	// Wrap handlers with middleware
	s.mux.HandleFunc("/v1/plan", s.withMiddleware(s.handlePlan))
	s.mux.HandleFunc("/v1/execute", s.withMiddleware(s.handleExecute))
	s.mux.HandleFunc("/v1/summarize", s.withMiddleware(s.handleSummarize))
	s.mux.HandleFunc("/health", s.handleHealth) // Health check doesn't need auth
	return s
}

// withMiddleware wraps a handler with authentication and rate limiting
func (s *Server) withMiddleware(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Rate limiting
		if !s.limiter.allow() {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		// Authentication (if token is configured)
		if s.token != "" {
			authToken := r.Header.Get("X-Auth-Token")
			if authToken == "" {
				// Also check Authorization header for Bearer token
				authHeader := r.Header.Get("Authorization")
				if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
					authToken = authHeader[7:]
				}
			}

			// Use constant-time comparison to prevent timing attacks
			if subtle.ConstantTimeCompare([]byte(authToken), []byte(s.token)) != 1 {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}

		handler(w, r)
	}
}

// GetToken returns the server's authentication token
func (s *Server) GetToken() string {
	return s.token
}

func (s *Server) Start(port int) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	fmt.Printf("LuciCodex Daemon listening on %s\n", addr)
	if s.token != "" {
		fmt.Printf("Auth token written to %s\n", TokenFile)
	} else {
		fmt.Println("Warning: Running without authentication")
	}
	// Configure HTTP server with timeouts to prevent resource exhaustion
	srv := &http.Server{
		Addr:         addr,
		Handler:      s.mux,
		ReadTimeout:  10 * time.Second,  // Time to read request headers + body
		WriteTimeout: 60 * time.Second,  // Time to write response (LLM calls can be slow)
		IdleTimeout:  120 * time.Second, // Keep-alive timeout
	}
	return srv.ListenAndServe()
}

type PlanRequest struct {
	Prompt   string            `json:"prompt"`
	Provider string            `json:"provider"`
	Model    string            `json:"model"`
	Config   map[string]string `json:"config"` // API keys override
}

type ExecuteRequest struct {
	Prompt   string                `json:"prompt"`
	Provider string                `json:"provider"`
	Model    string                `json:"model"`
	Config   map[string]string     `json:"config"`
	DryRun   bool                  `json:"dry_run"`
	Timeout  int                   `json:"timeout"`
	Commands []plan.PlannedCommand `json:"commands"` // Optional: Direct execution
}

type SummarizeRequest struct {
	Prompt   string               `json:"prompt"`
	Context  string               `json:"context"`
	Provider string               `json:"provider"`
	Model    string               `json:"model"`
	Config   map[string]string    `json:"config"`
	Commands []llm.SummaryCommand `json:"commands"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (s *Server) handlePlan(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Received /v1/plan request")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req PlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Prompt == "" {
		http.Error(w, "Prompt is required", http.StatusBadRequest)
		return
	}

	// Debug: Log received config keys (mask actual values for security)
	fmt.Printf("Plan request - Provider: %s, Model: %s\n", req.Provider, req.Model)
	if req.Config != nil {
		for k, v := range req.Config {
			if v != "" {
				fmt.Printf("  Config[%s]: (set, %d chars)\n", k, len(v))
			} else {
				fmt.Printf("  Config[%s]: (empty)\n", k)
			}
		}
	} else {
		fmt.Println("  Config: nil")
	}

	// Merge config
	cfg := s.cfg
	if req.Provider != "" {
		cfg.Provider = req.Provider
	}
	if req.Model != "" {
		cfg.Model = req.Model
	}
	if val, ok := req.Config["openai_key"]; ok && val != "" {
		cfg.OpenAIAPIKey = val
	}
	if val, ok := req.Config["gemini_key"]; ok && val != "" {
		cfg.APIKey = val
	}
	if val, ok := req.Config["anthropic_key"]; ok && val != "" {
		cfg.AnthropicAPIKey = val
	}
	cfg.ApplyProviderSettings()

	// Debug: Log final config state (mask actual values)
	fmt.Printf("Final config - Provider: %s, Model: %s\n", cfg.Provider, cfg.Model)
	fmt.Printf("  GeminiKey: %v, OpenAIKey: %v, AnthropicKey: %v\n",
		cfg.APIKey != "", cfg.OpenAIAPIKey != "", cfg.AnthropicAPIKey != "")

	ctx := r.Context()
	llmProvider := llm.NewProvider(cfg)

	// Collect facts
	factsCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	envFacts := openwrt.CollectFacts(factsCtx)

	instruction := prompts.GenerateSurvivalPrompt(cfg.MaxCommands)
	if envFacts != "" {
		instruction += "\n\nEnvironment facts (read-only):\n" + envFacts
	}
	fullPrompt := instruction + "\n\nUser request: " + req.Prompt

	// Generate plan
	planCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.TimeoutSeconds)*time.Second)
	defer cancel()

	p, err := llmProvider.GeneratePlan(planCtx, fullPrompt)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("LLM error: %v", err)})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":   true,
		"plan": p,
	})
}

func (s *Server) handleExecute(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Received /v1/execute request")
	if r.Method != http.MethodPost {
		fmt.Println("Error: Method not allowed")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Merge config
	cfg := s.cfg
	if req.Provider != "" {
		cfg.Provider = req.Provider
	}
	if req.Model != "" {
		cfg.Model = req.Model
	}
	if req.Timeout > 0 {
		cfg.TimeoutSeconds = req.Timeout
	}
	cfg.DryRun = req.DryRun

	if val, ok := req.Config["openai_key"]; ok && val != "" {
		cfg.OpenAIAPIKey = val
	}
	if val, ok := req.Config["gemini_key"]; ok && val != "" {
		cfg.APIKey = val
	}
	if val, ok := req.Config["anthropic_key"]; ok && val != "" {
		cfg.AnthropicAPIKey = val
	}
	cfg.ApplyProviderSettings()

	ctx := r.Context()
	llmProvider := llm.NewProvider(cfg)
	policyEngine := policy.New(cfg)
	execEngine := executor.New(cfg)

	var p plan.Plan
	var err error

	// Check if commands are provided directly (Stateless Execution)
	if len(req.Commands) > 0 {
		fmt.Println("Executing provided plan directly (skipping LLM)...")
		p = plan.Plan{
			Summary:  "Direct execution",
			Commands: req.Commands,
		}
	} else {
		// Legacy: Re-generate plan
		// Collect facts
		factsCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		envFacts := openwrt.CollectFacts(factsCtx)

		instruction := prompts.GenerateSurvivalPrompt(cfg.MaxCommands)
		if envFacts != "" {
			instruction += "\n\nEnvironment facts (read-only):\n" + envFacts
		}
		fullPrompt := instruction + "\n\nUser request: " + req.Prompt

		// Generate plan
		planCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.TimeoutSeconds)*time.Second)
		defer cancel()

		fmt.Println("Generating plan for execution...")
		start := time.Now()
		p, err = llmProvider.GeneratePlan(planCtx, fullPrompt)
		if err != nil {
			fmt.Printf("Plan generation failed: %v\n", err)
			http.Error(w, fmt.Sprintf("Failed to generate plan: %v", err), http.StatusInternalServerError)
			return
		}
		fmt.Printf("Plan generated in %v\n", time.Since(start))
	}

	if len(p.Commands) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":      true,
			"plan":    p, // Include the summary for conversational responses
			"result":  executor.Results{},
			"message": "No commands to execute",
		})
		return
	}

	// Validate
	if err := policyEngine.ValidatePlan(p); err != nil {
		fmt.Printf("Policy validation failed: %v\n", err)
		http.Error(w, fmt.Sprintf("Policy error: %v", err), http.StatusForbidden)
		return
	}

	if cfg.DryRun {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":      true,
			"plan":    p,
			"dry_run": true,
		})
		return
	}

	// Execute
	results := execEngine.RunPlan(ctx, p)

	results = execEngine.AutoRetry(ctx, llmProvider, policyEngine, results, nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":     true,
		"result": results,
	})
}

func (s *Server) handleSummarize(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Received /v1/summarize request")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SummarizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if len(req.Commands) == 0 {
		http.Error(w, "Commands are required for summarization", http.StatusBadRequest)
		return
	}

	cfg := s.cfg
	if req.Provider != "" {
		cfg.Provider = req.Provider
	}
	if req.Model != "" {
		cfg.Model = req.Model
	}
	if val, ok := req.Config["openai_key"]; ok && val != "" {
		cfg.OpenAIAPIKey = val
	}
	if val, ok := req.Config["gemini_key"]; ok && val != "" {
		cfg.APIKey = val
	}
	if val, ok := req.Config["anthropic_key"]; ok && val != "" {
		cfg.AnthropicAPIKey = val
	}
	cfg.ApplyProviderSettings()

	ctx := r.Context()

	// Ensure selected provider has a key; fail fast with a clear message.
	switch cfg.Provider {
	case "openai":
		if cfg.OpenAIAPIKey == "" {
			http.Error(w, "Summarize: missing OpenAI API key", http.StatusBadRequest)
			return
		}
	case "gemini":
		if cfg.APIKey == "" {
			http.Error(w, "Summarize: missing Gemini API key", http.StatusBadRequest)
			return
		}
	case "anthropic":
		if cfg.AnthropicAPIKey == "" {
			http.Error(w, "Summarize: missing Anthropic API key", http.StatusBadRequest)
			return
		}
	default:
		http.Error(w, fmt.Sprintf("Summarize: unsupported provider %s", cfg.Provider), http.StatusBadRequest)
		return
	}

	summary, details, err := llm.Summarize(ctx, cfg, llm.SummaryInput{
		Commands: req.Commands,
		Context:  req.Context,
		Prompt:   req.Prompt,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to summarize: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"summary": summary,
		"details": details,
	})
}
