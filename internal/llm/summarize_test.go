package llm

import (
	"strings"
	"testing"
)

func TestParseSummary_ValidJSON(t *testing.T) {
	input := `{"summary": "Your IP is 192.168.1.1", "details": ["Connected via eth0", "DHCP enabled"]}`
	summary, details := parseSummary(input)

	if summary != "Your IP is 192.168.1.1" {
		t.Errorf("expected specific summary, got %q", summary)
	}
	if len(details) != 2 {
		t.Fatalf("expected 2 details, got %d", len(details))
	}
	if details[0] != "Connected via eth0" {
		t.Errorf("expected first detail about eth0, got %q", details[0])
	}
}

func TestParseSummary_SummaryOnly(t *testing.T) {
	input := `{"summary": "Everything is working"}`
	summary, details := parseSummary(input)

	if summary != "Everything is working" {
		t.Errorf("expected summary, got %q", summary)
	}
	if details != nil {
		t.Errorf("expected nil details, got %v", details)
	}
}

func TestParseSummary_EmptySummary(t *testing.T) {
	input := `{"summary": "", "details": ["some detail"]}`
	summary, details := parseSummary(input)

	// Empty summary should fallback to raw text
	if summary != input {
		t.Errorf("expected raw text fallback, got %q", summary)
	}
	if details != nil {
		t.Errorf("expected nil details on fallback")
	}
}

func TestParseSummary_InvalidJSON(t *testing.T) {
	input := "This is just plain text, not JSON"
	summary, details := parseSummary(input)

	if summary != input {
		t.Errorf("expected raw text, got %q", summary)
	}
	if details != nil {
		t.Errorf("expected nil details, got %v", details)
	}
}

func TestParseSummary_EmptyInput(t *testing.T) {
	summary, details := parseSummary("")
	if summary != "" {
		t.Errorf("expected empty summary, got %q", summary)
	}
	if details != nil {
		t.Errorf("expected nil details")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		max      int
		expected string
	}{
		{"within limit", "hello", 10, "hello"},
		{"at limit", "hello", 5, "hello"},
		{"over limit", "hello world!", 8, "hello..."},
		{"zero max", "hello", 0, "hello"},
		{"negative max", "hello", -1, "hello"},
		{"very small max", "hello", 3, "hel"},
		{"max of 4", "hello world", 4, "h..."},
		{"empty string", "", 10, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncate(tt.input, tt.max)
			if result != tt.expected {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, result, tt.expected)
			}
		})
	}
}

func TestBuildSummaryPrompt(t *testing.T) {
	input := SummaryInput{
		Commands: []SummaryCommand{
			{
				Command: []string{"ip", "addr"},
				Output:  "192.168.1.1",
				Error:   "",
			},
			{
				Command: []string{"uci", "show"},
				Output:  "",
				Error:   "command not found",
			},
		},
		Context: "Router configuration check",
		Prompt:  "What is my IP?",
	}

	result := buildSummaryPrompt(input)

	// Check that prompt contains key elements
	if !strings.Contains(result, "What is my IP?") {
		t.Error("expected user prompt in output")
	}
	if !strings.Contains(result, "Router configuration check") {
		t.Error("expected context in output")
	}
	if !strings.Contains(result, "ip addr") {
		t.Error("expected command in output")
	}
	if !strings.Contains(result, "192.168.1.1") {
		t.Error("expected command output in prompt")
	}
	if !strings.Contains(result, "command not found") {
		t.Error("expected error output in prompt")
	}
	if !strings.Contains(result, "USER'S ORIGINAL QUESTION") {
		t.Error("expected question section header")
	}
	if !strings.Contains(result, "COMMAND EXECUTION RESULTS") {
		t.Error("expected results section header")
	}
}

func TestBuildSummaryPrompt_NoContext(t *testing.T) {
	input := SummaryInput{
		Commands: []SummaryCommand{
			{Command: []string{"echo", "test"}, Output: "test"},
		},
		Prompt: "Run a test",
	}

	result := buildSummaryPrompt(input)

	if strings.Contains(result, "Additional context") {
		t.Error("should not include context section when empty")
	}
}

func TestBuildSummaryPrompt_NoPrompt(t *testing.T) {
	input := SummaryInput{
		Commands: []SummaryCommand{
			{Command: []string{"echo", "test"}, Output: "test"},
		},
	}

	result := buildSummaryPrompt(input)

	if strings.Contains(result, "USER'S ORIGINAL QUESTION") {
		t.Error("should not include prompt section when empty")
	}
}

func TestBuildSummaryPrompt_TruncatesLongOutput(t *testing.T) {
	longOutput := strings.Repeat("x", 2000)
	input := SummaryInput{
		Commands: []SummaryCommand{
			{Command: []string{"cat", "bigfile"}, Output: longOutput},
		},
	}

	result := buildSummaryPrompt(input)

	// Output should be truncated to 1500 chars (with "..." suffix)
	if strings.Contains(result, longOutput) {
		t.Error("expected long output to be truncated")
	}
	if !strings.Contains(result, "...") {
		t.Error("expected truncation indicator")
	}
}
