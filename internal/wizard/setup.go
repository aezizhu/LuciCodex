package wizard

import (
    "bufio"
    "encoding/json"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "strconv"
    "strings"

    "github.com/aezizhu/LuciCodex/internal/config"
)

type Wizard struct {
    reader *bufio.Reader
    writer io.Writer
}

func New(reader io.Reader, writer io.Writer) *Wizard {
    return &Wizard{
        reader: bufio.NewReader(reader),
        writer: writer,
    }
}

func (w *Wizard) Run() error {
    fmt.Fprintf(w.writer, "LuciCodex Setup Wizard\n")
    fmt.Fprintf(w.writer, "===============\n\n")
    fmt.Fprintf(w.writer, "This wizard will help you configure LuciCodex for your OpenWrt router.\n\n")

    cfg := config.Config{
        Author:         "AZ <Aezi.zhu@icloud.com>",
        Endpoint:       "https://generativelanguage.googleapis.com/v1beta",
        Model:          "gemini-1.5-flash",
        Provider:       "gemini",
        DryRun:         true,
        AutoApprove:    false,
        TimeoutSeconds: 30,
        MaxCommands:    10,
        Allowlist: []string{
            `^uci(\s|$)`,
            `^ubus(\s|$)`,
            `^fw4(\s|$)`,
            `^opkg(\s|$)(update|install|remove|list|info)`,
            `^logread(\s|$)`,
            `^dmesg(\s|$)`,
            `^ip(\s|$)`,
            `^ifstatus(\s|$)`,
            `^cat(\s|$)`,
            `^tail(\s|$)`,
            `^grep(\s|$)`,
            `^awk(\s|$)`,
            `^sed(\s|$)`,
        },
        Denylist: []string{
            `^rm\s+-rf\s+/`,
            `^mkfs(\s|$)`,
            `^dd(\s|$)`,
            `^:(){:|:&};:`,
        },
        LogFile:        "/tmp/lucicodex.log",
        ElevateCommand: "",
    }

    // Step 1: Choose provider
    if err := w.setupProvider(&cfg); err != nil {
        return err
    }

    // Step 2: Configure API credentials
    if err := w.setupCredentials(&cfg); err != nil {
        return err
    }

    // Step 3: Security settings
    if err := w.setupSecurity(&cfg); err != nil {
        return err
    }

    // Step 4: Save configuration
    return w.saveConfig(cfg)
}

func (w *Wizard) setupProvider(cfg *config.Config) error {
    fmt.Fprintf(w.writer, "Step 1: Choose AI Provider\n")
    fmt.Fprintf(w.writer, "1. Gemini (Google, API key required)\n")
    fmt.Fprintf(w.writer, "2. Gemini CLI (External binary, OAuth login)\n")
    fmt.Fprintf(w.writer, "3. OpenAI (API key required)\n")
    fmt.Fprintf(w.writer, "4. Anthropic (API key required)\n")
    
    choice, err := w.readChoice("Enter choice [1-4]", 1, 4)
    if err != nil {
        return err
    }
    
    switch choice {
    case 1:
        cfg.Provider = "gemini"
        cfg.Model = w.readString("Model (default: gemini-1.5-flash)", "gemini-1.5-flash")
    case 2:
        cfg.Provider = "gemini-cli"
        cfg.ExternalGeminiPath = w.readString("Path to gemini CLI (default: /usr/bin/gemini)", "/usr/bin/gemini")
    case 3:
        cfg.Provider = "openai"
        cfg.Model = w.readString("Model (default: gpt-4o-mini)", "gpt-4o-mini")
    case 4:
        cfg.Provider = "anthropic"
        cfg.Model = w.readString("Model (default: claude-3-5-sonnet-20240620)", "claude-3-5-sonnet-20240620")
    }
    
    fmt.Fprintf(w.writer, "✓ Provider configured: %s\n\n", cfg.Provider)
    return nil
}

func (w *Wizard) setupCredentials(cfg *config.Config) error {
    fmt.Fprintf(w.writer, "Step 2: Configure Credentials\n")
    
    switch cfg.Provider {
    case "gemini":
        fmt.Fprintf(w.writer, "Get your API key from: https://aistudio.google.com/app/apikey\n")
        cfg.APIKey = w.readString("Gemini API key", "")
    case "gemini-cli":
        fmt.Fprintf(w.writer, "Install Gemini CLI: npm install -g @google/gemini-cli\n")
        fmt.Fprintf(w.writer, "Then run 'gemini' and follow OAuth login flow.\n")
        fmt.Fprintf(w.writer, "No API key needed for this mode.\n")
    case "openai":
        fmt.Fprintf(w.writer, "Get your API key from: https://platform.openai.com/api-keys\n")
        cfg.OpenAIAPIKey = w.readString("OpenAI API key", "")
    case "anthropic":
        fmt.Fprintf(w.writer, "Get your API key from: https://console.anthropic.com/\n")
        cfg.AnthropicAPIKey = w.readString("Anthropic API key", "")
    }
    
    fmt.Fprintf(w.writer, "✓ Credentials configured\n\n")
    return nil
}

