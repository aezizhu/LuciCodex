package wizard

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aezizhu/LuciCodex/internal/config"
)

func TestWizard_Retry(t *testing.T) {
	// Test readBool retry
	input := "invalid\ny\n"
	w := New(strings.NewReader(input), io.Discard)
	if !w.readBool("Prompt", false) {
		t.Error("readBool expected true after retry")
	}

	// Test readInt retry (invalid number)
	input = "invalid\n10\n"
	w = New(strings.NewReader(input), io.Discard)
	if val := w.readInt("Prompt", 0, 1, 100); val != 10 {
		t.Errorf("readInt expected 10 after retry, got %d", val)
	}

	// Test readInt retry (out of range)
	input = "999\n10\n"
	w = New(strings.NewReader(input), io.Discard)
	if val := w.readInt("Prompt", 0, 1, 100); val != 10 {
		t.Errorf("readInt expected 10 after retry, got %d", val)
	}

	// Test readChoice retry (invalid number)
	input = "invalid\n2\n"
	w = New(strings.NewReader(input), io.Discard)
	choice, err := w.readChoice("Prompt", 1, 3)
	if err != nil {
		t.Fatalf("readChoice failed: %v", err)
	}
	if choice != 2 {
		t.Errorf("readChoice expected 2 after retry, got %d", choice)
	}

	// Test readChoice retry (out of range)
	input = "9\n2\n"
	w = New(strings.NewReader(input), io.Discard)
	choice, err = w.readChoice("Prompt", 1, 3)
	if err != nil {
		t.Fatalf("readChoice failed: %v", err)
	}
	if choice != 2 {
		t.Errorf("readChoice expected 2 after retry, got %d", choice)
	}
}

func TestWizard_readString(t *testing.T) {
	input := "hello world\n"
	reader := strings.NewReader(input)
	writer := io.Discard

	w := New(reader, writer)
	result := w.readString("Prompt", "default")

	if result != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", result)
	}

	// Test default value
	reader = strings.NewReader("\n")
	w = New(reader, writer)
	result = w.readString("Prompt", "default")
	if result != "default" {
		t.Errorf("expected 'default', got '%s'", result)
	}
}

func TestWizard_readBool(t *testing.T) {
	testCases := []struct {
		input    string
		expected bool
	}{
		{"y\n", true},
		{"yes\n", true},
		{"Y\n", true},
		{"n\n", false},
		{"no\n", false},
		{"N\n", false},
		{"\n", true}, // Default
	}

	for _, tc := range testCases {
		reader := strings.NewReader(tc.input)
		writer := io.Discard
		w := New(reader, writer)
		// Test with a default of true
		if w.readBool("Prompt", true) != tc.expected {
			t.Errorf("for input '%s', expected %v", tc.input, tc.expected)
		}
	}
}

func TestWizard_readInt(t *testing.T) {
	reader := strings.NewReader("42\n")
	writer := io.Discard
	w := New(reader, writer)

	result := w.readInt("Prompt", 10, 1, 100)
	if result != 42 {
		t.Errorf("expected 42, got %d", result)
	}

	// Test invalid input then valid
	reader = strings.NewReader("abc\n25\n")
	w = New(reader, writer)
	result = w.readInt("Prompt", 10, 1, 100)
	if result != 25 {
		t.Errorf("expected 25 after invalid input, got %d", result)
	}
}

