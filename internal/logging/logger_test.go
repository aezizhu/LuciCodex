package logging

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aezizhu/LuciCodex/internal/plan"
)

func TestLogger_WriteJSON(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")

	logger := New(logFile)

	// Test logging a plan
	testPlan := plan.Plan{
		Commands: []plan.PlannedCommand{
			{Command: []string{"echo", "hello"}},
		},
		Summary: "Test plan",
	}
	logger.Plan("test prompt", testPlan)

	// Test logging results
	testResults := []ResultItem{
		{Index: 0, Command: []string{"echo", "hello"}, Output: "hello\n", Elapsed: 100 * time.Millisecond},
	}
	logger.Results(testResults)

	// Read the log file and verify its content
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 log lines, but got %d", len(lines))
	}

	// Verify the first log entry (plan)
	var planEntry map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &planEntry); err != nil {
		t.Fatalf("failed to unmarshal plan log entry: %v", err)
	}
	if planEntry["event"] != "plan" {
		t.Errorf("expected event 'plan', got '%s'", planEntry["event"])
	}

	// Verify the second log entry (results)
	var resultsEntry map[string]interface{}
	if err := json.Unmarshal([]byte(lines[1]), &resultsEntry); err != nil {
		t.Fatalf("failed to unmarshal results log entry: %v", err)
	}
	if resultsEntry["event"] != "results" {
		t.Errorf("expected event 'results', got '%s'", resultsEntry["event"])
	}
}

func TestLogger_NoPath(t *testing.T) {
	// This test ensures that creating a logger with an empty path
	// does not cause a panic when its methods are called.
	logger := New("")
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("The code panicked when logging with an empty path: %v", r)
		}
	}()

	logger.Plan("test prompt", plan.Plan{})
	logger.Results([]ResultItem{})
}

func TestLogger_Concurrency(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "concurrent.log")
	logger := New(logFile)

	var wg sync.WaitGroup
	numRoutines := 50

	for i := 0; i < numRoutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			logger.Plan("concurrent prompt", plan.Plan{Summary: "concurrent"})
		}()
	}

	wg.Wait()

	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != numRoutines {
		t.Errorf("expected %d log lines, but got %d", numRoutines, len(lines))
	}
}
