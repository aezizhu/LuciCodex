// Package config provides configuration loading and validation for LuciCodex.
//
// Configuration is loaded with the following precedence (highest to lowest):
//   1. Environment variables (GEMINI_API_KEY, OPENAI_API_KEY, etc.)
//   2. UCI configuration (/etc/config/lucicodex on OpenWrt)
//   3. JSON config file (/etc/lucicodex/config.json or ~/.config/lucicodex/config.json)
//   4. Built-in defaults
//
// Supported providers:
//   - gemini    - Google Gemini (default, free tier available)
//   - openai    - OpenAI GPT models
//   - anthropic - Anthropic Claude models
//
// Key configuration fields:
//   - Provider       - Active LLM provider (gemini/openai/anthropic)
//   - APIKey         - Gemini API key
//   - OpenAIAPIKey   - OpenAI API key
//   - AnthropicAPIKey - Anthropic API key
//   - DryRun         - Preview commands without execution
//   - TimeoutSeconds - Per-command timeout
//   - MaxCommands    - Maximum commands per plan
//
// Example usage:
//
//	cfg, err := config.Load("")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	if err := cfg.Validate(); err != nil {
//	    log.Fatal(err)
//	}
package config
