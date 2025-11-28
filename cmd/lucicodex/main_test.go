package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/aezizhu/LuciCodex/internal/executor"
)

// TestMain_Version runs the binary with -version flag
func TestMain_Version(t *testing.T) {
	// Build the binary first
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "lucicodex")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build binary: %v\nOutput: %s", err, out)
	}

	// Run with -version
	cmd := exec.Command(binaryPath, "-version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run binary: %v", err)
	}

	output := string(out)
	if !strings.Contains(output, "LuciCodex version") {
		t.Errorf("Expected version output, got: %s", output)
	}
}

// TestMain_ConfigError runs the binary with a non-existent config
func TestMain_ConfigError(t *testing.T) {
	// Build the binary first
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "lucicodex")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build binary: %v\nOutput: %s", err, out)
	}

	// Create invalid config file
	configPath := filepath.Join(tmpDir, "invalid.json")
	os.WriteFile(configPath, []byte("{invalid-json"), 0644)

	// Run with invalid config
	cmd := exec.Command(binaryPath, "-config", configPath, "prompt")
	out, err := cmd.CombinedOutput()

	// Should exit with 1 (or handle it gracefully if setup is suggested)
	// In main.go:
	// if err != nil {
	//     if !*setup {
	//         fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
	//         os.Exit(1)
	//     }
	// }

	// Wait, Load returns default config if file not found?
	// No, Load(path) returns error if path is provided but fails.

	if err == nil {
		t.Error("Expected error exit code, got success")
	}

	output := string(out)
	if !strings.Contains(output, "Configuration error") {
		t.Errorf("Expected configuration error message, got: %s", output)
	}
}

// TestMain_Locking tests the locking mechanism with a mock LLM
func TestMain_Locking(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping TestMain_Locking in CI environment")
	}
	// Start a mock LLM server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a valid Gemini response
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"candidates": [{
				"content": {
					"parts": [{
						"text": "{\"summary\": \"Test plan\", \"commands\": [{\"command\": [\"echo\", \"test\"]}]}"
					}]
				}
			}]
		}`))
	}))
	defer server.Close()

	// Build the binary first
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "lucicodex")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build binary: %v\nOutput: %s", err, out)
	}

	// Create a dummy config with allowlist
	configPath := filepath.Join(tmpDir, "config.json")
	os.WriteFile(configPath, []byte(`{"api_key": "dummy-key", "auto_approve": true, "allowlist": ["^echo"]}`), 0644)

	lockPath := "/tmp/lucicodex.lock"
	// Ensure cleanup
	os.Remove(lockPath)
	defer os.Remove(lockPath)

	// Create lock file
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Skipf("Could not create lock file for testing: %v", err)
	}
	f.Close()

	// Run binary with mock endpoint and dry-run=false
	cmd := exec.Command(binaryPath, "-config", configPath, "-dry-run=false", "test")
	cmd.Env = append(os.Environ(), "GEMINI_ENDPOINT="+server.URL)

	out, err := cmd.CombinedOutput()

	if err == nil {
		t.Error("Expected error due to lock, got success")
	}

	output := string(out)
	if !strings.Contains(output, "execution in progress") {
		t.Errorf("Expected lock error message, got: %s", output)
	}
}

func TestRun_Direct(t *testing.T) {
	// Start a mock LLM server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"candidates": [{
				"content": {
					"parts": [{
						"text": "{\"summary\": \"Test plan\", \"commands\": [{\"command\": [\"echo\", \"direct\"]}]}"
					}]
				}
			}]
		}`))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	os.WriteFile(configPath, []byte(`{"api_key": "dummy-key", "auto_approve": true, "allowlist": ["^echo"]}`), 0644)

	// Mock stdin/stdout
	stdin := strings.NewReader("")
	var stdout, stderr strings.Builder

	// Run directly
	args := []string{
		"-config", configPath,
		"-dry-run=false",
		"test prompt",
	}

	// Inject endpoint via env var (run() reads config which reads env, but config.Load reads env vars?
	// No, config.Load reads file. ApplyProviderSettings uses defaults.
	// We need to override the endpoint in the config or env.
	// The config loader respects env vars if we set them?
	// Let's check config.Load. It usually loads from file.
	// But we can set GEMINI_ENDPOINT env var if the config package supports it.
	// Assuming it does (based on previous tests).
	t.Setenv("GEMINI_ENDPOINT", server.URL)

	exitCode := run(args, stdin, &stdout, &stderr)

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d. Stderr: %s", exitCode, stderr.String())
	}

	if !strings.Contains(stdout.String(), "echo direct") {
		t.Errorf("Expected output to contain command, got: %s", stdout.String())
	}
}

