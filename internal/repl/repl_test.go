package repl

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aezizhu/LuciCodex/internal/config"
	"github.com/aezizhu/LuciCodex/internal/executor"
	"github.com/aezizhu/LuciCodex/internal/openwrt"
	"github.com/aezizhu/LuciCodex/internal/plan"
)

// MockLLMProvider is a mock implementation of the llm.Provider interface.
type MockLLMProvider struct {
	GeneratePlanFunc     func(ctx context.Context, prompt string) (plan.Plan, error)
	GenerateErrorFixFunc func(ctx context.Context, originalCommand string, errorOutput string, attempt int) (plan.Plan, error)
}

func (m *MockLLMProvider) GeneratePlan(ctx context.Context, prompt string) (plan.Plan, error) {
	if m.GeneratePlanFunc != nil {
		return m.GeneratePlanFunc(ctx, prompt)
	}
	return plan.Plan{}, errors.New("GeneratePlan not implemented")
}

func (m *MockLLMProvider) GenerateErrorFix(ctx context.Context, originalCommand string, errorOutput string, attempt int) (plan.Plan, error) {
	if m.GenerateErrorFixFunc != nil {
		return m.GenerateErrorFixFunc(ctx, originalCommand, errorOutput, attempt)
	}
	return plan.Plan{}, errors.New("GenerateErrorFix not implemented")
}

func TestREPL_SimpleCommands(t *testing.T) {
	input := "status\nset dry-run=false\nhelp\nhistory\nexit\n"
	var output bytes.Buffer
	cfg := config.Config{DryRun: true}

	r := New(cfg, strings.NewReader(input), &output)
	r.addToHistory("a previous command")
	r.Run(context.Background())

	outStr := output.String()
	if !strings.Contains(outStr, "Dry run: true") {
		t.Error("expected to see initial status of dry-run")
	}
	if !strings.Contains(outStr, "Set dry-run to false") {
		t.Error("expected to see confirmation of setting dry-run")
	}
	if !strings.Contains(outStr, "Available commands:") {
		t.Error("expected to see help output")
	}
	if !strings.Contains(outStr, "1  a previous command") {
		t.Error("expected to see history output")
	}
}

func TestREPL_ExecutePrompt(t *testing.T) {
	// --- Setup Mocks ---
	// Mock for LLM provider
	mockProvider := &MockLLMProvider{
		GeneratePlanFunc: func(ctx context.Context, prompt string) (plan.Plan, error) {
			return plan.Plan{
				Summary: "Mock plan for allowed command",
				Commands: []plan.PlannedCommand{
					{Command: []string{"ip", "addr"}},
				},
			}, nil
		},
	}

	// Mock for executor engine
	originalExecCommand := executor.GetRunCommand()
	defer executor.SetRunCommand(originalExecCommand)
	executor.SetRunCommand(func(ctx context.Context, argv []string) (string, error) {
		if len(argv) > 0 && argv[0] == "ip" {
			return "1: lo: <LOOPBACK,UP,LOWER_UP>...", nil
		}
		return "", errors.New("unexpected command")
	})

	// Mock for openwrt facts
	originalFactsCommand := openwrt.GetRunCommand()
	defer openwrt.SetRunCommand(originalFactsCommand)
	openwrt.SetRunCommand(func(ctx context.Context, name string, args ...string) string {
		return "mock facts"
	})

	// --- Setup REPL ---
	input := "show ip address\ny\nexit\n" // prompt, confirmation, exit
	var output bytes.Buffer
	// Use a config that would allow the 'ip' command
	cfg := config.Config{
		DryRun:      false,
		AutoApprove: false,
		Allowlist:   []string{
			"^ip(\\s|$)",
		},
	}

	r := New(cfg, strings.NewReader(input), &output)
	// Replace provider with mock
	r.provider = mockProvider

	// --- Act ---
	r.Run(context.Background())

	// --- Assert ---
	outStr := output.String()
	if !strings.Contains(outStr, "Mock plan for allowed command") {
		t.Error("expected to see the plan summary from the mock provider")
	}
	if !strings.Contains(outStr, "Execute these commands? [y/N]:") {
		t.Error("expected to see the confirmation prompt")
	}
	if !strings.Contains(outStr, "1: lo: <LOOPBACK,UP,LOWER_UP>...") {
		t.Errorf("expected to see the indented execution result. Full output:\n%s", outStr)
	}
	if !strings.Contains(outStr, "All commands executed successfully") {
		t.Error("expected to see the final success message")
	}
}
