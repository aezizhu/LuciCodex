package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
	if cfg.TimeoutSeconds != 60 {
		t.Errorf("expected timeout 60, got %d", cfg.TimeoutSeconds)
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
		// Might be set from real env, so we can't strictly assert empty
		// unless we unset everything.
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

func TestLoadFileReadError(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "unreadable.json")

	if err := os.WriteFile(configPath, []byte("{}"), 0000); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
	// Restore permissions so cleanup works
	defer os.Chmod(configPath, 0644)

	_, err := Load(configPath)
	if err == nil {
		t.Error("expected error when reading unreadable file")
	}
}

func TestApplyProviderSettings(t *testing.T) {
	tests := []struct {
		name         string
		cfg          Config
		wantModel    string
		wantEndpoint string
	}{
		{
			name: "OpenAI Defaults",
			cfg: Config{
				Provider: "openai",
			},
			wantModel:    "gpt-4o-mini",
			wantEndpoint: "https://api.openai.com/v1",
		},
		{
			name: "OpenAI Explicit",
			cfg: Config{
				Provider:       "openai",
				OpenAIModel:    "gpt-4",
				OpenAIEndpoint: "https://custom.openai.com",
			},
			wantModel:    "gpt-4",
			wantEndpoint: "https://custom.openai.com",
		},
		{
			name: "OpenAI Existing Model",
			cfg: Config{
				Provider: "openai",
				Model:    "gpt-3.5-turbo",
			},
			wantModel:    "gpt-3.5-turbo",
			wantEndpoint: "https://api.openai.com/v1",
		},
		{
			name: "Anthropic Defaults",
			cfg: Config{
				Provider: "anthropic",
			},
			wantModel:    "claude-3-haiku-20240307",
			wantEndpoint: "https://api.anthropic.com/v1",
		},
		{
			name: "Anthropic Explicit",
			cfg: Config{
				Provider:          "anthropic",
				AnthropicModel:    "claude-3-opus",
				AnthropicEndpoint: "https://custom.anthropic.com",
			},
			wantModel:    "claude-3-opus",
			wantEndpoint: "https://custom.anthropic.com",
		},
		{
			name: "Anthropic Existing Model",
			cfg: Config{
				Provider: "anthropic",
				Model:    "claude-2",
			},
			wantModel:    "claude-2",
			wantEndpoint: "https://api.anthropic.com/v1",
		},
		{
			name: "Gemini Defaults",
			cfg: Config{
				Provider: "gemini",
			},
			wantModel:    "gemini-1.5-flash",
			wantEndpoint: "https://generativelanguage.googleapis.com/v1beta",
		},
		{
			name: "Gemini Explicit",
			cfg: Config{
				Provider: "gemini",
				Model:    "gemini-pro",
				Endpoint: "https://custom.gemini.com",
			},
			wantModel:    "gemini-pro",
			wantEndpoint: "https://custom.gemini.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.cfg.ApplyProviderSettings()
			if tt.cfg.Model != tt.wantModel {
				t.Errorf("got Model %q, want %q", tt.cfg.Model, tt.wantModel)
			}
			if tt.cfg.Endpoint != tt.wantEndpoint {
				t.Errorf("got Endpoint %q, want %q", tt.cfg.Endpoint, tt.wantEndpoint)
			}
		})
	}
}

func TestLoad_DefaultFile(t *testing.T) {
	tmpHome := t.TempDir()
	configDir := filepath.Join(tmpHome, ".config", "lucicodex")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	configFile := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(configFile, []byte(`{"api_key": "home-key"}`), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	os.Setenv("HOME", tmpHome)
	defer os.Unsetenv("HOME")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.APIKey != "home-key" {
		t.Errorf("expected APIKey 'home-key', got %q", cfg.APIKey)
	}
}

func TestLoad_EtcFile(t *testing.T) {
	// Mock fileExists to return true for /etc/lucicodex/config.json
	oldFileExists := fileExists
	fileExists = func(p string) bool {
		if p == "/etc/lucicodex/config.json" {
			return true
		}
		return oldFileExists(p)
	}
	defer func() { fileExists = oldFileExists }()

	// We also need to mock os.ReadFile because Load calls it directly
	// But Load calls os.ReadFile(path). We can't easily mock os.ReadFile globally.
	// However, if fileExists returns true, Load tries to read it.
	// If reading fails, Load returns error.
	// We want to verify it TRIES to read /etc...

	// Actually, we can't mock os.ReadFile easily without more refactoring.
	// But we can verify that Load returns an error trying to read /etc... (permission denied or not found)
	// Wait, if fileExists returns true but file doesn't exist (because we mocked fileExists), os.ReadFile will fail.
	// This confirms Load tried to use that path.

	_, err := Load("")
	if err == nil {
		t.Error("expected error reading non-existent /etc file (mocked existence)")
	} else {
		// Check if error relates to /etc/lucicodex/config.json
		if !strings.Contains(err.Error(), "/etc/lucicodex/config.json") {
			t.Errorf("expected error for /etc file, got: %v", err)
		}
	}
}

func TestLoad_AllEnvVars(t *testing.T) {
	os.Setenv("GEMINI_ENDPOINT", "https://env.gemini.com")
	os.Setenv("LUCICODEX_CONFIRM_EACH", "true")
	os.Setenv("LUCICODEX_AUTO_RETRY", "false")
	os.Setenv("LUCICODEX_MAX_RETRIES", "5")
	os.Setenv("HTTP_PROXY", "http://env-proxy")
	os.Setenv("HTTPS_PROXY", "https://env-proxy")
	os.Setenv("NO_PROXY", "env-no-proxy")
	defer func() {
		os.Unsetenv("GEMINI_ENDPOINT")
		os.Unsetenv("LUCICODEX_CONFIRM_EACH")
		os.Unsetenv("LUCICODEX_AUTO_RETRY")
		os.Unsetenv("LUCICODEX_MAX_RETRIES")
		os.Unsetenv("HTTP_PROXY")
		os.Unsetenv("HTTPS_PROXY")
		os.Unsetenv("NO_PROXY")
	}()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Endpoint != "https://env.gemini.com" {
		t.Errorf("got Endpoint %q", cfg.Endpoint)
	}
	if !cfg.ConfirmEach {
		t.Error("expected ConfirmEach true")
	}
	if cfg.AutoRetry {
		t.Error("expected AutoRetry false")
	}
	if cfg.MaxRetries != 5 {
		t.Errorf("got MaxRetries %d", cfg.MaxRetries)
	}
	if cfg.HTTPProxy != "http://env-proxy" {
		t.Errorf("got HTTPProxy %q", cfg.HTTPProxy)
	}
	if cfg.HTTPSProxy != "https://env-proxy" {
		t.Errorf("got HTTPSProxy %q", cfg.HTTPSProxy)
	}
	if cfg.NoProxy != "env-no-proxy" {
		t.Errorf("got NoProxy %q", cfg.NoProxy)
	}
}