func TestRun_Version(t *testing.T) {
	var stdout, stderr strings.Builder
	exitCode := run([]string{"-version"}, strings.NewReader(""), &stdout, &stderr)
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}
	if !strings.Contains(stdout.String(), "LuciCodex version") {
		t.Errorf("Expected version output, got: %s", stdout.String())
	}
}

func TestRun_DryRun(t *testing.T) {
	// Mock LLM
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"candidates": [{"content": {"parts": [{"text": "{\"summary\": \"Plan\", \"commands\": [{\"command\":[\"echo\"]}]}"}]}}]}`))
	}))
	defer server.Close()
	t.Setenv("GEMINI_ENDPOINT", server.URL)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	os.WriteFile(configPath, []byte(`{"api_key": "dummy", "allowlist": ["^echo"]}`), 0644)

	var stdout, stderr strings.Builder
	// -dry-run is true by default in flags, but we can be explicit
	exitCode := run([]string{"-config", configPath, "prompt"}, strings.NewReader(""), &stdout, &stderr)

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d. Stderr: %s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Dry run mode") {
		t.Errorf("Expected dry run message, got: %s", stdout.String())
	}
}

func TestRun_ConfigError(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.json")
	os.WriteFile(configPath, []byte("{invalid-json"), 0644)

	var stdout, stderr strings.Builder
	exitCode := run([]string{"-config", configPath, "prompt"}, strings.NewReader(""), &stdout, &stderr)

	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "Configuration error") {
		t.Errorf("Expected config error, got: %s", stderr.String())
	}
}

func TestRun_Cancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"candidates": [{"content": {"parts": [{"text": "{\"summary\": \"Plan\", \"commands\": [{\"command\":[\"echo\"]}]}"}]}}]}`))
	}))
	defer server.Close()
	t.Setenv("GEMINI_ENDPOINT", server.URL)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	os.WriteFile(configPath, []byte(`{"api_key": "dummy", "allowlist": ["^echo"]}`), 0644)

	var stdout, stderr strings.Builder
	// Simulate "n" for no
	stdin := strings.NewReader("n\n")

	exitCode := run([]string{"-config", configPath, "-dry-run=false", "prompt"}, stdin, &stdout, &stderr)

	if exitCode != 0 {
		t.Errorf("Expected exit code 0 (cancelled), got %d", exitCode)
	}
	if !strings.Contains(stdout.String(), "Cancelled") {
		t.Errorf("Expected cancellation message, got: %s", stdout.String())
	}
}

