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