func (w *Wizard) setupSecurity(cfg *config.Config) error {
    fmt.Fprintf(w.writer, "Step 3: Security Settings\n")
    
    dryRun := w.readBool("Enable dry-run mode by default? (recommended)", true)
    cfg.DryRun = dryRun
    
    if !dryRun {
        autoApprove := w.readBool("Auto-approve commands without confirmation? (not recommended)", false)
        cfg.AutoApprove = autoApprove
    }
    
    maxCmds := w.readInt("Maximum commands per request", cfg.MaxCommands, 1, 50)
    cfg.MaxCommands = maxCmds
    
    timeout := w.readInt("Command timeout (seconds)", cfg.TimeoutSeconds, 5, 300)
    cfg.TimeoutSeconds = timeout
    
    if w.readBool("Configure privilege elevation command (sudo/doas)?", false) {
        elevate := w.readString("Elevation command (e.g., 'doas -n' or 'sudo -n')", "")
        cfg.ElevateCommand = elevate
    }
    
    fmt.Fprintf(w.writer, "✓ Security settings configured\n\n")
    return nil
}

func (w *Wizard) saveConfig(cfg config.Config) error {
    fmt.Fprintf(w.writer, "Step 4: Save Configuration\n")
    
    paths := []string{
        "/etc/lucicodex/config.json",
        filepath.Join(os.Getenv("HOME"), ".config", "lucicodex", "config.json"),
    }
    
    fmt.Fprintf(w.writer, "Choose configuration location:\n")
    for i, path := range paths {
        fmt.Fprintf(w.writer, "%d. %s\n", i+1, path)
    }
    
    choice, err := w.readChoice("Enter choice", 1, len(paths))
    if err != nil {
        return err
    }
    
    configPath := paths[choice-1]
    
    // Create directory if needed
    if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
        return fmt.Errorf("create config directory: %w", err)
    }
    
    // Save config
    data, err := json.MarshalIndent(cfg, "", "  ")
    if err != nil {
        return fmt.Errorf("marshal config: %w", err)
    }
    
    if err := os.WriteFile(configPath, data, 0o600); err != nil {
        return fmt.Errorf("write config: %w", err)
    }
    
    fmt.Fprintf(w.writer, "✓ Configuration saved to %s\n\n", configPath)
    fmt.Fprintf(w.writer, "Setup complete! You can now run:\n")
    fmt.Fprintf(w.writer, "  lucicodex \"restart wifi\"\n")
    fmt.Fprintf(w.writer, "  lucicodex -interactive\n\n")
    
    return nil
}

func (w *Wizard) readString(prompt, defaultValue string) string {
    if defaultValue != "" {
        fmt.Fprintf(w.writer, "%s [%s]: ", prompt, defaultValue)
    } else {
        fmt.Fprintf(w.writer, "%s: ", prompt)
    }
    
    line, _ := w.reader.ReadString('\n')
    line = strings.TrimSpace(line)
    
    if line == "" {
        return defaultValue
    }
    return line
}

func (w *Wizard) readBool(prompt string, defaultValue bool) bool {
    defaultStr := "n"
    if defaultValue {
        defaultStr = "y"
    }
    
    for {
        fmt.Fprintf(w.writer, "%s [%s]: ", prompt, defaultStr)
        line, _ := w.reader.ReadString('\n')
        line = strings.TrimSpace(strings.ToLower(line))
        
        if line == "" {
            return defaultValue
        }
        
        if line == "y" || line == "yes" {
            return true
        }
        if line == "n" || line == "no" {
            return false
        }
        
        fmt.Fprintf(w.writer, "Please enter y/yes or n/no\n")
    }
}

func (w *Wizard) readInt(prompt string, defaultValue, min, max int) int {
    for {
        fmt.Fprintf(w.writer, "%s [%d]: ", prompt, defaultValue)
        line, _ := w.reader.ReadString('\n')
        line = strings.TrimSpace(line)
        
        if line == "" {
            return defaultValue
        }
        
        value, err := strconv.Atoi(line)
        if err != nil {
            fmt.Fprintf(w.writer, "Please enter a valid number\n")
            continue
        }
        
        if value < min || value > max {
            fmt.Fprintf(w.writer, "Please enter a number between %d and %d\n", min, max)
            continue
        }
        
        return value
    }
}

func (w *Wizard) readChoice(prompt string, min, max int) (int, error) {
    for {
        fmt.Fprintf(w.writer, "%s [%d-%d]: ", prompt, min, max)
        line, _ := w.reader.ReadString('\n')
        line = strings.TrimSpace(line)
        
        choice, err := strconv.Atoi(line)
        if err != nil {
            fmt.Fprintf(w.writer, "Please enter a valid number\n")
            continue
        }
        
        if choice < min || choice > max {
            fmt.Fprintf(w.writer, "Please enter a number between %d and %d\n", min, max)
            continue
        }
        
        return choice, nil
    }
}