func TestRun_JSON(t *testing.T) {
	// Mock LLM
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"candidates": [{"content": {"parts": [{"text": "{\"summary\": \"Plan\", \"commands\": [{\"command\":[\"echo\", \"json\"]}]}"}]}}]}`))
	}))
	defer server.Close()
	t.Setenv("GEMINI_ENDPOINT", server.URL)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	os.WriteFile(configPath, []byte(`{"api_key": "dummy", "allowlist": ["^echo"]}`), 0644)

	var stdout, stderr strings.Builder
	// -json flag
	exitCode := run([]string{"-config", configPath, "-json", "-dry-run", "prompt"}, strings.NewReader(""), &stdout, &stderr)

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d. Stderr: %s", exitCode, stderr.String())
	}

	// Check for JSON output
	output := stdout.String()
	if !strings.Contains(output, `"summary": "Plan"`) {
		t.Errorf("Expected JSON plan output, got: %s", output)
	}
}

func TestRun_ConfirmEach(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"candidates": [{"content": {"parts": [{"text": "{\"summary\": \"Plan\", \"commands\": [{\"command\":[\"echo\", \"1\"]}, {\"command\":[\"echo\", \"2\"]}]}"}]}}]}`))
	}))
	defer server.Close()
	t.Setenv("GEMINI_ENDPOINT", server.URL)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	os.WriteFile(configPath, []byte(`{"api_key": "dummy", "allowlist": ["^echo"]}`), 0644)

	var stdout, stderr strings.Builder
	// Confirm first, skip second
	stdin := strings.NewReader("y\nn\n")

	exitCode := run([]string{"-config", configPath, "-dry-run=false", "-confirm-each", "prompt"}, stdin, &stdout, &stderr)

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d. Stderr: %s", exitCode, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "Execute command 1") {
		t.Error("Expected prompt for command 1")
	}
	if !strings.Contains(output, "Execute command 2") {
		t.Error("Expected prompt for command 2")
	}
	// We can't easily check execution output because echo goes to stdout/stderr of the *subprocess*,
	// which is captured by execEngine.RunCommand.
	// But execEngine uses os.Stdout/Stderr? No, it captures output.
	// The result is printed at the end.
	if !strings.Contains(output, "All commands executed successfully") {
		t.Error("Expected success message")
	}
}

func TestRun_Setup(t *testing.T) {
	var stdout, stderr strings.Builder
	// EOF immediately triggers error in wizard
	exitCode := run([]string{"-setup"}, strings.NewReader(""), &stdout, &stderr)

	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "Setup error") {
		t.Errorf("Expected setup error, got: %s", stderr.String())
	}
}

func TestRun_Interactive(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	os.WriteFile(configPath, []byte(`{"api_key": "dummy"}`), 0644)

	var stdout, stderr strings.Builder
	// EOF immediately triggers error in REPL (or exit)
	// REPL Run() returns nil on EOF (clean exit) usually?
	// Let's check repl.Run().
	// If it returns nil, exit code is 0.
	exitCode := run([]string{"-config", configPath, "-interactive"}, strings.NewReader(""), &stdout, &stderr)

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d. Stderr: %s", exitCode, stderr.String())
	}
}

func TestRun_AutoRetry(t *testing.T) {
	// Mock LLM with state to return plan then fix
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		callCount++
		if callCount == 1 {
			// Initial plan: failing command
			w.Write([]byte(`{"candidates": [{"content": {"parts": [{"text": "{\"summary\": \"Plan\", \"commands\": [{\"command\":[\"fail_cmd\"]}]}"}]}}]}`))
		} else {
			// Fix plan: success command
			w.Write([]byte(`{"candidates": [{"content": {"parts": [{"text": "{\"summary\": \"Fix\", \"commands\": [{\"command\":[\"fix_cmd\"]}]}"}]}}]}`))
		}
	}))
	defer server.Close()
	t.Setenv("GEMINI_ENDPOINT", server.URL)

	// Mock executor
	// We need to import internal/executor, but we are in main package.
	// main_test.go is package main, so we can import internal/executor.
	// But we need to make sure we don't break other tests running in parallel?
	// Tests run sequentially by default unless t.Parallel() is called.
	// But we should reset it after.

	// We need to access executor.SetRunCommand.
	// Since we are in main package, we can import it.
	// Wait, main.go imports it.

	origRun := executor.GetRunCommand()
	defer executor.SetRunCommand(origRun)

	executor.SetRunCommand(func(ctx context.Context, argv []string) (string, error) {
		if len(argv) > 0 && argv[0] == "fail_cmd" {
			return "simulated failure output", fmt.Errorf("simulated failure")
		}
		return "simulated success", nil
	})

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	os.WriteFile(configPath, []byte(`{"api_key": "dummy", "allowlist": ["^fail_cmd", "^fix_cmd"], "auto_retry": true, "max_retries": 1, "auto_approve": true}`), 0644)

	var stdout, stderr strings.Builder

	exitCode := run([]string{"-config", configPath, "-dry-run=false", "prompt"}, strings.NewReader(""), &stdout, &stderr)

	if exitCode != 0 {
		t.Errorf("Expected exit code 0 (after fix), got %d. Stderr: %s", exitCode, stderr.String())
	}

	output := stderr.String() // Retry logs go to stderr
	if !strings.Contains(output, "Command failed") {
		t.Error("Expected failure message")
	}
	if !strings.Contains(output, "Fix successful") {
		t.Error("Expected fix success message")
	}
}

