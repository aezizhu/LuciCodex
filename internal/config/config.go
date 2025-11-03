package config

import (
    "encoding/json"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "strconv"
    "strings"
)

type Config struct {
    Author         string   `json:"author"`
    APIKey         string   `json:"api_key"`
    Endpoint       string   `json:"endpoint"`
    Model          string   `json:"model"`
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
    MaxRetries     int      `json:"max_retries"`
    AutoRetry      bool     `json:"auto_retry"`
    // Optional external providers/API keys
    OpenAIAPIKey   string   `json:"openai_api_key"`
    AnthropicAPIKey string  `json:"anthropic_api_key"`
    ExternalGeminiPath string `json:"external_gemini_path"`
    GoogleOAuthClientID string `json:"google_oauth_client_id"`
    GoogleOAuthClientSecret string `json:"google_oauth_client_secret"`
}

func defaultConfig() Config {
    return Config{
        Author:         "AZ <Aezi.zhu@icloud.com>",
        Endpoint:       "https://generativelanguage.googleapis.com/v1beta",
        Model:          "gemini-1.5-flash",
        Provider:       "gemini",
        DryRun:         true,
        AutoApprove:    false,
        TimeoutSeconds: 30,
        MaxCommands:    10,
        MaxRetries:     2,
        AutoRetry:      true,
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
        ConfirmEach: false,
        LogFile: "/tmp/lucicodex.log",
        ElevateCommand: "",
        OpenAIAPIKey: "",
        AnthropicAPIKey: "",
        ExternalGeminiPath: "/usr/bin/gemini",
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
            return cfg, fmt.Errorf("read config: %w", err)
        }
        if err := json.Unmarshal(b, &cfg); err != nil {
            return cfg, fmt.Errorf("parse config: %w", err)
        }
    }

    // UCI (OpenWrt)
    if key, _ := uciGet("lucicodex.@api[0].key"); key != "" {
        cfg.APIKey = key
    }
    if m, _ := uciGet("lucicodex.@api[0].model"); m != "" {
        cfg.Model = m
    }
    if ep, _ := uciGet("lucicodex.@api[0].endpoint"); ep != "" {
        cfg.Endpoint = ep
    }
    if prov, _ := uciGet("lucicodex.@api[0].provider"); prov != "" {
        cfg.Provider = prov
    }
    if openaiKey, _ := uciGet("lucicodex.@api[0].openai_key"); openaiKey != "" {
        cfg.OpenAIAPIKey = openaiKey
    }
    if anthropicKey, _ := uciGet("lucicodex.@api[0].anthropic_key"); anthropicKey != "" {
        cfg.AnthropicAPIKey = anthropicKey
    }
    if dryRun, _ := uciGet("lucicodex.@settings[0].dry_run"); dryRun == "1" {
        cfg.DryRun = true
    } else if dryRun == "0" {
        cfg.DryRun = false
    }
    if confirmEach, _ := uciGet("lucicodex.@settings[0].confirm_each"); confirmEach == "1" {
        cfg.ConfirmEach = true
    } else if confirmEach == "0" {
        cfg.ConfirmEach = false
    }
    if timeout, _ := uciGet("lucicodex.@settings[0].timeout"); timeout != "" {
        if t, err := strconv.Atoi(timeout); err == nil && t > 0 {
            cfg.TimeoutSeconds = t
        }
    }
    if maxCmds, _ := uciGet("lucicodex.@settings[0].max_commands"); maxCmds != "" {
        if m, err := strconv.Atoi(maxCmds); err == nil && m > 0 {
            cfg.MaxCommands = m
        }
    }
    if logFile, _ := uciGet("lucicodex.@settings[0].log_file"); logFile != "" {
        cfg.LogFile = logFile
    }

    if v := strings.TrimSpace(os.Getenv("GEMINI_API_KEY")); v != "" {
        cfg.APIKey = v
    }
    if v := strings.TrimSpace(os.Getenv("GEMINI_ENDPOINT")); v != "" {
        cfg.Endpoint = v
    }
    if v := strings.TrimSpace(os.Getenv("LUCICODEX_MODEL")); v != "" {
        cfg.Model = v
    }
    if v := strings.TrimSpace(os.Getenv("LUCICODEX_LOG_FILE")); v != "" {
        cfg.LogFile = v
    }
    if v := strings.TrimSpace(os.Getenv("LUCICODEX_ELEVATE")); v != "" {
        cfg.ElevateCommand = v
    }
    if v := strings.TrimSpace(os.Getenv("LUCICODEX_PROVIDER")); v != "" {
        cfg.Provider = v
    }
    if v := strings.TrimSpace(os.Getenv("OPENAI_API_KEY")); v != "" {
        cfg.OpenAIAPIKey = v
    }
    if v := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY")); v != "" {
        cfg.AnthropicAPIKey = v
    }
    if v := strings.TrimSpace(os.Getenv("LUCICODEX_EXTERNAL_GEMINI")); v != "" {
        cfg.ExternalGeminiPath = v
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

    return cfg, nil
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