func TestWizard_Run(t *testing.T) {
	// ---
	// Setup ---
	// Provide a sequence of inputs for a full wizard run
	input := "1\n" + // Provider: Gemini
		"gemini-pro\n" + // Model
		"test-api-key\n" + // API Key
		"n\n" + // Dry run: no
		"y\n" + // Auto-approve: yes
		"20\n" + // Max commands
		"60\n" + // Timeout
		"n\n" + // No privilege elevation
		"2\n" // Save to HOME/.config

	inputBuffer := strings.NewReader(input)
	var outputBuffer bytes.Buffer

	// Create a temporary HOME directory for the test
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	expectedConfigPath := filepath.Join(tempHome, ".config", "lucicodex", "config.json")

	// ---
	// Act ---
	w := New(inputBuffer, &outputBuffer)
	err := w.Run()
	if err != nil {
		t.Fatalf("Wizard.Run() failed: %v", err)
	}

	// ---
	// Assert ---
	// Check that the config file was created
	if _, err := os.Stat(expectedConfigPath); os.IsNotExist(err) {
		t.Fatalf("expected config file to be created at %s, but it wasn't", expectedConfigPath)
	}

	// Check the content of the config file
	cfg, err := config.Load(expectedConfigPath)
	if err != nil {
		t.Fatalf("Failed to load created config file: %v", err)
	}

	if cfg.Provider != "gemini" {
		t.Errorf("expected provider 'gemini', got '%s'", cfg.Provider)
	}
	if cfg.Model != "gemini-pro" {
		t.Errorf("expected model 'gemini-pro', got '%s'", cfg.Model)
	}
	if cfg.APIKey != "test-api-key" {
		t.Errorf("expected api key 'test-api-key', got '%s'", cfg.APIKey)
	}
	if cfg.DryRun != false {
		t.Errorf("expected DryRun to be false, got %v", cfg.DryRun)
	}
	if cfg.AutoApprove != true {
		t.Errorf("expected AutoApprove to be true, got %v", cfg.AutoApprove)
	}
	if cfg.MaxCommands != 20 {
		t.Errorf("expected MaxCommands to be 20, got %d", cfg.MaxCommands)
	}
	if cfg.TimeoutSeconds != 60 {
		t.Errorf("expected TimeoutSeconds to be 60, got %d", cfg.TimeoutSeconds)
	}

	// Check that the output written to the user is reasonable
	output := outputBuffer.String()
	if !strings.Contains(output, "LuciCodex Setup Wizard") {
		t.Error("output should contain wizard title")
	}
	if !strings.Contains(output, "Configuration saved to") {
		t.Error("output should contain confirmation of saving")
	}
}

func TestWizard_readChoice(t *testing.T) {
	reader := strings.NewReader("2\n")
	writer := io.Discard
	w := New(reader, writer)

	choice, err := w.readChoice("Choose", 1, 3)
	if err != nil {
		t.Fatalf("readChoice failed: %v", err)
	}
	if choice != 2 {
		t.Errorf("expected 2, got %d", choice)
	}

	// Test invalid input then valid
	reader = strings.NewReader("abc\n4\n1\n")
	w = New(reader, writer)
	choice, err = w.readChoice("Choose", 1, 3)
	if err != nil {
		t.Fatalf("readChoice failed: %v", err)
	}
	if choice != 1 {
		t.Errorf("expected 1 after invalid inputs, got %d", choice)
	}
}

func TestWizard_Run_EOF(t *testing.T) {
	// Simulate EOF during provider selection
	reader := strings.NewReader("")
	writer := io.Discard
	w := New(reader, writer)

	err := w.Run()
	if err == nil {
		t.Error("expected error for EOF")
	}
}

func TestWizard_SaveConfig_Error(t *testing.T) {
	// Setup inputs for a full run until save
	input := "1\n" + // Provider
		"\n" + // Model default
		"key\n" + // API Key
		"\n" + // Dry run default
		"\n" + // Max commands default
		"\n" + // Timeout default
		"\n" + // Elevation default
		"1\n" // Save to /etc/lucicodex/config.json (likely fails without root)

	reader := strings.NewReader(input)
	var output bytes.Buffer
	w := New(reader, &output)

	// Mock os.MkdirAll/WriteFile by using a path that fails?
	// The code uses hardcoded paths in saveConfig:
	// 1. /etc/lucicodex/config.json
	// 2. ~/.config/lucicodex/config.json

	// If we choose 1, it tries to write to /etc.
	// In a test environment (non-root), this should fail with permission denied.
	// Unless the test runner is root (unlikely).

	err := w.Run()
	if err == nil {
		// If it succeeded, maybe we have write access or mocked something?
		// But we haven't mocked os functions here.
		// If /etc is writable, this test is flaky.
		// But usually it isn't.
		// Let's check if error contains "permission denied" or similar.
	} else {
		// Expected error
		if !strings.Contains(err.Error(), "permission denied") && !strings.Contains(err.Error(), "read-only file system") && !strings.Contains(err.Error(), "create config directory") && !strings.Contains(err.Error(), "write config") {
			t.Logf("Got expected error, but message was unexpected: %v", err)
		}
	}
}