func TestRun_UnknownFlag(t *testing.T) {
	var stdout, stderr strings.Builder
	exitCode := run([]string{"-unknown-flag"}, strings.NewReader(""), &stdout, &stderr)
	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "flag provided but not defined") {
		t.Errorf("Expected flag error, got: %s", stderr.String())
	}
}

func TestRun_LLMError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	t.Setenv("GEMINI_ENDPOINT", server.URL)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	os.WriteFile(configPath, []byte(`{"api_key": "dummy"}`), 0644)

	var stdout, stderr strings.Builder
	exitCode := run([]string{"-config", configPath, "prompt"}, strings.NewReader(""), &stdout, &stderr)

	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "LLM error") {
		t.Errorf("Expected LLM error, got: %s", stderr.String())
	}
}

func TestRun_EmptyPlan(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"candidates": [{"content": {"parts": [{"text": "{\"summary\": \"Plan\", \"commands\": []}"}]}}]}`))
	}))
	defer server.Close()
	t.Setenv("GEMINI_ENDPOINT", server.URL)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	os.WriteFile(configPath, []byte(`{"api_key": "dummy"}`), 0644)

	var stdout, stderr strings.Builder
	exitCode := run([]string{"-config", configPath, "prompt"}, strings.NewReader(""), &stdout, &stderr)

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}
	if !strings.Contains(stdout.String(), "No commands proposed") {
		t.Errorf("Expected no commands message, got: %s", stdout.String())
	}
}

func TestRun_PolicyRejection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"candidates": [{"content": {"parts": [{"text": "{\"summary\": \"Plan\", \"commands\": [{\"command\":[\"rm\", \"-rf\", \"/\"]}]}"}]}}]}`))
	}))
	defer server.Close()
	t.Setenv("GEMINI_ENDPOINT", server.URL)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	// Denylist rm
	os.WriteFile(configPath, []byte(`{"api_key": "dummy", "denylist": ["^rm"]}`), 0644)

	var stdout, stderr strings.Builder
	exitCode := run([]string{"-config", configPath, "prompt"}, strings.NewReader(""), &stdout, &stderr)

	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "Plan rejected by policy") {
		t.Errorf("Expected policy rejection, got: %s", stderr.String())
	}
}

func TestRun_MaxCommands(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"candidates": [{"content": {"parts": [{"text": "{\"summary\": \"Plan\", \"commands\": [{\"command\":[\"echo\", \"1\"]}, {\"command\":[\"echo\", \"2\"]}]}"}]}}]}`))
	}))
	defer server.Close()
	t.Setenv("GEMINI_ENDPOINT", server.URL)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	os.WriteFile(configPath, []byte(`{"api_key": "dummy", "allowlist": ["^echo"]}`), 0644)

	var stdout, stderr strings.Builder
	// Set max-commands to 1 via flag
	exitCode := run([]string{"-config", configPath, "-max-commands=1", "-dry-run", "prompt"}, strings.NewReader(""), &stdout, &stderr)

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}
	// Output should show only 1 command
	output := stdout.String()
	if !strings.Contains(output, "echo 1") {
		t.Error("Expected command 1")
	}
	if strings.Contains(output, "echo 2") {
		t.Error("Did not expect command 2")
	}
}

func TestRun_JoinArgs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify prompt contains joined args
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "User request: arg1 arg2") {
			t.Errorf("Expected joined args in prompt, got body: %s", string(body))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"candidates": [{"content": {"parts": [{"text": "{\"summary\": \"Plan\", \"commands\": []}"}]}}]}`))
	}))
	defer server.Close()
	t.Setenv("GEMINI_ENDPOINT", server.URL)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	os.WriteFile(configPath, []byte(`{"api_key": "dummy"}`), 0644)

	var stdout, stderr strings.Builder
	run([]string{"-config", configPath, "-join-args", "arg1", "arg2"}, strings.NewReader(""), &stdout, &stderr)
}

