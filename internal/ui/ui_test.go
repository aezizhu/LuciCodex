package ui

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aezizhu/LuciCodex/internal/executor"
	"github.com/aezizhu/LuciCodex/internal/plan"
)

func stripAnsi(s string) string {
	// Simple regex to strip ANSI codes
	return strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(s, "\033[0m", ""), "\033[31m", ""), "\033[32m", ""), "\033[33m", ""), "\033[34m", ""), "\033[1m", "")
}

func TestPrintPlan(t *testing.T) {
	var buf bytes.Buffer

	p := plan.Plan{
		Summary: "Test plan summary",
		Commands: []plan.PlannedCommand{
			{
				Command:     []string{"echo", "hello"},
				Description: "Print hello",
			},
			{
				Command:     []string{"ls", "-la"},
				Description: "List files",
			},
		},
		Warnings: []string{"Warning 1", "Warning 2"},
	}

	PrintPlan(&buf, p)
	output := stripAnsi(buf.String())

	if !strings.Contains(output, "Summary: Test plan summary") {
		t.Errorf("expected to see summary in output")
	}
	if !strings.Contains(output, "[1] echo hello") {
		t.Errorf("expected to see first command")
	}
	if !strings.Contains(output, "Print hello") {
		t.Errorf("expected to see first command description")
	}
	if !strings.Contains(output, "[2] ls -la") {
		t.Errorf("expected to see second command")
	}
	if !strings.Contains(output, "Warnings:") {
		t.Errorf("expected to see warnings header")
	}
	if !strings.Contains(output, "⚠ Warning 1") {
		t.Errorf("expected to see first warning")
	}
	if !strings.Contains(output, "⚠ Warning 2") {
		t.Errorf("expected to see second warning")
	}
}

func TestPrintPlan_NoWarnings(t *testing.T) {
	var buf bytes.Buffer

	p := plan.Plan{
		Summary: "Simple plan",
		Commands: []plan.PlannedCommand{
			{Command: []string{"echo", "test"}},
		},
	}

	PrintPlan(&buf, p)
	output := stripAnsi(buf.String())

	if strings.Contains(output, "Warnings:") {
		t.Errorf("should not show warnings section when there are no warnings")
	}
}

func TestConfirm_Yes(t *testing.T) {
	testCases := []struct {
		input    string
		expected bool
	}{
		{"y\n", true},
		{"yes\n", true},
		{"Y\n", true},
		{"YES\n", true},
		{"n\n", false},
		{"no\n", false},
		{"N\n", false},
		{"\n", false},
		{"maybe\n", false},
	}

	for _, tc := range testCases {
		reader := bufio.NewReader(strings.NewReader(tc.input))
		var buf bytes.Buffer

		result, err := Confirm(reader, &buf, "Test prompt")
		if err != nil {
			t.Fatalf("unexpected error for input '%s': %v", tc.input, err)
		}
		if result != tc.expected {
			t.Errorf("for input '%s', expected %v but got %v", tc.input, tc.expected, result)
		}
		// Confirm prompt contains color codes, so we check raw buffer or stripped
		if !strings.Contains(stripAnsi(buf.String()), "Test prompt [y/N]:") {
			t.Errorf("expected to see prompt in output")
		}
	}
}

func TestPrintResults_Success(t *testing.T) {
	var buf bytes.Buffer

	res := executor.Results{
		Items: []executor.Result{
			{
				Index:   0,
				Command: []string{"echo", "hello"},
				Output:  "hello\n",
				Elapsed: 10 * time.Millisecond,
			},
			{
				Index:   1,
				Command: []string{"pwd"},
				Output:  "/home/user\n",
				Elapsed: 5 * time.Millisecond,
			},
		},
		Failed: 0,
	}

	PrintResults(&buf, res)
	output := stripAnsi(buf.String())

	if !strings.Contains(output, "[1] (ok, 10ms) echo hello") {
		t.Errorf("expected to see first command result, got: %s", output)
	}
	if !strings.Contains(output, "  hello") {
		t.Errorf("expected to see indented output")
	}
	if !strings.Contains(output, "[2] (ok, 5ms) pwd") {
		t.Errorf("expected to see second command result")
	}
	if !strings.Contains(output, "All commands executed successfully") {
		t.Errorf("expected to see success message")
	}
}

func TestPrintResults_WithFailures(t *testing.T) {
	var buf bytes.Buffer

	res := executor.Results{
		Items: []executor.Result{
			{
				Index:   0,
				Command: []string{"false"},
				Err:     errors.New("command failed"),
				Elapsed: 10 * time.Millisecond,
			},
		},
		Failed: 1,
	}

	PrintResults(&buf, res)
	output := stripAnsi(buf.String())

	if !strings.Contains(output, "[1] (error, 10ms) false") {
		t.Errorf("expected to see error status")
	}
	if !strings.Contains(output, "1 command(s) failed") {
		t.Errorf("expected to see failure message")
	}
	if strings.Contains(output, "All commands executed successfully") {
		t.Errorf("should not show success message when commands failed")
	}
}

func TestIndent(t *testing.T) {
	testCases := []struct {
		input    string
		n        int
		expected string
	}{
		{"hello\nworld", 2, "  hello\n  world"},
		{"single line", 4, "    single line"},
		{"trailing\nnewline\n", 2, "  trailing\n  newline"},
		{"", 2, "  "}, // Empty string gets indented to spaces
	}

	for _, tc := range testCases {
		result := indent(tc.input, tc.n)
		if result != tc.expected {
			t.Errorf("indent(%q, %d) = %q, want %q", tc.input, tc.n, result, tc.expected)
		}
	}
}

func TestPrintPlanJSON(t *testing.T) {
	var buf bytes.Buffer

	p := plan.Plan{
		Summary: "Test plan",
		Commands: []plan.PlannedCommand{
			{
				Command:     []string{"echo", "test"},
				Description: "Test command",
			},
		},
	}

	err := PrintPlanJSON(&buf, p)
	if err != nil {
		t.Fatalf("PrintPlanJSON failed: %v", err)
	}

	// Verify it's valid JSON
	var decoded plan.Plan
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if decoded.Summary != "Test plan" {
		t.Errorf("expected summary 'Test plan', got '%s'", decoded.Summary)
	}
	if len(decoded.Commands) != 1 {
		t.Errorf("expected 1 command, got %d", len(decoded.Commands))
	}
}

func TestPrintResultsJSON(t *testing.T) {
	var buf bytes.Buffer

	res := executor.Results{
		Items: []executor.Result{
			{
				Index:   0,
				Command: []string{"echo", "test"},
				Output:  "test\n",
				Elapsed: 10 * time.Millisecond,
			},
		},
		Failed: 0,
	}

	err := PrintResultsJSON(&buf, res)
	if err != nil {
		t.Fatalf("PrintResultsJSON failed: %v", err)
	}

	// Verify it's valid JSON
	var decoded executor.Results
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if len(decoded.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(decoded.Items))
	}
	if decoded.Failed != 0 {
		t.Errorf("expected 0 failures, got %d", decoded.Failed)
	}
}
