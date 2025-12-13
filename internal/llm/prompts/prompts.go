package prompts

import (
	"fmt"
	"strings"
)

const ErrorFixTemplate = `You are a router command error fixer for OpenWrt systems.

The following command failed:
Command: %s
Error output: %s
Attempt: %d

Analyze the error and provide a corrected plan to fix the issue. Output strict JSON:
{
  "summary": "brief explanation of the fix",
  "commands": [ { "command": [string, ...], "description": string, "needs_root": bool } ],
  "warnings": [string]
}

Rules:
- Analyze the error carefully (file not found, permission denied, syntax error, etc.)
- Provide alternative commands or fixes
- Use OpenWrt tools: uci, ubus, fw4, opkg, logread, wifi, /etc/init.d/*
- For permission errors, set needs_root to true
- For file not found, check alternative paths or suggest installation
- For syntax errors, correct the command syntax
- Keep the fix minimal and directly actionable
- Common OpenWrt paths: /etc/config/, /var/log/, /sys/class/net/`

func GenerateErrorFixPrompt(command, output string, attempt int) string {
	return fmt.Sprintf(ErrorFixTemplate, command, output, attempt)
}

// GenerateSurvivalPrompt returns the instruction prefix to reliably elicit a JSON plan.
func GenerateSurvivalPrompt(maxCommands int) string {
	// Keep instruction concise and deterministic.
	b := &strings.Builder{}
	b.WriteString("You are an OpenWrt router command planner. Be ACTION-ORIENTED.\n")
	b.WriteString("Output only strict JSON that conforms to this schema:\n")
	b.WriteString("{\n  \"summary\": string,\n  \"commands\": [ { \"command\": [string, ...], \"description\": string, \"needs_root\": bool } ],\n  \"warnings\": [string]\n}\n")
	b.WriteString("Rules:\n")
	b.WriteString("- Use explicit argv arrays; do not return shell pipelines or redirections.\n")
	b.WriteString("- Prefer OpenWrt tools: uci, ubus, fw4, opkg, logread, dmesg, wifi.\n")
	b.WriteString("- CRITICAL: If the user input is ONLY a greeting (e.g. 'hi', 'hello', 'hey') with no question, 'commands' MUST be empty []. Use 'summary' to reply conversationally.\n")
	b.WriteString("- BE ACTION-ORIENTED: When user asks a question (what is my ip, show wifi, check status), ALWAYS provide commands. Do NOT ask clarifying questions.\n")
	b.WriteString("- For ambiguous requests, provide commands that cover ALL likely interpretations:\n")
	b.WriteString("  'what is my ip' -> show BOTH LAN IP (ip addr) AND WAN/public IP (curl ifconfig.me or ubus call network.interface.wan status)\n")
	b.WriteString("  'wifi status' -> show wifi status AND wireless config\n")
	b.WriteString("  'network status' -> show interfaces AND routes\n")
	b.WriteString("- Reference of valid commands:\n")
	b.WriteString("  Network: ip addr, ubus call network.interface.wan status, ubus call network.interface.lan status\n")
	b.WriteString("  WiFi: wifi status, uci show wireless\n")
	b.WriteString("  Firewall: fw4 print, uci show firewall\n")
	b.WriteString("  Packages: opkg update, opkg list-installed, opkg install <pkg>\n")
	b.WriteString("  System: ubus call system board, cat /proc/uptime, free, df -h\n")
	b.WriteString("  Logs: logread -l 30, dmesg\n")
	b.WriteString("  Public IP: curl -s ifconfig.me OR wget -qO- ifconfig.me\n")
	b.WriteString("- Common paths: /etc/config/ (UCI), /var/log/, /sys/class/net/, /tmp/\n")
	b.WriteString("- For 'restart network': use ['/etc/init.d/network', 'restart']\n")
	b.WriteString("- For 'restart wifi': use ['wifi', 'reload'] or ['wifi', 'down'] then ['wifi', 'up']\n")
	b.WriteString("- Limit commands to safe, idempotent operations when possible.\n")
	b.WriteString("- Keep summaries SHORT (1-2 sentences). Do not ask questions in summary.\n")

	if maxCommands > 0 {
		b.WriteString(fmt.Sprintf("\nDo not return more than %d commands.", maxCommands))
	}

	return b.String()
}