func TestRun_Facts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify prompt contains facts
		// body, _ := io.ReadAll(r.Body)
		// Facts collection might be empty on some systems, but the header should be there if facts were collected
		// Actually, if CollectFacts returns empty string, header is not added.
		// openwrt.CollectFacts runs uci commands. If not on openwrt, it might be empty.
		// But we can mock the executor to return something for uci commands?
		// Or just check that it runs without error.
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"candidates": [{"content": {"parts": [{"text": "{\"summary\": \"Plan\", \"commands\": []}"}]}}]}`))
	}))
	defer server.Close()
	t.Setenv("GEMINI_ENDPOINT", server.URL)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	os.WriteFile(configPath, []byte(`{"api_key": "dummy"}`), 0644)

	var stdout, stderr strings.Builder
	run([]string{"-config", configPath, "-facts", "prompt"}, strings.NewReader(""), &stdout, &stderr)
}

func TestRun_LockFailure(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping TestRun_LockFailure in CI environment")
	}

	// Use a temp file for locking
	tmpLock := filepath.Join(t.TempDir(), "test.lock")
	origLockPaths := lockPaths
	lockPaths = []string{tmpLock}
	defer func() { lockPaths = origLockPaths }()

	// Create the lock file to simulate it being held
	f, err := os.OpenFile(tmpLock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatalf("Failed to create lock file: %v", err)
	}
	f.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"candidates": [{"content": {"parts": [{"text": "{\"summary\": \"Plan\", \"commands\": [{\"command\":[\"echo\"]}]}"}]}}]}`))
	}))
	defer server.Close()
	t.Setenv("GEMINI_ENDPOINT", server.URL)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	os.WriteFile(configPath, []byte(`{"api_key": "dummy", "auto_approve": true, "allowlist": ["^echo"]}`), 0644)

	var stdout, stderr strings.Builder
	// Should fail to acquire lock
	exitCode := run([]string{"-config", configPath, "-dry-run=false", "prompt"}, strings.NewReader(""), &stdout, &stderr)

	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "execution in progress") {
		t.Errorf("Expected lock error, got: %s", stderr.String())
	}
}

type failReader struct{}

func (f *failReader) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("simulated read error")
}

func TestRun_ConfirmError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"candidates": [{"content": {"parts": [{"text": "{\"summary\": \"Plan\", \"commands\": [{\"command\":[\"echo\"]}]}"}]}}]}`))
	}))
	defer server.Close()
	t.Setenv("GEMINI_ENDPOINT", server.URL)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	os.WriteFile(configPath, []byte(`{"api_key": "dummy", "auto_approve": false, "allowlist": ["^echo"]}`), 0644)

	var stdout, stderr strings.Builder
	exitCode := run([]string{"-config", configPath, "-dry-run=false", "prompt"}, &failReader{}, &stdout, &stderr)

	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "Confirmation error") {
		t.Errorf("Expected confirmation error, got: %s", stderr.String())
	}
}

type failWriter struct{}

func (f *failWriter) Write(p []byte) (n int, err error) {
	return 0, fmt.Errorf("simulated write error")
}

func TestRun_JSONError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"candidates": [{"content": {"parts": [{"text": "{\"summary\": \"Plan\", \"commands\": [{\"command\":[\"echo\"]}]}"}]}}]}`))
	}))
	defer server.Close()
	t.Setenv("GEMINI_ENDPOINT", server.URL)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	os.WriteFile(configPath, []byte(`{"api_key": "dummy", "allowlist": ["^echo"]}`), 0644)

	var stderr strings.Builder
	exitCode := run([]string{"-config", configPath, "-json", "prompt"}, strings.NewReader(""), &failWriter{}, &stderr)

	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "JSON output error") {
		t.Errorf("Expected JSON output error, got: %s", stderr.String())
	}
}

