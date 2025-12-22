package testutil

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"

	"github.com/aezizhu/LuciCodex/internal/config"
)

// TestingT is an interface wrapper around *testing.T
type TestingT interface {
	Helper()
	Fatalf(format string, args ...interface{})
	Fatal(args ...interface{})
	Errorf(format string, args ...interface{})
	Error(args ...interface{})
	TempDir() string
	Logf(format string, args ...interface{})
}

// AssertNoError fails the test if err is not nil
func AssertNoError(t TestingT, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

// AssertError fails the test if err is nil
func AssertError(t TestingT, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// AssertEqual fails the test if got != want
func AssertEqual(t TestingT, got, want interface{}) {
	t.Helper()
	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// AssertContains fails the test if haystack doesn't contain needle
func AssertContains(t TestingT, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("expected %q to contain %q", haystack, needle)
	}
}

// AssertNotContains fails the test if haystack contains needle
func AssertNotContains(t TestingT, haystack, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Fatalf("expected %q to not contain %q", haystack, needle)
	}
}

// AssertTrue fails the test if condition is false
func AssertTrue(t TestingT, condition bool) {
	t.Helper()
	if !condition {
		t.Fatal("expected true, got false")
	}
}

// AssertFalse fails the test if condition is true
func AssertFalse(t TestingT, condition bool) {
	t.Helper()
	if condition {
		t.Fatal("expected false, got true")
	}
}

// TempConfig creates a temporary config file with the given config and returns its path
func TempConfig(t TestingT, cfg config.Config) string {
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
func MockHTTPServer(t TestingT, statusCode int, responseBody string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(responseBody))
	}))
}

// MockHTTPServerJSON creates a mock HTTP server that returns JSON response
func MockHTTPServerJSON(t TestingT, statusCode int, responseData interface{}) *httptest.Server {
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
func MockHTTPServerFunc(t TestingT, handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(handler))
}

// ReadBody reads the entire request body and returns it as a string
func ReadBody(t TestingT, r io.Reader) string {
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
func TempFile(t TestingT, content string) string {
	t.Helper()

	tmpFile, err := os.CreateTemp(t.TempDir(), "test-*")
	AssertNoError(t, err)
	defer tmpFile.Close()

	_, err = tmpFile.WriteString(content)
	AssertNoError(t, err)

	return tmpFile.Name()
}

// TempDir creates a temporary directory
func TempDir(t TestingT) string {
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

// ansiReplacer is pre-compiled for efficient ANSI code stripping
var ansiReplacer = strings.NewReplacer(
	"\033[0m", "",
	"\033[31m", "",
	"\033[32m", "",
	"\033[33m", "",
	"\033[34m", "",
	"\033[1m", "",
)

// StripAnsi removes ANSI color codes from a string
func StripAnsi(s string) string {
	return ansiReplacer.Replace(s)
}
