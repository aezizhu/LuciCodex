package plan

import (
    "encoding/json"
    "fmt"
    "strings"
)

// PlannedCommand represents a single command to execute safely without shell interpolation.
type PlannedCommand struct {
    Command     []string `json:"command"`
    Description string   `json:"description,omitempty"`
    NeedsRoot   bool     `json:"needs_root,omitempty"`
}

// Plan is the structured response expected from the model.
type Plan struct {
    Summary  string           `json:"summary,omitempty"`
    Commands []PlannedCommand `json:"commands"`
    Warnings []string         `json:"warnings,omitempty"`
}

// BuildInstruction returns the instruction prefix to reliably elicit a JSON plan.
func BuildInstruction(cfg interface{}) string {
    // Keep instruction concise and deterministic.
    b := &strings.Builder{}
    b.WriteString("You are an OpenWrt router command planner.\n")
    b.WriteString("Output only strict JSON that conforms to this schema:\n")
    b.WriteString("{\n  \"summary\": string,\n  \"commands\": [ { \"command\": [string, ...], \"description\": string, \"needs_root\": bool } ],\n  \"warnings\": [string]\n}\n")
    b.WriteString("Rules:\n")
    b.WriteString("- Use explicit argv arrays; do not return shell pipelines or redirections.\n")
    b.WriteString("- Prefer OpenWrt tools: uci, ubus, fw4, opkg, logread, dmesg, wifi.\n")
    b.WriteString("- Common commands:\n")
    b.WriteString("  Network: uci show network, ip addr, ifconfig, ifstatus <interface>\n")
    b.WriteString("  WiFi: wifi status, uci show wireless, wifi down/up, /etc/init.d/network restart\n")
    b.WriteString("  Firewall: fw4 print, uci show firewall\n")
    b.WriteString("  Packages: opkg update, opkg list-installed, opkg install <pkg>\n")
    b.WriteString("  System: ubus call system board, cat /proc/uptime, free, df -h\n")
    b.WriteString("  Logs: logread | tail -n 20, dmesg | tail -n 20\n")
    b.WriteString("  DNS: nslookup google.com, cat /etc/resolv.conf\n")
    b.WriteString("- Common paths: /etc/config/ (UCI), /var/log/, /sys/class/net/, /tmp/\n")
    b.WriteString("- For 'restart network': use ['/etc/init.d/network', 'restart']\n")
    b.WriteString("- For 'restart wifi': use ['wifi', 'reload'] or ['wifi', 'down'] then ['wifi', 'up']\n")
    b.WriteString("- For system logs: use ['logread'] or ['logread', '-e', 'pattern']\n")
    b.WriteString("- Limit commands to safe, idempotent operations when possible.\n")
    b.WriteString("- Keep the commands minimal and directly actionable.\n")
    return b.String()
}

// BuildInstructionWithLimit adds a hint for maximum number of commands.
func BuildInstructionWithLimit(maxCommands int) string {
    base := BuildInstruction(nil)
    if maxCommands > 0 {
        return base + "\nDo not return more than " + fmt.Sprint(maxCommands) + " commands."
    }
    return base
}

// TryUnmarshalPlan attempts to decode a JSON string to Plan.
func TryUnmarshalPlan(s string) (Plan, error) {
    var p Plan
    err := json.Unmarshal([]byte(s), &p)
    return p, err
}