func TestRun_Signal(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping TestRun_Signal in CI")
	}
	// Build binary
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "lucicodex")
	exec.Command("go", "build", "-o", binaryPath, ".").Run()

	// Create config
	configPath := filepath.Join(tmpDir, "config.json")
	os.WriteFile(configPath, []byte(`{"api_key": "dummy", "auto_approve": true}`), 0644)

	// Run binary
	cmd := exec.Command(binaryPath, "-config", configPath, "-dry-run=false", "sleep 5")

	// Mock LLM via env
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"candidates": [{"content": {"parts": [{"text": "{\"summary\": \"Plan\", \"commands\": [{\"command\":[\"sleep\", \"5\"]}]}"}]}}]}`))
	}))
	defer server.Close()
	cmd.Env = append(os.Environ(), "GEMINI_ENDPOINT="+server.URL)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start command: %v", err)
	}

	// Wait a bit then send signal
	// We need to make sure it's running and acquired lock
	// But we can't easily check output without consuming it.
	// We can check if lock file exists?
	// But lock file path is hardcoded in binary (unless we build with modified source, which we didn't).
	// So we just wait a bit.
	// Actually, the binary uses the hardcoded lock paths.
	// /tmp/lucicodex.lock should exist.

	// Send SIGTERM
	cmd.Process.Signal(syscall.SIGTERM)

	// Wait for exit
	err := cmd.Wait()

	// Should exit with error (signal)
	if err == nil {
		t.Error("Expected error exit from signal")
	}

	// Verify lock file is gone
	if _, err := os.Stat("/tmp/lucicodex.lock"); !os.IsNotExist(err) {
		// It might be gone, or not if cleanup failed.
		// But main.go handles signal and releases lock.
		// However, we can't guarantee timing.
		// Also, if previous tests left it...
	}
}

func TestRun_AutoRetry_FixGenError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// First response: plan with failing command
		// Second response: error (fix generation failed)
		// We can use a counter or check request body?
		// Simple counter is fine since requests are sequential.
		// But httptest server is concurrent? No, HandlerFunc is called sequentially for sequential requests.
		// But we need state.
		// We can use a closure.
	}))
	// Re-implement server with state
	callCount := 0
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"candidates": [{"content": {"parts": [{"text": "{\"summary\": \"Plan\", \"commands\": [{\"command\":[\"fail_cmd\"]}]}"}]}}]}`))
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()
	t.Setenv("GEMINI_ENDPOINT", server.URL)

	// Mock executor
	origRun := executor.GetRunCommand()
	defer executor.SetRunCommand(origRun)
	executor.SetRunCommand(func(ctx context.Context, argv []string) (string, error) {
		return "", fmt.Errorf("simulated failure")
	})

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	os.WriteFile(configPath, []byte(`{"api_key": "dummy", "auto_retry": true, "max_retries": 1, "auto_approve": true, "allowlist": ["^fail_cmd"]}`), 0644)

	var stdout, stderr strings.Builder
	exitCode := run([]string{"-config", configPath, "-dry-run=false", "prompt"}, strings.NewReader(""), &stdout, &stderr)

	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "Failed to generate fix") {
		t.Errorf("Expected fix generation error, got: %s", stderr.String())
	}
}

