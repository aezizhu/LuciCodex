package config

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Author         string   `json:"author"`
	APIKey         string   `json:"api_key"`  // Gemini API key
	Endpoint       string   `json:"endpoint"` // Active endpoint (set based on provider)
	Model          string   `json:"model"`    // Active model (set based on provider)
	Provider       string   `json:"provider"`
	DryRun         bool     `json:"dry_run"`
	AutoApprove    bool     `json:"auto_approve"`
	ConfirmEach    bool     `json:"confirm_each"`
	TimeoutSeconds int      `json:"timeout_seconds"`
	MaxCommands    int      `json:"max_commands"`
	Allowlist      []string `json:"allowlist"`
	Denylist       []string `json:"denylist"`
	LogFile        string   `json:"log_file"`
	ElevateCommand string   `json:"elevate_command"`
	// Retry configuration
	MaxRetries int  `json:"max_retries"`
	AutoRetry  bool `json:"auto_retry"`
	// Provider-specific API keys
	OpenAIAPIKey    string `json:"openai_api_key"`
	AnthropicAPIKey string `json:"anthropic_api_key"`
	// Provider-specific endpoints (stored separately for switching)
	OpenAIEndpoint    string `json:"openai_endpoint"`
	AnthropicEndpoint string `json:"anthropic_endpoint"`
	// Provider-specific models (stored separately for switching)
	OpenAIModel    string `json:"openai_model"`
	AnthropicModel string `json:"anthropic_model"`
}

func defaultConfig() Config {
	return Config{
		Author:            "AZ <Aezi.zhu@icloud.com>",
		Endpoint:          "https://generativelanguage.googleapis.com/v1beta",
		Model:             "gemini-2.5-flash",
		Provider:          "gemini",
		DryRun:            true,
		AutoApprove:       false,
		TimeoutSeconds:    30,
		MaxCommands:       10,
		MaxRetries:        2,
		AutoRetry:         true,
		OpenAIEndpoint:    "https://api.openai.com/v1",
		OpenAIModel:       "gpt-5-mini",
		AnthropicEndpoint: "https://api.anthropic.com/v1",
		AnthropicModel:    "claude-haiku-4-5-20251001",
		Allowlist: []string{
			`^uci(\s|$)`,
			`^ubus(\s|$)`,
			`^fw4(\s|$)`,
			`^opkg\s+(?:update|install|remove|list(?:-installed|-upgradable)?|info)(?:\s|$)`,
			`^logread(\s|$)`,
			`^dmesg(\s|$)`,
			`^ip(\s|$)`,
			`^ifstatus(\s|$)`,
			`^cat(\s|$)`,
			`^tail(\s|$)`,
			`^grep(\s|$)`,
			`^awk(\s|$)`,
			`^sed(\s|$)`,
			`^wifi(\s|$)`,
			`^ping(\s|$)`,
			`^nslookup(\s|$)`,
			`^ifconfig(\s|$)`,
			`^route(\s|$)`,
			`^iptables(\s|$)`,
			`^/etc/init\.d/`,
		},
		Denylist: []string{
			`^rm\s+-rf\s+/`,
			`^mkfs(\s|$)`,
			`^dd(\s|$)`,
			`^:(){:|:&};:`,
		},
		ConfirmEach:    false,
		LogFile:        "/tmp/lucicodex.log",
		ElevateCommand: "",
	}
}

