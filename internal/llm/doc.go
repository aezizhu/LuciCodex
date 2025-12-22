// Package llm provides LLM client implementations for LuciCodex.
//
// This package implements the Provider interface for multiple LLM services,
// enabling LuciCodex to generate execution plans and summarize command outputs.
//
// Supported providers:
//   - GeminiClient    - Google Gemini API (gemini-3-flash default)
//   - OpenAIClient    - OpenAI API (gpt-5-mini default)
//   - AnthropicClient - Anthropic API (claude-haiku-4-5-20251001 default)
//
// Error handling:
//   - APIError    - Wraps HTTP errors from LLM APIs with status codes
//   - ParseError  - Wraps JSON parsing failures with context
//
// The package includes helper functions for:
//   - Automatic provider selection based on configuration
//   - HTTP client configuration with proxy support
//   - Response parsing and plan extraction
//   - Command output summarization
//
// Example usage:
//
//	cfg := config.Load("")
//	provider := llm.NewProvider(cfg)
//	plan, err := provider.GeneratePlan(ctx, "show network interfaces")
//	if err != nil {
//	    var apiErr *llm.APIError
//	    if errors.As(err, &apiErr) && apiErr.IsRateLimited() {
//	        // Handle rate limiting
//	    }
//	}
package llm
