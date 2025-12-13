package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aezizhu/LuciCodex/internal/config"
)

// SummaryCommand represents a single executed command with its output and error.
type SummaryCommand struct {
	Command []string `json:"command"`
	Output  string   `json:"output"`
	Error   string   `json:"error"`
}

// SummaryInput contains execution outputs plus optional user context.
type SummaryInput struct {
	Commands []SummaryCommand
	Context  string
	Prompt   string
}

// Summarize generates a concise summary of execution outputs using the selected provider.
func Summarize(ctx context.Context, cfg config.Config, input SummaryInput) (string, []string, error) {
	switch cfg.Provider {
	case "openai":
		client := NewOpenAIClient(cfg)
		prompt := buildSummaryPrompt(input)
		return client.Summarize(ctx, prompt)
	case "gemini":
		client := NewGeminiClient(cfg)
		prompt := buildSummaryPrompt(input)
		return client.Summarize(ctx, prompt)
	case "anthropic":
		client := NewAnthropicClient(cfg)
		prompt := buildSummaryPrompt(input)
		return client.Summarize(ctx, prompt)
	default:
		return "", nil, fmt.Errorf("unsupported provider for summarization: %s", cfg.Provider)
	}
}

func buildSummaryPrompt(input SummaryInput) string {
	var b strings.Builder
	b.WriteString("You are an assistant helping an OpenWrt router user. Analyze the command outputs below and DIRECTLY ANSWER the user's original question.\n\n")
	b.WriteString("Return strict JSON with this shape:\n")
	b.WriteString("{\"summary\": string, \"details\": [string]}\n\n")
	b.WriteString("Guidelines:\n")
	b.WriteString("- summary: DIRECTLY ANSWER the user's question in 1-2 sentences. Extract specific values (IP addresses, status, names, etc.) from the output.\n")
	b.WriteString("- details: Optional array of additional relevant information from the output.\n")
	b.WriteString("- Be helpful and concise. Focus on what the user asked, not on describing commands.\n")
	b.WriteString("- If the user asked 'what is my IP?', respond with 'Your IP address is X.X.X.X' - not 'The command ran successfully'.\n")
	b.WriteString("- If something failed, explain what went wrong and suggest a fix.\n\n")

	if input.Prompt != "" {
		b.WriteString("USER'S ORIGINAL QUESTION:\n")
		b.WriteString(truncate(input.Prompt, 800))
		b.WriteString("\n\n")
	}
	if input.Context != "" {
		b.WriteString("Additional context:\n")
		b.WriteString(truncate(input.Context, 800))
		b.WriteString("\n\n")
	}

	b.WriteString("COMMAND EXECUTION RESULTS:\n")
	for i, cmd := range input.Commands {
		cmdLine := strings.Join(cmd.Command, " ")
		b.WriteString(fmt.Sprintf("%d) Command: %s\n", i+1, cmdLine))
		if cmd.Output != "" {
			b.WriteString("Output:\n")
			b.WriteString(truncate(cmd.Output, 1500))
			b.WriteString("\n")
		}
		if cmd.Error != "" {
			b.WriteString("Error: ")
			b.WriteString(truncate(cmd.Error, 600))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("\nNow answer the user's question based on the command output above.")
	return b.String()
}

// parseSummary attempts to parse JSON {"summary": "...", "details": [...]} and falls back to text.
func parseSummary(text string) (string, []string) {
	var payload struct {
		Summary string   `json:"summary"`
		Details []string `json:"details"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err == nil && payload.Summary != "" {
		return payload.Summary, payload.Details
	}
	return text, nil
}

func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
