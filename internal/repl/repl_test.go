package repl

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aezizhu/LuciCodex/internal/config"
	"github.com/aezizhu/LuciCodex/internal/plan"
	"github.com/aezizhu/LuciCodex/internal/testutil"
)

// MockProvider implements llm.Provider for testing
type MockProvider struct {
	Plan plan.Plan
	Err  error
}

func (m *MockProvider) GeneratePlan(ctx context.Context, prompt string) (plan.Plan, error) {
	return m.Plan, m.Err
}

func (m *MockProvider) GenerateErrorFix(ctx context.Context, cmd, output string, attempt int) (plan.Plan, error) {
	return m.Plan, m.Err
}

func TestREPL_Commands(t *testing.T) {
	input := `help
status
set dry-run=false
status
history
clear
exit
`
	var output bytes.Buffer
	cfg := config.Config{
		Provider: "test",
		Model:    "test-model",
		DryRun:   true,
	}

	r := New(cfg, strings.NewReader(input), &output)

	// Run REPL
	err := r.Run(context.Background())
	testutil.AssertNoError(t, err)

	outStr := testutil.StripAnsi(output.String())

	// Check help output
	testutil.AssertContains(t, outStr, "Available commands:")

	// Check initial status
	testutil.AssertContains(t, outStr, "Dry run: true")

	// Check set command
	testutil.AssertContains(t, outStr, "Set dry-run to false")

	// Check updated status
	testutil.AssertContains(t, outStr, "Dry run: false")

	// Check history
	testutil.AssertContains(t, outStr, "history")

	// Check clear
	testutil.AssertContains(t, outStr, "History cleared")
}

func TestREPL_LLMInteraction(t *testing.T) {
	// Mock LLM response
	mockPlan := plan.Plan{
		Summary: "Test Plan",
		Commands: []plan.PlannedCommand{
			{Command: []string{"echo", "test"}},
		},
	}

	input := "do something\nexit\n"
	var output bytes.Buffer
	cfg := config.Config{
		Provider:    "test",
		DryRun:      true, // Keep dry run to avoid execution
		MaxCommands: 10,
		Allowlist:   []string{"^echo"},
	}

	r := New(cfg, strings.NewReader(input), &output)

	// Inject mock provider
	r.provider = &MockProvider{Plan: mockPlan}

	err := r.Run(context.Background())
	testutil.AssertNoError(t, err)

	outStr := testutil.StripAnsi(output.String())

	// Check plan output
	testutil.AssertContains(t, outStr, "Summary: Test Plan")
	testutil.AssertContains(t, outStr, "echo test")
	testutil.AssertContains(t, outStr, "Dry run mode - no execution")
}

func TestREPL_HistoryCommand(t *testing.T) {
	// Test !1 command
	input := "echo test\n!1\nexit\n"
	var output bytes.Buffer
	cfg := config.Config{
		DryRun:    true,
		Allowlist: []string{"^echo"},
	}

	r := New(cfg, strings.NewReader(input), &output)

	// Inject mock provider
	mockPlan := plan.Plan{
		Summary: "Echo Test",
		Commands: []plan.PlannedCommand{
			{Command: []string{"echo", "test"}},
		},
	}
	r.provider = &MockProvider{Plan: mockPlan}

	err := r.Run(context.Background())
	testutil.AssertNoError(t, err)

	outStr := testutil.StripAnsi(output.String())
	testutil.AssertContains(t, outStr, "Re-running: echo test")
}

func TestREPL_SetCommands(t *testing.T) {
	input := `set provider=anthropic
set model=claude-3-opus
set auto-approve=true
set unknown=value
set invalid
exit
`
	var output bytes.Buffer
	cfg := config.Config{
		Provider: "gemini",
	}

	r := New(cfg, strings.NewReader(input), &output)

	err := r.Run(context.Background())
	testutil.AssertNoError(t, err)

	outStr := testutil.StripAnsi(output.String())

	testutil.AssertContains(t, outStr, "Set provider to anthropic")
	testutil.AssertContains(t, outStr, "Set model to claude-3-opus")
	testutil.AssertContains(t, outStr, "Set auto-approve to true")
	testutil.AssertContains(t, outStr, "unknown setting: unknown")
	testutil.AssertContains(t, outStr, "usage: set key=value")
}

func TestREPL_LLMError(t *testing.T) {
	input := "do something\nexit\n"
	var output bytes.Buffer
	cfg := config.Config{Provider: "test"}
	r := New(cfg, strings.NewReader(input), &output)

	// Inject mock provider with error
	r.provider = &MockProvider{Err: context.DeadlineExceeded}

	err := r.Run(context.Background())
	testutil.AssertNoError(t, err)

	outStr := testutil.StripAnsi(output.String())
	testutil.AssertContains(t, outStr, "Error: LLM error")
}

func TestREPL_HistoryErrors(t *testing.T) {
	// Add a command first so history is not empty
	input := "echo test\n!abc\n!999\nexit\n"
	var output bytes.Buffer
	cfg := config.Config{
		Provider: "test",
		DryRun:   true,
	}
	r := New(cfg, strings.NewReader(input), &output)

	// Mock provider for the "echo test" command
	r.provider = &MockProvider{Plan: plan.Plan{
		Summary:  "Echo",
		Commands: []plan.PlannedCommand{{Command: []string{"echo", "test"}}},
	}}

	err := r.Run(context.Background())
	testutil.AssertNoError(t, err)

	outStr := testutil.StripAnsi(output.String())
	testutil.AssertContains(t, outStr, "Error: invalid history index")      // !abc
	testutil.AssertContains(t, outStr, "Error: history index out of range") // !999
}

