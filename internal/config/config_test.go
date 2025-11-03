package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := defaultConfig()

    if cfg.Author != "AZ <Aezi.zhu@icloud.com>" {
        t.Errorf("expected author 'AZ <Aezi.zhu@icloud.com>', got %q", cfg.Author)
    }
	if cfg.Provider != "gemini" {
		t.Errorf("expected provider 'gemini', got %q", cfg.Provider)
	}
	if cfg.Model != "gemini-1.5-flash" {
		t.Errorf("expected model 'gemini-1.5-flash', got %q", cfg.Model)
	}
	if !cfg.DryRun {
		t.Error("expected DryRun to be true by default")
	}
	if cfg.AutoApprove {
		t.Error("expected AutoApprove to be false by default")
	}
	if cfg.TimeoutSeconds != 30 {
		t.Errorf("expected timeout 30, got %d", cfg.TimeoutSeconds)
	}
	if cfg.MaxCommands != 10 {
		t.Errorf("expected max commands 10, got %d", cfg.MaxCommands)
	}
	if len(cfg.Allowlist) == 0 {
		t.Error("expected non-empty allowlist")
	}
	if len(cfg.Denylist) == 0 {
		t.Error("expected non-empty denylist")
	}
}

func TestLoadWithEnvVars(t *testing.T) {
    os.Setenv("GEMINI_API_KEY", "test-key-123")
    os.Setenv("LUCICODEX_MODEL", "gemini-pro")
    os.Setenv("LUCICODEX_PROVIDER", "gemini")
    os.Setenv("LUCICODEX_LOG_FILE", "/tmp/test.log")
    os.Setenv("LUCICODEX_ELEVATE", "sudo")
	defer func() {
        os.Unsetenv("GEMINI_API_KEY")
        os.Unsetenv("LUCICODEX_MODEL")
        os.Unsetenv("LUCICODEX_PROVIDER")
        os.Unsetenv("LUCICODEX_LOG_FILE")
        os.Unsetenv("LUCICODEX_ELEVATE")
	}()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.APIKey != "test-key-123" {
		t.Errorf("expected API key 'test-key-123', got %q", cfg.APIKey)
	}
	if cfg.Model != "gemini-pro" {
		t.Errorf("expected model 'gemini-pro', got %q", cfg.Model)
	}
	if cfg.Provider != "gemini" {
		t.Errorf("expected provider 'gemini', got %q", cfg.Provider)
	}
	if cfg.LogFile != "/tmp/test.log" {
		t.Errorf("expected log file '/tmp/test.log', got %q", cfg.LogFile)
	}
	if cfg.ElevateCommand != "sudo" {
		t.Errorf("expected elevate command 'sudo', got %q", cfg.ElevateCommand)
	}
}

func TestLoadWithOpenAIEnvVars(t *testing.T) {
    os.Setenv("OPENAI_API_KEY", "openai-key-123")
    os.Setenv("LUCICODEX_PROVIDER", "openai")
	defer func() {
        os.Unsetenv("OPENAI_API_KEY")
        os.Unsetenv("LUCICODEX_PROVIDER")
	}()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.OpenAIAPIKey != "openai-key-123" {
		t.Errorf("expected OpenAI API key 'openai-key-123', got %q", cfg.OpenAIAPIKey)
	}
	if cfg.Provider != "openai" {
		t.Errorf("expected provider 'openai', got %q", cfg.Provider)
	}
}

func TestLoadWithAnthropicEnvVars(t *testing.T) {
    os.Setenv("ANTHROPIC_API_KEY", "anthropic-key-123")
    os.Setenv("LUCICODEX_PROVIDER", "anthropic")
	defer func() {
        os.Unsetenv("ANTHROPIC_API_KEY")
        os.Unsetenv("LUCICODEX_PROVIDER")
	}()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.AnthropicAPIKey != "anthropic-key-123" {
		t.Errorf("expected Anthropic API key 'anthropic-key-123', got %q", cfg.AnthropicAPIKey)
	}
	if cfg.Provider != "anthropic" {
		t.Errorf("expected provider 'anthropic', got %q", cfg.Provider)
	}
}

func TestLoadFromJSONFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	testConfig := Config{
		Author:         "test-author",
		APIKey:         "file-key-456",
		Model:          "test-model",
		Provider:       "gemini",
		DryRun:         false,
		AutoApprove:    true,
		TimeoutSeconds: 60,
		MaxCommands:    20,
		Allowlist:      []string{"^test"},
		Denylist:       []string{"^danger"},
		LogFile:        "/tmp/custom.log",
	}

	data, err := json.Marshal(testConfig)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Author != "test-author" {
		t.Errorf("expected author 'test-author', got %q", cfg.Author)
	}
	if cfg.APIKey != "file-key-456" {
		t.Errorf("expected API key 'file-key-456', got %q", cfg.APIKey)
	}
	if cfg.Model != "test-model" {
		t.Errorf("expected model 'test-model', got %q", cfg.Model)
	}
	if cfg.DryRun {
		t.Error("expected DryRun to be false")
	}
	if !cfg.AutoApprove {
		t.Error("expected AutoApprove to be true")
	}
	if cfg.TimeoutSeconds != 60 {
		t.Errorf("expected timeout 60, got %d", cfg.TimeoutSeconds)
	}
	if cfg.MaxCommands != 20 {
		t.Errorf("expected max commands 20, got %d", cfg.MaxCommands)
	}
}

func TestLoadEnvOverridesFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	testConfig := Config{
		APIKey:   "file-key",
		Model:    "file-model",
		Provider: "gemini",
	}

	data, err := json.Marshal(testConfig)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

    os.Setenv("GEMINI_API_KEY", "env-key")
    os.Setenv("LUCICODEX_MODEL", "env-model")
	defer func() {
        os.Unsetenv("GEMINI_API_KEY")
        os.Unsetenv("LUCICODEX_MODEL")
	}()

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.APIKey != "env-key" {
		t.Errorf("expected env to override file: got API key %q", cfg.APIKey)
	}
	if cfg.Model != "env-model" {
		t.Errorf("expected env to override file: got model %q", cfg.Model)
	}
}

func TestLoadMissingAPIKey(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Errorf("Load should not fail without API key (lazy validation): %v", err)
	}
	if cfg.APIKey != "" {
		t.Errorf("expected empty API key, got %q", cfg.APIKey)
	}
}

func TestLoadInvalidJSONFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	if err := os.WriteFile(configPath, []byte("invalid json {{{"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	os.Setenv("GEMINI_API_KEY", "test-key")
	defer os.Unsetenv("GEMINI_API_KEY")

	_, err := Load(configPath)
	if err == nil {
		t.Error("expected error when parsing invalid JSON")
	}
}

func TestFileExists(t *testing.T) {
	tmpDir := t.TempDir()
	existingFile := filepath.Join(tmpDir, "exists.txt")
	if err := os.WriteFile(existingFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if !fileExists(existingFile) {
		t.Error("expected fileExists to return true for existing file")
	}

	nonExistentFile := filepath.Join(tmpDir, "does-not-exist.txt")
	if fileExists(nonExistentFile) {
		t.Error("expected fileExists to return false for non-existent file")
	}

	if fileExists(tmpDir) {
		t.Error("expected fileExists to return false for directory")
	}
}

func TestLoadWithExternalGeminiPath(t *testing.T) {
    os.Setenv("GEMINI_API_KEY", "test-key")
    os.Setenv("LUCICODEX_EXTERNAL_GEMINI", "/custom/path/gemini")
	defer func() {
        os.Unsetenv("GEMINI_API_KEY")
        os.Unsetenv("LUCICODEX_EXTERNAL_GEMINI")
	}()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.ExternalGeminiPath != "/custom/path/gemini" {
		t.Errorf("expected external gemini path '/custom/path/gemini', got %q", cfg.ExternalGeminiPath)
	}
}

func TestLoadTrimsWhitespace(t *testing.T) {
    os.Setenv("GEMINI_API_KEY", "  test-key-with-spaces  ")
    os.Setenv("LUCICODEX_MODEL", "\tmodel-with-tabs\t")
	defer func() {
        os.Unsetenv("GEMINI_API_KEY")
        os.Unsetenv("LUCICODEX_MODEL")
	}()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.APIKey != "test-key-with-spaces" {
		t.Errorf("expected trimmed API key, got %q", cfg.APIKey)
	}
	if cfg.Model != "model-with-tabs" {
		t.Errorf("expected trimmed model, got %q", cfg.Model)
	}
}
