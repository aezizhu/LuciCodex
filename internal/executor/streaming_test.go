package executor

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aezizhu/LuciCodex/internal/plan"
	"github.com/aezizhu/LuciCodex/internal/testutil"
)

func TestRunPlanStreaming_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real execution in short mode")
	}

	cfg := testutil.DefaultTestConfig()
	engine := New(cfg)

	p := plan.Plan{
		Summary: "streaming test",
		Commands: []plan.PlannedCommand{
			{Command: []string{"echo", "hello"}, Description: "say hello"},
		},
	}

	var buf bytes.Buffer
	results := engine.RunPlanStreaming(context.Background(), p, &buf)

	testutil.AssertEqual(t, len(results.Items), 1)
	testutil.AssertEqual(t, results.Failed, 0)

	// Check that streaming output contains command info
	output := buf.String()
	if !strings.Contains(output, "Executing") {
		t.Error("expected 'Executing' in streaming output")
	}
	if !strings.Contains(output, "Done") {
		t.Error("expected 'Done' in streaming output")
	}
}

func TestRunPlanStreaming_WithFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real execution in short mode")
	}

	cfg := testutil.DefaultTestConfig()
	engine := New(cfg)

	p := plan.Plan{
		Commands: []plan.PlannedCommand{
			{Command: []string{"false"}, Description: "failing command"},
		},
	}

	var buf bytes.Buffer
	results := engine.RunPlanStreaming(context.Background(), p, &buf)

	testutil.AssertEqual(t, results.Failed, 1)
	testutil.AssertError(t, results.Items[0].Err)

	output := buf.String()
	if !strings.Contains(output, "Failed") {
		t.Error("expected 'Failed' in streaming output")
	}
}

func TestRunPlanStreaming_EmptyCommand(t *testing.T) {
	cfg := testutil.DefaultTestConfig()
	engine := New(cfg)

	p := plan.Plan{
		Commands: []plan.PlannedCommand{
			{Command: []string{}, Description: "empty"},
		},
	}

	var buf bytes.Buffer
	results := engine.RunPlanStreaming(context.Background(), p, &buf)

	testutil.AssertEqual(t, results.Failed, 1)
	testutil.AssertError(t, results.Items[0].Err)
	testutil.AssertContains(t, results.Items[0].Err.Error(), "empty command")
}

func TestRunPlanStreaming_MultipleCommands(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real execution in short mode")
	}

	cfg := testutil.DefaultTestConfig()
	engine := New(cfg)

	p := plan.Plan{
		Commands: []plan.PlannedCommand{
			{Command: []string{"echo", "first"}, Description: "first"},
			{Command: []string{"echo", "second"}, Description: "second"},
			{Command: []string{"echo", "third"}, Description: "third"},
		},
	}

	var buf bytes.Buffer
	results := engine.RunPlanStreaming(context.Background(), p, &buf)

	testutil.AssertEqual(t, len(results.Items), 3)
	testutil.AssertEqual(t, results.Failed, 0)
}

func TestRunPlanStreaming_WithElevation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real execution in short mode")
	}

	cfg := testutil.DefaultTestConfig()
	// Use "env" as a harmless elevate command that just passes through
	cfg.ElevateCommand = "env"
	engine := New(cfg)

	p := plan.Plan{
		Commands: []plan.PlannedCommand{
			{Command: []string{"echo", "elevated"}, Description: "show config", NeedsRoot: true},
		},
	}

	var buf bytes.Buffer
	results := engine.RunPlanStreaming(context.Background(), p, &buf)

	testutil.AssertEqual(t, results.Failed, 0)
	if !strings.Contains(buf.String(), "elevated") {
		t.Error("expected 'elevated' in output")
	}
}

func TestDefaultRunCommand_OutputTruncation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	// Create a command that outputs more than MaxOutputSize
	// Use "yes" with head to generate large output
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Generate output larger than MaxOutputSize using dd
	output, _ := DefaultRunCommand(ctx, []string{"dd", "if=/dev/zero", "bs=1024", "count=600", "status=none"})

	if len(output) > MaxOutputSize+100 { // Allow small overhead for truncation message
		t.Errorf("expected output truncated near %d bytes, got %d", MaxOutputSize, len(output))
	}
}

func TestFormatCommand_Empty(t *testing.T) {
	result := FormatCommand([]string{})
	testutil.AssertEqual(t, result, "")
}

func TestFormatCommand_NoQuotingNeeded(t *testing.T) {
	result := FormatCommand([]string{"uci", "show", "network"})
	testutil.AssertEqual(t, result, "uci show network")
}

func TestFormatCommand_WithTab(t *testing.T) {
	result := FormatCommand([]string{"echo", "hello\tworld"})
	if !strings.Contains(result, "\"") {
		t.Errorf("expected quoted argument for tab, got %q", result)
	}
}

func TestFieldsSafe(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"sudo", []string{"sudo"}},
		{"sudo -n", []string{"sudo", "-n"}},
		{"  spaces  around  ", []string{"spaces", "around"}},
		{"", nil},
		{"single", []string{"single"}},
	}

	for _, tt := range tests {
		result := fieldsSafe(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("fieldsSafe(%q) = %v, want %v", tt.input, result, tt.expected)
			continue
		}
		for i := range result {
			if result[i] != tt.expected[i] {
				t.Errorf("fieldsSafe(%q)[%d] = %q, want %q", tt.input, i, result[i], tt.expected[i])
			}
		}
	}
}

func TestStringBuilderPool(t *testing.T) {
	// Get a builder from the pool
	b := stringBuilderPool.Get().(*strings.Builder)
	b.Reset()
	b.WriteString("test")
	testutil.AssertEqual(t, b.String(), "test")

	// Put it back
	stringBuilderPool.Put(b)

	// Get again and ensure it can be reused
	b2 := stringBuilderPool.Get().(*strings.Builder)
	b2.Reset()
	b2.WriteString("reused")
	testutil.AssertEqual(t, b2.String(), "reused")
	stringBuilderPool.Put(b2)
}

func TestRunPlanStreaming_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real execution in short mode")
	}

	cfg := testutil.DefaultTestConfig()
	cfg.TimeoutSeconds = 1
	engine := New(cfg)

	p := plan.Plan{
		Commands: []plan.PlannedCommand{
			{Command: []string{"sleep", "100"}, Description: "slow"},
		},
	}

	var buf bytes.Buffer
	results := engine.RunPlanStreaming(context.Background(), p, &buf)

	testutil.AssertEqual(t, results.Failed, 1)
	testutil.AssertError(t, results.Items[0].Err)
}