func TestREPL_EmptyPlan(t *testing.T) {
	input := "do nothing\nexit\n"
	var output bytes.Buffer
	cfg := config.Config{Provider: "test"}
	r := New(cfg, strings.NewReader(input), &output)

	// Inject mock provider with empty plan
	mockPlan := plan.Plan{
		Summary:  "Empty Plan",
		Commands: []plan.PlannedCommand{},
	}
	r.provider = &MockProvider{Plan: mockPlan}

	err := r.Run(context.Background())
	testutil.AssertNoError(t, err)

	outStr := testutil.StripAnsi(output.String())
	// With the new UI, empty plans show the summary from the LLM response
	testutil.AssertContains(t, outStr, "Empty Plan")
}

func TestREPL_MaxCommands(t *testing.T) {
	input := "do too much\nexit\n"
	var output bytes.Buffer
	cfg := config.Config{
		Provider:    "test",
		MaxCommands: 1,
		DryRun:      true,
	}
	r := New(cfg, strings.NewReader(input), &output)

	// Inject mock provider with 2 commands
	mockPlan := plan.Plan{
		Summary: "Too Many Commands",
		Commands: []plan.PlannedCommand{
			{Command: []string{"echo", "1"}},
			{Command: []string{"echo", "2"}},
		},
	}
	r.provider = &MockProvider{Plan: mockPlan}

	err := r.Run(context.Background())
	testutil.AssertNoError(t, err)

	// Should only show first command
	outStr := testutil.StripAnsi(output.String())
	testutil.AssertContains(t, outStr, "echo 1")
	testutil.AssertNotContains(t, outStr, "echo 2")
}

func TestREPL_ConfirmationCancellation(t *testing.T) {
	input := "do something\nn\nexit\n" // Command, then 'n' to cancel
	var output bytes.Buffer
	cfg := config.Config{
		Provider:    "test",
		DryRun:      false, // Need dry run false to trigger confirmation
		AutoApprove: false,
		Allowlist:   []string{"^echo"},
	}
	r := New(cfg, strings.NewReader(input), &output)

	r.provider = &MockProvider{Plan: plan.Plan{
		Summary:  "Dangerous",
		Commands: []plan.PlannedCommand{{Command: []string{"echo", "dangerous"}}},
	}}

	err := r.Run(context.Background())
	testutil.AssertNoError(t, err)

	outStr := testutil.StripAnsi(output.String())
	testutil.AssertContains(t, outStr, "Execute these commands?")
	testutil.AssertContains(t, outStr, "Cancelled")
	testutil.AssertNotContains(t, outStr, "Executing:")
}

func TestREPL_HistoryLimit(t *testing.T) {
	// Add 3 commands with limit 2
	input := "cmd1\ncmd2\ncmd3\nhistory\nexit\n"
	var output bytes.Buffer
	cfg := config.Config{
		Provider: "test",
		DryRun:   true,
	}
	r := New(cfg, strings.NewReader(input), &output)
	r.maxHistory = 2 // Override for test

	// Mock provider to just return success
	r.provider = &MockProvider{Plan: plan.Plan{
		Summary:  "Echo",
		Commands: []plan.PlannedCommand{{Command: []string{"echo", "ok"}}},
	}}

	err := r.Run(context.Background())
	testutil.AssertNoError(t, err)

	outStr := testutil.StripAnsi(output.String())
	// Should contain cmd2 and cmd3, but not cmd1
	testutil.AssertContains(t, outStr, "cmd2")
	testutil.AssertContains(t, outStr, "cmd3")
	testutil.AssertNotContains(t, outStr, "cmd1")
}

func TestREPL_ShowHistory(t *testing.T) {
	input := "cmd1\ncmd2\nhistory\nexit\n"
	var output bytes.Buffer
	cfg := config.Config{
		Provider: "test",
		DryRun:   true,
	}
	r := New(cfg, strings.NewReader(input), &output)

	r.provider = &MockProvider{Plan: plan.Plan{
		Summary:  "Echo",
		Commands: []plan.PlannedCommand{{Command: []string{"echo", "ok"}}},
	}}

	err := r.Run(context.Background())
	testutil.AssertNoError(t, err)

	outStr := testutil.StripAnsi(output.String())
	testutil.AssertContains(t, outStr, "1  cmd1")
	testutil.AssertContains(t, outStr, "2  cmd2")
}

type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("read error")
}

func TestREPL_ReadError(t *testing.T) {
	var output bytes.Buffer
	cfg := config.Config{Provider: "test"}
	// bufio.NewReader wraps our errorReader. ReadString calls Read.
	r := New(cfg, &errorReader{}, &output)

	err := r.Run(context.Background())
	if err == nil || err.Error() != "read error" {
		t.Errorf("expected read error, got %v", err)
	}
}

func TestREPL_ExecutionResults(t *testing.T) {
	input := "echo test\nexit\n"
	var output bytes.Buffer
	cfg := config.Config{
		Provider:    "test",
		DryRun:      false,
		AutoApprove: true,
		Allowlist:   []string{"^echo"},
	}
	r := New(cfg, strings.NewReader(input), &output)

	// Mock provider
	r.provider = &MockProvider{Plan: plan.Plan{
		Summary:  "Echo",
		Commands: []plan.PlannedCommand{{Command: []string{"echo", "test"}}},
	}}

	err := r.Run(context.Background())
	testutil.AssertNoError(t, err)

	outStr := testutil.StripAnsi(output.String())
	testutil.AssertContains(t, outStr, "echo test")
}