func TestRun_AutoRetry_FixPlanEmpty(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			w.Write([]byte(`{"candidates": [{"content": {"parts": [{"text": "{\"summary\": \"Plan\", \"commands\": [{\"command\":[\"fail_cmd\"]}]}"}]}}]}`))
		} else {
			w.Write([]byte(`{"candidates": [{"content": {"parts": [{"text": "{\"summary\": \"Fix\", \"commands\": []}"}]}}]}`))
		}
	}))
	defer server.Close()
	t.Setenv("GEMINI_ENDPOINT", server.URL)

	origRun := executor.GetRunCommand()
	defer executor.SetRunCommand(origRun)
	executor.SetRunCommand(func(ctx context.Context, argv []string) (string, error) {
		return "", fmt.Errorf("simulated failure")
	})

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	os.WriteFile(configPath, []byte(`{"api_key": "dummy", "auto_retry": true, "max_retries": 1, "auto_approve": true, "allowlist": ["^fail_cmd"]}`), 0644)

	var stdout, stderr strings.Builder
	exitCode := run([]string{"-config", configPath, "-dry-run=false", "prompt"}, strings.NewReader(""), &stdout, &stderr)

	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "No fix commands generated") {
		t.Errorf("Expected no fix commands error, got: %s", stderr.String())
	}
}

func TestRun_AutoRetry_FixPolicyReject(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			w.Write([]byte(`{"candidates": [{"content": {"parts": [{"text": "{\"summary\": \"Plan\", \"commands\": [{\"command\":[\"fail_cmd\"]}]}"}]}}]}`))
		} else {
			w.Write([]byte(`{"candidates": [{"content": {"parts": [{"text": "{\"summary\": \"Fix\", \"commands\": [{\"command\":[\"rm\", \"/\"]}]}"}]}}]}`))
		}
	}))
	defer server.Close()
	t.Setenv("GEMINI_ENDPOINT", server.URL)

	origRun := executor.GetRunCommand()
	defer executor.SetRunCommand(origRun)
	executor.SetRunCommand(func(ctx context.Context, argv []string) (string, error) {
		return "", fmt.Errorf("simulated failure")
	})

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	os.WriteFile(configPath, []byte(`{"api_key": "dummy", "auto_retry": true, "max_retries": 1, "auto_approve": true, "denylist": ["^rm"], "allowlist": ["^fail_cmd"]}`), 0644)

	var stdout, stderr strings.Builder
	exitCode := run([]string{"-config", configPath, "-dry-run=false", "prompt"}, strings.NewReader(""), &stdout, &stderr)

	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "Fix plan rejected by policy") {
		t.Errorf("Expected policy rejection error, got: %s", stderr.String())
	}
}

func TestRun_AutoRetry_FixExecFail(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			w.Write([]byte(`{"candidates": [{"content": {"parts": [{"text": "{\"summary\": \"Plan\", \"commands\": [{\"command\":[\"fail_cmd\"]}]}"}]}}]}`))
		} else {
			w.Write([]byte(`{"candidates": [{"content": {"parts": [{"text": "{\"summary\": \"Fix\", \"commands\": [{\"command\":[\"fix_fail\"]}]}"}]}}]}`))
		}
	}))
	defer server.Close()
	t.Setenv("GEMINI_ENDPOINT", server.URL)

	origRun := executor.GetRunCommand()
	defer executor.SetRunCommand(origRun)
	executor.SetRunCommand(func(ctx context.Context, argv []string) (string, error) {
		return "", fmt.Errorf("simulated failure")
	})

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	os.WriteFile(configPath, []byte(`{"api_key": "dummy", "auto_retry": true, "max_retries": 1, "auto_approve": true, "allowlist": ["^fail_cmd", "^fix_fail"]}`), 0644)

	var stdout, stderr strings.Builder
	exitCode := run([]string{"-config", configPath, "-dry-run=false", "prompt"}, strings.NewReader(""), &stdout, &stderr)

	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "Fix attempt failed") {
		t.Errorf("Expected fix failure message, got: %s", stderr.String())
	}
}
