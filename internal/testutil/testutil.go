package testutil

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aezizhu/LuciCodex/internal/config"
)

// AssertNoError fails the test if err is not nil
func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

// AssertError fails the test if err is nil
func AssertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// AssertEqual fails the test if got != want
func AssertEqual(t *testing.T, got, want interface{}) {
	t.Helper()
	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// AssertContains fails the test if haystack doesn't contain needle
func AssertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("expected %q to contain %q", haystack, needle)
	}
}

// AssertNotContains fails the test if haystack contains needle
func AssertNotContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Fatalf("expected %q to not contain %q", haystack, needle)
	}
}

// AssertTrue fails the test if condition is false
func AssertTrue(t *testing.T, condition bool) {
	t.Helper()
	if !condition {
		t.Fatal("expected true, got false")
	}
}

// AssertFalse fails the test if condition is true
func AssertFalse(t *testing.T, condition bool) {
	t.Helper()
	if condition {
		t.Fatal("expected false, got true")
	}
}

// TempConfig creates a temporary config file with the given config and returns its path
func TempConfig(t *testing.T, cfg config.Config) string {
	t.Helper()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	data, err := json.Marshal(cfg)
	AssertNoError(t, err)

	err = os.WriteFile(configPath, data, 0644)
	AssertNoError(t, err)

	return configPath
}

// MockHTTPServer creates a mock HTTP server that returns the given response
// The caller is responsible for closing the server
func MockHTTPServer(t *testing.T, statusCode int, responseBody string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(responseBody))
	}))
}

// MockHTTPServerJSON creates a mock HTTP server that returns JSON response
func MockHTTPServerJSON(t *testing.T, statusCode int, responseData interface{}) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		err := json.NewEncoder(w).Encode(responseData)
		if err != nil {
			t.Logf("failed to encode response: %v", err)
		}
	}))
}

// MockHTTPServerFunc creates a mock HTTP server with a custom handler function
func MockHTTPServerFunc(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(handler))
}

// ReadBody reads the entire request body and returns it as a string
func ReadBody(t *testing.T, r io.Reader) string {
	t.Helper()
	data, err := io.ReadAll(r)
	AssertNoError(t, err)
	return string(data)
}

// DefaultTestConfig returns a default config for testing
func DefaultTestConfig() config.Config {
	return config.Config{
		Author:         "test-author",
		APIKey:         "test-api-key",
		Model:          "test-model",
		Provider:       "gemini",
		Endpoint:       "http://localhost:8080",
		DryRun:         false,
		AutoApprove:    true,
		TimeoutSeconds: 30,
		MaxCommands:    10,
		MaxRetries:     2,
		AutoRetry:      true,
		Allowlist:      []string{"^uci", "^echo"},
		Denylist:       []string{"^rm -rf"},
		LogFile:        "/tmp/test.log",
	}
}

// TempFile creates a temporary file with the given content
func TempFile(t *testing.T, content string) string {
	t.Helper()

	tmpFile, err := os.CreateTemp(t.TempDir(), "test-*")
	AssertNoError(t, err)
	defer tmpFile.Close()

	_, err = tmpFile.WriteString(content)
	AssertNoError(t, err)

	return tmpFile.Name()
}

// TempDir creates a temporary directory
func TempDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// FileExists checks if a file exists
func FileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil && !info.IsDir()
}

// DirExists checks if a directory exists
func DirExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil && info.IsDir()
}

// StripAnsi removes ANSI color codes from a string
func StripAnsi(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(s, "\033[0m", ""), "\033[31m", ""), "\033[32m", ""), "\033[33m", ""), "\033[34m", ""), "\033[1m", "")
}
