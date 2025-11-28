package metrics

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aezizhu/LuciCodex/internal/plan"
)

func TestRecordRequest(t *testing.T) {
	c := NewCollector("") // No file path for this test
	c.Stop()              // Stop periodic saving for predictable test

	p := plan.Plan{
		Commands: []plan.PlannedCommand{
			{Command: []string{"ls", "-l"}},
			{Command: []string{"cat", "file"}},
		},
	}
	testErr := errors.New("test error")

	// Record one successful request
	c.RecordRequest("provider1", "prompt1", p, 100*time.Millisecond, nil)
	// Record one failed request
	c.RecordRequest("provider1", "prompt2", p, 200*time.Millisecond, testErr)
	// Record another successful request from a different provider
	c.RecordRequest("provider2", "prompt3", p, 150*time.Millisecond, nil)

	m := c.GetMetrics()

	if m.TotalRequests != 3 {
		t.Errorf("expected TotalRequests to be 3, got %d", m.TotalRequests)
	}
	if m.TotalCommands != 6 {
		t.Errorf("expected TotalCommands to be 6, got %d", m.TotalCommands)
	}
	if m.SuccessfulRuns != 2 {
		t.Errorf("expected SuccessfulRuns to be 2, got %d", m.SuccessfulRuns)
	}
	if m.FailedRuns != 1 {
		t.Errorf("expected FailedRuns to be 1, got %d", m.FailedRuns)
	}
	if m.ProviderUsage["provider1"] != 2 {
		t.Errorf("expected provider1 usage to be 2, got %d", m.ProviderUsage["provider1"])
	}
	if m.ProviderUsage["provider2"] != 1 {
		t.Errorf("expected provider2 usage to be 1, got %d", m.ProviderUsage["provider2"])
	}
	if m.CommandPatterns["ls"] != 3 {
		t.Errorf("expected 'ls' pattern count to be 3, got %d", m.CommandPatterns["ls"])
	}
	if m.ErrorTypes["*errors.errorString"] != 1 {
		t.Errorf("expected error type count to be 1, got %d", m.ErrorTypes["*errors.errorString"])
	}
	expectedTotalDuration := 450 * time.Millisecond
	if m.TotalDuration != expectedTotalDuration {
		t.Errorf("expected TotalDuration to be %v, got %v", expectedTotalDuration, m.TotalDuration)
	}
	expectedAvgDuration := expectedTotalDuration / 3
	if m.AverageDuration != expectedAvgDuration {
		t.Errorf("expected AverageDuration to be %v, got %v", expectedAvgDuration, m.AverageDuration)
	}
	if len(m.RecentRequests) != 3 {
		t.Errorf("expected 3 recent requests, got %d", len(m.RecentRequests))
	}
}

func TestSaveAndLoad(t *testing.T) {
	tempDir := t.TempDir()
	metricsFile := filepath.Join(tempDir, "metrics.json")

	// Create a collector, record some data, and save it
	c1 := NewCollector(metricsFile)
	p := plan.Plan{Commands: []plan.PlannedCommand{{Command: []string{"ls"}}}}
	c1.RecordRequest("provider1", "prompt1", p, 100*time.Millisecond, nil)
	c1.Stop() // Stops periodic save and does a final save

	// Wait a moment to ensure file is written
	time.Sleep(100 * time.Millisecond)

	// Create a new collector and load the file
	c2 := NewCollector(metricsFile)
	defer c2.Stop()

	m2 := c2.GetMetrics()

	if m2.TotalRequests != 1 {
		t.Fatalf("expected TotalRequests to be 1 after loading, got %d", m2.TotalRequests)
	}
	if m2.ProviderUsage["provider1"] != 1 {
		t.Errorf("expected provider1 usage to be 1 after loading, got %d", m2.ProviderUsage["provider1"])
	}
}

