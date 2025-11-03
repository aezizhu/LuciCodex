package plugins

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "time"

    "github.com/aezizhu/LuciCodex/internal/plan"
)

// Plugin represents a plugin that can extend LuciCodex functionality
type Plugin interface {
    Name() string
    Description() string
    CanHandle(prompt string) bool
    GeneratePlan(ctx context.Context, prompt string) (plan.Plan, error)
}

// ExternalPlugin implements Plugin for external executable plugins
type ExternalPlugin struct {
    name        string
    description string
    path        string
    keywords    []string
}

// PluginMetadata describes plugin capabilities
type PluginMetadata struct {
    Name        string   `json:"name"`
    Description string   `json:"description"`
    Keywords    []string `json:"keywords"`
    Version     string   `json:"version"`
    Author      string   `json:"author"`
}

// Manager manages the plugin system
type Manager struct {
    plugins    []Plugin
    pluginDirs []string
}

func NewManager(pluginDirs []string) *Manager {
    return &Manager{
        plugins:    make([]Plugin, 0),
        pluginDirs: pluginDirs,
    }
}

func (m *Manager) LoadPlugins() error {
    for _, dir := range m.pluginDirs {
        if err := m.loadFromDir(dir); err != nil {
            // Continue loading other directories even if one fails
            continue
        }
    }
    return nil
}

func (m *Manager) loadFromDir(dir string) error {
    entries, err := os.ReadDir(dir)
    if err != nil {
        return err
    }

    for _, entry := range entries {
        if entry.IsDir() {
            continue
        }
        
        pluginPath := filepath.Join(dir, entry.Name())
        
        // Check if it's executable
        info, err := entry.Info()
        if err != nil {
            continue
        }
        
        if info.Mode()&0o111 == 0 {
            continue
        }
        
        // Try to load metadata
        plugin, err := m.loadExternalPlugin(pluginPath)
        if err != nil {
            continue
        }
        
        m.plugins = append(m.plugins, plugin)
    }
    
    return nil
}

func (m *Manager) loadExternalPlugin(path string) (*ExternalPlugin, error) {
    // Get plugin metadata
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    cmd := exec.CommandContext(ctx, path, "--metadata")
    output, err := cmd.Output()
    if err != nil {
        return nil, fmt.Errorf("failed to get metadata: %w", err)
    }
    
    var metadata PluginMetadata
    if err := json.Unmarshal(output, &metadata); err != nil {
        return nil, fmt.Errorf("invalid metadata: %w", err)
    }
    
    return &ExternalPlugin{
        name:        metadata.Name,
        description: metadata.Description,
        path:        path,
        keywords:    metadata.Keywords,
    }, nil
}

func (m *Manager) FindPlugin(prompt string) Plugin {
    for _, plugin := range m.plugins {
        if plugin.CanHandle(prompt) {
            return plugin
        }
    }
    return nil
}

func (m *Manager) ListPlugins() []Plugin {
    return m.plugins
}

// ExternalPlugin implementation
func (p *ExternalPlugin) Name() string {
    return p.name
}

func (p *ExternalPlugin) Description() string {
    return p.description
}

func (p *ExternalPlugin) CanHandle(prompt string) bool {
    promptLower := strings.ToLower(prompt)
    for _, keyword := range p.keywords {
        if strings.Contains(promptLower, strings.ToLower(keyword)) {
            return true
        }
    }
    return false
}

func (p *ExternalPlugin) GeneratePlan(ctx context.Context, prompt string) (plan.Plan, error) {
    cmd := exec.CommandContext(ctx, p.path, "--plan", prompt)
    output, err := cmd.Output()
    if err != nil {
        return plan.Plan{}, fmt.Errorf("plugin execution failed: %w", err)
    }
    
    var planResult plan.Plan
    if err := json.Unmarshal(output, &planResult); err != nil {
        return plan.Plan{}, fmt.Errorf("invalid plan output: %w", err)
    }
    
    return planResult, nil
}

// Built-in plugins

// NetworkPlugin handles network-related requests
type NetworkPlugin struct{}

func (p *NetworkPlugin) Name() string {
    return "network"
}

func (p *NetworkPlugin) Description() string {
    return "Handle network configuration and diagnostics"
}

func (p *NetworkPlugin) CanHandle(prompt string) bool {
    keywords := []string{"network", "wifi", "ethernet", "interface", "ip", "route", "dns"}
    promptLower := strings.ToLower(prompt)
    
    for _, keyword := range keywords {
        if strings.Contains(promptLower, keyword) {
            return true
        }
    }
    return false
}

