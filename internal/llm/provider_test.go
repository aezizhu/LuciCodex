package llm

import (
	"testing"

	"github.com/aezizhu/LuciCodex/internal/config"
)

func TestNewProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		wantType string
	}{
		{"gemini", "gemini", "*llm.GeminiClient"},
		{"openai", "openai", "*llm.OpenAIClient"},
		{"anthropic", "anthropic", "*llm.AnthropicClient"},
		{"default", "", "*llm.GeminiClient"},
		{"unknown", "unknown", "*llm.GeminiClient"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Config{
				Provider: tt.provider,
				APIKey:   "test",
			}
			p := NewProvider(cfg)

			// We can't easily check the type string without reflection or type assertion,
			// but we can check if it's not nil
			if p == nil {
				t.Errorf("NewProvider() returned nil")
			}

			// Basic type assertion check
			switch tt.wantType {
			case "*llm.GeminiClient":
				if _, ok := p.(*GeminiClient); !ok {
					t.Errorf("expected GeminiClient")
				}
			case "*llm.OpenAIClient":
				if _, ok := p.(*OpenAIClient); !ok {
					t.Errorf("expected OpenAIClient")
				}
			case "*llm.AnthropicClient":
				if _, ok := p.(*AnthropicClient); !ok {
					t.Errorf("expected AnthropicClient")
				}
			}
		})
	}
}