func TestGetSummary(t *testing.T) {
	c := NewCollector("")
	c.Stop()

	p1 := plan.Plan{Commands: []plan.PlannedCommand{{Command: []string{"ls"}}}}
	p2 := plan.Plan{Commands: []plan.PlannedCommand{{Command: []string{"cat"}}}}
	c.RecordRequest("p1", "prompt", p1, 100*time.Millisecond, nil)
	c.RecordRequest("p1", "prompt", p1, 150*time.Millisecond, nil)
	c.RecordRequest("p2", "prompt", p2, 200*time.Millisecond, errors.New("fail"))
	c.RecordRequest("p1", "prompt", p1, 50*time.Millisecond, nil)

	summary := c.GetSummary()

	if summary.TotalRequests != 4 {
		t.Errorf("expected summary TotalRequests to be 4, got %d", summary.TotalRequests)
	}
	if summary.TopProvider != "p1" {
		t.Errorf("expected summary TopProvider to be 'p1', got '%s'", summary.TopProvider)
	}
	// ls appears 3 times, cat appears 1 time - ls should be top
	if summary.TopCommand != "ls" {
		t.Errorf("expected summary TopCommand to be 'ls', got '%s'", summary.TopCommand)
	}
	// 3 successful runs out of 4 total requests
	if summary.SuccessRate != 75.0 {
		t.Errorf("expected summary SuccessRate to be 75.0, got %f", summary.SuccessRate)
	}
}

func TestConcurrency(t *testing.T) {
	c := NewCollector("")
	defer c.Stop()

	var wg sync.WaitGroup
	numRoutines := 100
	p := plan.Plan{Commands: []plan.PlannedCommand{{Command: []string{"test"}}}}

	for i := 0; i < numRoutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.RecordRequest("concurrent-provider", "prompt", p, 10*time.Millisecond, nil)
		}()
	}

	wg.Wait()

	m := c.GetMetrics()
	if m.TotalRequests != int64(numRoutines) {
		t.Errorf("expected TotalRequests to be %d after concurrent runs, got %d", numRoutines, m.TotalRequests)
	}
	if m.TotalDuration != time.Duration(numRoutines)*10*time.Millisecond {
		t.Errorf("expected TotalDuration to be %v, got %v", time.Duration(numRoutines)*10*time.Millisecond, m.TotalDuration)
	}
}

func TestCollector_FullBuffer(t *testing.T) {
	c := NewCollector("")
	c.Stop()

	// Fill buffer + 1
	p := plan.Plan{}
	for i := 0; i < 101; i++ {
		c.RecordRequest("p", "prompt", p, 0, nil)
	}

	m := c.GetMetrics()
	if len(m.RecentRequests) != 100 {
		t.Errorf("expected 100 recent requests, got %d", len(m.RecentRequests))
	}
}

func TestCollector_Errors(t *testing.T) {
	// Test Save error (invalid path)
	c := NewCollector("/invalid/path/metrics.json")
	c.Stop()
	if err := c.Save(); err == nil {
		t.Error("expected error saving to invalid path")
	}

	// Test Load error (corrupt file)
	tmpFile := filepath.Join(t.TempDir(), "corrupt.json")
	if err := os.WriteFile(tmpFile, []byte("{invalid json"), 0644); err != nil {
		t.Fatal(err)
	}

	// Manually create collector to avoid NewCollector starting periodicSave which overwrites file
	c = &Collector{
		metrics:  &Metrics{},
		filePath: tmpFile,
	}

	if err := c.Load(); err == nil {
		t.Error("expected error loading corrupt file")
	}

	// Test Load error (permission denied)
	// Hard to simulate portably without root/chmod tricks that might fail in containers.
	// But corrupt file covers json.Unmarshal error.
	// os.ReadFile error is covered by IsNotExist check (returns nil).
	// We need a non-NotExist error.
	if err := os.Chmod(tmpFile, 0000); err == nil {
		// If chmod succeeds, try to load
		if err := c.Load(); err == nil {
			t.Error("expected error loading unreadable file")
		}
		os.Chmod(tmpFile, 0644) // Restore
	}
}

func TestCollector_LongPrompt(t *testing.T) {
	c := NewCollector("")
	c.Stop()

	longPrompt := strings.Repeat("a", 200)
	c.RecordRequest("p", longPrompt, plan.Plan{}, 0, nil)

	m := c.GetMetrics()
	if len(m.RecentRequests) != 1 {
		t.Fatal("expected 1 request")
	}

	got := m.RecentRequests[0].Prompt
	if len(got) > 100 {
		t.Errorf("expected prompt length <= 100, got %d", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Error("expected prompt to be truncated with ...")
	}
}