func (p *NetworkPlugin) GeneratePlan(ctx context.Context, prompt string) (plan.Plan, error) {
    promptLower := strings.ToLower(prompt)
    
    var commands []plan.PlannedCommand
    
    if strings.Contains(promptLower, "restart") && strings.Contains(promptLower, "wifi") {
        commands = append(commands, plan.PlannedCommand{
            Command:     []string{"uci", "set", "wireless.@wifi-device[0].disabled=1"},
            Description: "Disable WiFi device",
        })
        commands = append(commands, plan.PlannedCommand{
            Command:     []string{"uci", "commit", "wireless"},
            Description: "Commit wireless changes",
        })
        commands = append(commands, plan.PlannedCommand{
            Command:     []string{"wifi", "reload"},
            Description: "Reload WiFi configuration",
        })
        commands = append(commands, plan.PlannedCommand{
            Command:     []string{"uci", "delete", "wireless.@wifi-device[0].disabled"},
            Description: "Re-enable WiFi device",
        })
        commands = append(commands, plan.PlannedCommand{
            Command:     []string{"uci", "commit", "wireless"},
            Description: "Commit wireless changes",
        })
        commands = append(commands, plan.PlannedCommand{
            Command:     []string{"wifi", "reload"},
            Description: "Reload WiFi configuration",
        })
    } else if strings.Contains(promptLower, "show") && strings.Contains(promptLower, "interface") {
        commands = append(commands, plan.PlannedCommand{
            Command:     []string{"uci", "show", "network"},
            Description: "Show network configuration",
        })
        commands = append(commands, plan.PlannedCommand{
            Command:     []string{"ip", "addr", "show"},
            Description: "Show IP addresses",
        })
    }
    
    return plan.Plan{
        Summary:  "Network operation: " + prompt,
        Commands: commands,
    }, nil
}

// FirewallPlugin handles firewall-related requests
type FirewallPlugin struct{}

func (p *FirewallPlugin) Name() string {
    return "firewall"
}

func (p *FirewallPlugin) Description() string {
    return "Handle firewall configuration and rules"
}

func (p *FirewallPlugin) CanHandle(prompt string) bool {
    keywords := []string{"firewall", "port", "block", "allow", "rule", "fw4"}
    promptLower := strings.ToLower(prompt)
    
    for _, keyword := range keywords {
        if strings.Contains(promptLower, keyword) {
            return true
        }
    }
    return false
}

func (p *FirewallPlugin) GeneratePlan(ctx context.Context, prompt string) (plan.Plan, error) {
    promptLower := strings.ToLower(prompt)
    
    var commands []plan.PlannedCommand
    
    if strings.Contains(promptLower, "open") && strings.Contains(promptLower, "port") {
        // Extract port number if possible
        port := "22" // default
        if strings.Contains(promptLower, "22") {
            port = "22"
        } else if strings.Contains(promptLower, "80") {
            port = "80"
        } else if strings.Contains(promptLower, "443") {
            port = "443"
        }
        
        commands = append(commands, plan.PlannedCommand{
            Command:     []string{"uci", "add", "firewall", "rule"},
            Description: "Add new firewall rule",
        })
        commands = append(commands, plan.PlannedCommand{
            Command:     []string{"uci", "set", "firewall.@rule[-1].name=Allow_Port_" + port},
            Description: "Set rule name",
        })
        commands = append(commands, plan.PlannedCommand{
            Command:     []string{"uci", "set", "firewall.@rule[-1].src=wan"},
            Description: "Set source zone",
        })
        commands = append(commands, plan.PlannedCommand{
            Command:     []string{"uci", "set", "firewall.@rule[-1].proto=tcp"},
            Description: "Set protocol",
        })
        commands = append(commands, plan.PlannedCommand{
            Command:     []string{"uci", "set", "firewall.@rule[-1].dest_port=" + port},
            Description: "Set destination port",
        })
        commands = append(commands, plan.PlannedCommand{
            Command:     []string{"uci", "set", "firewall.@rule[-1].target=ACCEPT"},
            Description: "Set target to accept",
        })
        commands = append(commands, plan.PlannedCommand{
            Command:     []string{"uci", "commit", "firewall"},
            Description: "Commit firewall changes",
        })
        commands = append(commands, plan.PlannedCommand{
            Command:     []string{"fw4", "reload"},
            Description: "Reload firewall",
        })
    }
    
    return plan.Plan{
        Summary:  "Firewall operation: " + prompt,
        Commands: commands,
    }, nil
}

// GetBuiltinPlugins returns all built-in plugins
func GetBuiltinPlugins() []Plugin {
    return []Plugin{
        &NetworkPlugin{},
        &FirewallPlugin{},
    }
}
