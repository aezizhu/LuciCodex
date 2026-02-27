package ui

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aezizhu/LuciCodex/internal/executor"
	"github.com/aezizhu/LuciCodex/internal/plan"
)

func TestPrintResponse_WithSummary(t *testing.T) {
	var buf bytes.Buffer

	p := plan.Plan{
		Summary: "Hello! I can help you configure your router.",
	}

	PrintResponse(&buf, p)
	output := buf.String()

	if !strings.Contains(output, "Hello! I can help you configure your router.") {
		t.Errorf("expected summary in output, got %q", output)
	}
}

func TestPrintResponse_WithWarnings(t *testing.T) {
	var buf bytes.Buffer

	p := plan.Plan{
		Summary:  "Here is your info.",
		Warnings: []string{"Connection may be slow"},
	}

	PrintResponse(&buf, p)
	output := stripAnsi(buf.String())

	if !strings.Contains(output, "Note:") {
		t.Error("expected warnings with 'Note:' prefix")
	}
	if !strings.Contains(output, "Connection may be slow") {
		t.Error("expected warning text in output")
	}
}

func TestPrintResponse_EmptySummary(t *testing.T) {
	var buf bytes.Buffer

	p := plan.Plan{}
	PrintResponse(&buf, p)
	output := buf.String()

	if !strings.Contains(output, "no specific commands are needed") {
		t.Errorf("expected fallback message, got %q", output)
	}
}

func TestPrintPlan_EmptyCommands(t *testing.T) {
	var buf bytes.Buffer

	p := plan.Plan{
		Summary:  "Nothing to do",
		Commands: []plan.PlannedCommand{},
	}

	PrintPlan(&buf, p)
	output := stripAnsi(buf.String())

	if !strings.Contains(output, "No commands to execute") {
		t.Error("expected 'No commands to execute' message")
	}
}

func TestPrintPlan_NoDescription(t *testing.T) {
	var buf bytes.Buffer

	p := plan.Plan{
		Commands: []plan.PlannedCommand{
			{Command: []string{"echo", "test"}, Description: ""},
		},
	}

	PrintPlan(&buf, p)
	output := stripAnsi(buf.String())

	// Should still show the command
	if !strings.Contains(output, "echo test") {
		t.Error("expected command in output")
	}
	// Should not show description arrow if description is empty
	if strings.Contains(output, "→") {
		t.Error("should not show description arrow when description is empty")
	}
}

func TestPrintSummary_AllSuccess(t *testing.T) {
	var buf bytes.Buffer

	res := executor.Results{
		Items: []executor.Result{
			{Index: 0, Command: []string{"echo", "1"}, Elapsed: time.Millisecond},
			{Index: 1, Command: []string{"echo", "2"}, Elapsed: time.Millisecond},
		},
		Failed: 0,
	}

	PrintSummary(&buf, res)
	output := stripAnsi(buf.String())

	if !strings.Contains(output, "All 2 command(s) executed successfully") {
		t.Errorf("expected success summary, got %q", output)
	}
}

func TestPrintSummary_WithFailures(t *testing.T) {
	var buf bytes.Buffer

	res := executor.Results{
		Items: []executor.Result{
			{Index: 0, Command: []string{"echo", "1"}, Elapsed: time.Millisecond},
			{Index: 1, Command: []string{"false"}, Err: errors.New("fail"), Elapsed: time.Millisecond},
		},
		Failed: 1,
	}

	PrintSummary(&buf, res)
	output := stripAnsi(buf.String())

	if !strings.Contains(output, "1 of 2 command(s) failed") {
		t.Errorf("expected failure summary, got %q", output)
	}
}

func TestPrintSummary_Empty(t *testing.T) {
	var buf bytes.Buffer

	res := executor.Results{}
	PrintSummary(&buf, res)

	// Should not output anything for empty results
	if buf.Len() != 0 {
		t.Errorf("expected no output for empty results, got %q", buf.String())
	}
}

func TestPrintAnswer_Basic(t *testing.T) {
	var buf bytes.Buffer

	PrintAnswer(&buf, "Your IP address is 192.168.1.1", nil)
	output := stripAnsi(buf.String())

	if !strings.Contains(output, "Answer:") {
		t.Error("expected Answer: header")
	}
	if !strings.Contains(output, "192.168.1.1") {
		t.Error("expected IP in answer")
	}
}

func TestPrintAnswer_WithDetails(t *testing.T) {
	var buf bytes.Buffer

	details := []string{"Interface: eth0", "Subnet: 255.255.255.0"}
	PrintAnswer(&buf, "Network info collected", details)
	output := stripAnsi(buf.String())

	if !strings.Contains(output, "Network info collected") {
		t.Error("expected summary in output")
	}
	if !strings.Contains(output, "Interface: eth0") {
		t.Error("expected first detail")
	}
	if !strings.Contains(output, "Subnet: 255.255.255.0") {
		t.Error("expected second detail")
	}
}

func TestPrintAnswer_NoDetails(t *testing.T) {
	var buf bytes.Buffer

	PrintAnswer(&buf, "Everything is fine", []string{})
	output := buf.String()

	// With empty details slice, bullet points should not appear
	if strings.Contains(output, "•") {
		t.Error("should not show bullet points with empty details")
	}
}

func TestColorize(t *testing.T) {
	result := Colorize(Red, "error")
	if result != "\033[31merror\033[0m" {
		t.Errorf("unexpected colorize result: %q", result)
	}

	result2 := Colorize(Green+Bold, "success")
	if !strings.HasPrefix(result2, "\033[32m\033[1m") {
		t.Errorf("unexpected colorize result: %q", result2)
	}
	if !strings.HasSuffix(result2, "\033[0m") {
		t.Errorf("expected Reset suffix in: %q", result2)
	}
}

func TestPrintResults_WithOutput(t *testing.T) {
	var buf bytes.Buffer

	res := executor.Results{
		Items: []executor.Result{
			{
				Index:   0,
				Command: []string{"uci", "show", "network"},
				Output:  "network.lan=interface\nnetwork.lan.proto=dhcp",
				Elapsed: 50 * time.Millisecond,
			},
		},
		Failed: 0,
	}

	PrintResults(&buf, res)
	output := stripAnsi(buf.String())

	if !strings.Contains(output, "network.lan=interface") {
		t.Error("expected command output to be displayed")
	}
	if !strings.Contains(output, "network.lan.proto=dhcp") {
		t.Error("expected full output to be displayed")
	}
}

func TestPrintResults_EmptyOutput(t *testing.T) {
	var buf bytes.Buffer

	res := executor.Results{
		Items: []executor.Result{
			{
				Index:   0,
				Command: []string{"true"},
				Output:  "",
				Elapsed: time.Millisecond,
			},
		},
		Failed: 0,
	}

	PrintResults(&buf, res)
	output := stripAnsi(buf.String())

	// Should not have indented empty output
	if !strings.Contains(output, "All commands executed successfully") {
		t.Error("expected success message")
	}
}