// Load loads configuration from env, UCI (if available), and optional JSON file.
// Precedence: env > UCI > file > defaults
func Load(path string) (Config, error) {
	cfg := defaultConfig()

	// File
	if path == "" {
		if fileExists("/etc/lucicodex/config.json") {
			path = "/etc/lucicodex/config.json"
		} else {
			home, _ := os.UserHomeDir()
			p := filepath.Join(home, ".config", "lucicodex", "config.json")
			if fileExists(p) {
				path = p
			}
		}
	}
	if path != "" && fileExists(path) {
		b, err := os.ReadFile(path)
		if err != nil {
			return cfg, err
		}
		if err := json.Unmarshal(b, &cfg); err != nil {
			return cfg, err
		}
	}

	// Helper to try main section, then settings section, then api section
	getUci := func(option string) string {
		// Try named 'main' section first
		if val, _ := uciGet("lucicodex.main." + option); val != "" {
			return val
		}
		// Try anonymous settings section
		if val, _ := uciGet("lucicodex.@settings[0]." + option); val != "" {
			return val
		}
		// Try anonymous api section (legacy)
		if val, _ := uciGet("lucicodex.@api[0]." + option); val != "" {
			return val
		}
		return ""
	}

	// Load provider first (needed to determine which settings to use)
	if prov := getUci("provider"); prov != "" {
		cfg.Provider = prov
	}

	// Load all API keys from UCI
	if key := getUci("key"); key != "" {
		cfg.APIKey = key
	}
	if key := getUci("openai_key"); key != "" {
		cfg.OpenAIAPIKey = key
	}
	if key := getUci("anthropic_key"); key != "" {
		cfg.AnthropicAPIKey = key
	}

	// Load all provider-specific models and endpoints from UCI
	if m := getUci("model"); m != "" {
		cfg.Model = m
	}
	if ep := getUci("endpoint"); ep != "" {
		cfg.Endpoint = ep
	}
	if m := getUci("openai_model"); m != "" {
		cfg.OpenAIModel = m
	}
	if ep := getUci("openai_endpoint"); ep != "" {
		cfg.OpenAIEndpoint = ep
	}
	if m := getUci("anthropic_model"); m != "" {
		cfg.AnthropicModel = m
	}
	if ep := getUci("anthropic_endpoint"); ep != "" {
		cfg.AnthropicEndpoint = ep
	}

	// Load settings from UCI
	if dryRun := getUci("dry_run"); dryRun == "1" {
		cfg.DryRun = true
	} else if dryRun == "0" {
		cfg.DryRun = false
	}
	if confirmEach := getUci("confirm_each"); confirmEach == "1" {
		cfg.ConfirmEach = true
	} else if confirmEach == "0" {
		cfg.ConfirmEach = false
	}
	if timeout := getUci("timeout"); timeout != "" {
		if t, err := strconv.Atoi(timeout); err == nil && t > 0 {
			cfg.TimeoutSeconds = t
		}
	}
	if maxCmds := getUci("max_commands"); maxCmds != "" {
		if m, err := strconv.Atoi(maxCmds); err == nil && m > 0 {
			cfg.MaxCommands = m
		}
	}
	if logFile := getUci("log_file"); logFile != "" {
		cfg.LogFile = logFile
	}

	// Environment variables override everything
	if v := strings.TrimSpace(os.Getenv("LUCICODEX_PROVIDER")); v != "" {
		cfg.Provider = v
	}
	if v := strings.TrimSpace(os.Getenv("GEMINI_API_KEY")); v != "" {
		cfg.APIKey = v
	}
	if v := strings.TrimSpace(os.Getenv("OPENAI_API_KEY")); v != "" {
		cfg.OpenAIAPIKey = v
	}
	if v := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY")); v != "" {
		cfg.AnthropicAPIKey = v
	}
	if v := strings.TrimSpace(os.Getenv("LUCICODEX_MODEL")); v != "" {
		cfg.Model = v
	}
	if v := strings.TrimSpace(os.Getenv("GEMINI_ENDPOINT")); v != "" {
		cfg.Endpoint = v
	}
	if v := strings.TrimSpace(os.Getenv("LUCICODEX_LOG_FILE")); v != "" {
		cfg.LogFile = v
	}
	if v := strings.TrimSpace(os.Getenv("LUCICODEX_ELEVATE")); v != "" {
		cfg.ElevateCommand = v
	}
	if v := strings.TrimSpace(os.Getenv("LUCICODEX_CONFIRM_EACH")); v != "" {
		cfg.ConfirmEach = v == "1" || strings.ToLower(v) == "true"
	}
	if v := strings.TrimSpace(os.Getenv("LUCICODEX_AUTO_RETRY")); v != "" {
		cfg.AutoRetry = v == "1" || strings.ToLower(v) == "true"
	}
	if v := strings.TrimSpace(os.Getenv("LUCICODEX_MAX_RETRIES")); v != "" {
		if r, err := strconv.Atoi(v); err == nil && r >= 0 {
			cfg.MaxRetries = r
		}
	}

	// Set active Model and Endpoint based on provider
	cfg.ApplyProviderSettings()

	return cfg, nil
}

// ApplyProviderSettings sets the active Model and Endpoint based on the selected provider.
// This should be called after any provider changes (e.g., CLI flag overrides).
func (cfg *Config) ApplyProviderSettings() {
	switch cfg.Provider {
	case "openai":
		if cfg.OpenAIModel != "" {
			cfg.Model = cfg.OpenAIModel
		} else if cfg.Model == "" || cfg.Model == "gemini-2.5-flash" {
			cfg.Model = "gpt-5-mini"
		}
		if cfg.OpenAIEndpoint != "" {
			cfg.Endpoint = cfg.OpenAIEndpoint
		} else {
			cfg.Endpoint = "https://api.openai.com/v1"
		}
	case "anthropic":
		if cfg.AnthropicModel != "" {
			cfg.Model = cfg.AnthropicModel
		} else if cfg.Model == "" || cfg.Model == "gemini-2.5-flash" {
			cfg.Model = "claude-haiku-4-5-20251001"
		}
		if cfg.AnthropicEndpoint != "" {
			cfg.Endpoint = cfg.AnthropicEndpoint
		} else {
			cfg.Endpoint = "https://api.anthropic.com/v1"
		}
	default: // gemini
		if cfg.Model == "" {
			cfg.Model = "gemini-2.5-flash"
		}
		if cfg.Endpoint == "" {
			cfg.Endpoint = "https://generativelanguage.googleapis.com/v1beta"
		}
	}
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func uciGet(key string) (string, error) {
	_, err := exec.LookPath("uci")
	if err != nil {
		return "", err
	}
	out, err := exec.Command("uci", "-q", "get", key).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
