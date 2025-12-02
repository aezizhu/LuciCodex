package prompts

import "fmt"

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