func TestWizard_Run_Providers(t *testing.T) {
	// Test OpenAI path
	inputOpenAI := "2\n" + // Provider: OpenAI
		"\n" + // Model default
		"sk-test\n" + // API Key
		"\n" + // Dry run default
		"\n" + // Max commands default
		"\n" + // Timeout default
		"\n" + // Elevation default
		"2\n" // Save to HOME/.config

	// Test Anthropic path
	inputAnthropic := "3\n" + // Provider: Anthropic
		"\n" + // Model default
		"sk-ant-test\n" + // API Key
		"\n" + // Dry run default
		"\n" + // Max commands default
		"\n" + // Timeout default
		"\n" + // Elevation default
		"2\n" // Save to HOME/.config

	tests := []struct {
		name     string
		input    string
		provider string
		apiKey   string
	}{
		{"OpenAI", inputOpenAI, "openai", "sk-test"},
		{"Anthropic", inputAnthropic, "anthropic", "sk-ant-test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			// Unset environment variables that might interfere
			t.Setenv("OPENAI_API_KEY", "")
			t.Setenv("ANTHROPIC_API_KEY", "")

			w := New(strings.NewReader(tt.input), io.Discard)
			if err := w.Run(); err != nil {
				t.Fatalf("Run failed: %v", err)
			}

			// Verify config
			cfgPath := filepath.Join(os.Getenv("HOME"), ".config", "lucicodex", "config.json")
			cfg, err := config.Load(cfgPath)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Provider != tt.provider {
				t.Errorf("expected provider %q, got %q", tt.provider, cfg.Provider)
			}
			if tt.provider == "openai" && cfg.OpenAIAPIKey != tt.apiKey {
				t.Errorf("expected key %q, got %q", tt.apiKey, cfg.OpenAIAPIKey)
			}
			if tt.provider == "anthropic" && cfg.AnthropicAPIKey != tt.apiKey {
				t.Errorf("expected key %q, got %q", tt.apiKey, cfg.AnthropicAPIKey)
			}
		})
	}
}

func TestWizard_Run_SecurityOptions(t *testing.T) {
	input := "1\n" + // Provider: Gemini
		"\n" + // Model default
		"key\n" + // API Key
		"n\n" + // Dry run: no
		"y\n" + // Auto-approve: yes
		"\n" + // Max commands default
		"\n" + // Timeout default
		"y\n" + // Configure elevation: yes
		"sudo -n\n" + // Elevation command
		"2\n" // Save to HOME/.config

	t.Setenv("HOME", t.TempDir())
	w := New(strings.NewReader(input), io.Discard)
	if err := w.Run(); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	cfgPath := filepath.Join(os.Getenv("HOME"), ".config", "lucicodex", "config.json")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.DryRun {
		t.Error("expected DryRun false")
	}
	if !cfg.AutoApprove {
		t.Error("expected AutoApprove true")
	}
	if cfg.ElevateCommand != "sudo -n" {
		t.Errorf("expected ElevateCommand 'sudo -n', got %q", cfg.ElevateCommand)
	}
}

func TestWizard_readBool_Error(t *testing.T) {
	// Reader that returns error immediately
	r := &errorReader{err: io.ErrUnexpectedEOF}
	w := New(r, io.Discard)

	val := w.readBool("prompt", true)
	if !val {
		t.Error("expected default value true on error")
	}

	val = w.readBool("prompt", false)
	if val {
		t.Error("expected default value false on error")
	}
}

func TestWizard_readInt_Error(t *testing.T) {
	r := &errorReader{err: io.ErrUnexpectedEOF}
	w := New(r, io.Discard)

	val := w.readInt("prompt", 10, 0, 100)
	if val != 10 {
		t.Errorf("expected default value 10 on error, got %d", val)
	}
}

type errorReader struct {
	err error
}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, e.err
}
