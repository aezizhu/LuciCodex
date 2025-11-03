package plan

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildInstruction(t *testing.T) {
	instruction := BuildInstruction(nil)

	if !strings.Contains(instruction, "router command planner") {
		t.Error("expected instruction to contain 'router command planner'")
	}
	if !strings.Contains(instruction, "strict JSON") {
		t.Error("expected instruction to mention strict JSON")
	}
	if !strings.Contains(instruction, "argv arrays") {
		t.Error("expected instruction to mention argv arrays")
	}
	if !strings.Contains(instruction, "OpenWrt tools") {
		t.Error("expected instruction to mention OpenWrt tools")
	}
	if !strings.Contains(instruction, "uci") {
		t.Error("expected instruction to mention uci")
	}
	if !strings.Contains(instruction, "commands") {
		t.Error("expected instruction to mention commands")
	}
}

func TestBuildInstructionWithLimit(t *testing.T) {
	tests := []struct {
		name        string
		maxCommands int
		wantLimit   bool
	}{
		{"with limit", 5, true},
		{"with zero limit", 0, false},
		{"with negative limit", -1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instruction := BuildInstructionWithLimit(tt.maxCommands)

			if !strings.Contains(instruction, "router command planner") {
				t.Error("expected instruction to contain base content")
			}

			if tt.wantLimit {
				if !strings.Contains(instruction, "Do not return more than") {
					t.Error("expected instruction to contain limit message")
				}
				if !strings.Contains(instruction, "5 commands") {
					t.Error("expected instruction to contain specific limit")
				}
			} else {
				if strings.Contains(instruction, "Do not return more than") {
					t.Error("expected no limit message when maxCommands <= 0")
				}
			}
		})
	}
}

func TestTryUnmarshalPlan_Valid(t *testing.T) {
	validJSON := `{
		"summary": "Test plan",
		"commands": [
			{
				"command": ["uci", "show", "network"],
				"description": "Show network config",
				"needs_root": false
			},
			{
				"command": ["ubus", "call", "system", "info"],
				"description": "Get system info",
				"needs_root": true
			}
		],
		"warnings": ["This is a test warning"]
	}`

	plan, err := TryUnmarshalPlan(validJSON)
	if err != nil {
		t.Fatalf("TryUnmarshalPlan failed: %v", err)
	}

	if plan.Summary != "Test plan" {
		t.Errorf("expected summary 'Test plan', got %q", plan.Summary)
	}

	if len(plan.Commands) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(plan.Commands))
	}

	cmd1 := plan.Commands[0]
	if len(cmd1.Command) != 3 {
		t.Errorf("expected command 1 to have 3 args, got %d", len(cmd1.Command))
	}
	if cmd1.Command[0] != "uci" {
		t.Errorf("expected command 1 arg 0 to be 'uci', got %q", cmd1.Command[0])
	}
	if cmd1.Description != "Show network config" {
		t.Errorf("expected command 1 description 'Show network config', got %q", cmd1.Description)
	}
	if cmd1.NeedsRoot {
		t.Error("expected command 1 needs_root to be false")
	}

	cmd2 := plan.Commands[1]
	if len(cmd2.Command) != 4 {
		t.Errorf("expected command 2 to have 4 args, got %d", len(cmd2.Command))
	}
	if cmd2.Command[0] != "ubus" {
		t.Errorf("expected command 2 arg 0 to be 'ubus', got %q", cmd2.Command[0])
	}
	if !cmd2.NeedsRoot {
		t.Error("expected command 2 needs_root to be true")
	}

	if len(plan.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(plan.Warnings))
	}
	if plan.Warnings[0] != "This is a test warning" {
		t.Errorf("expected warning 'This is a test warning', got %q", plan.Warnings[0])
	}
}

func TestTryUnmarshalPlan_MinimalValid(t *testing.T) {
	minimalJSON := `{
		"commands": [
			{
				"command": ["echo", "hello"]
			}
		]
	}`

	plan, err := TryUnmarshalPlan(minimalJSON)
	if err != nil {
		t.Fatalf("TryUnmarshalPlan failed: %v", err)
	}

	if plan.Summary != "" {
		t.Errorf("expected empty summary, got %q", plan.Summary)
	}

	if len(plan.Commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(plan.Commands))
	}

	if len(plan.Warnings) != 0 {
		t.Errorf("expected no warnings, got %d", len(plan.Warnings))
	}
}

func TestTryUnmarshalPlan_EmptyCommands(t *testing.T) {
	emptyJSON := `{
		"summary": "No commands",
		"commands": []
	}`

	plan, err := TryUnmarshalPlan(emptyJSON)
	if err != nil {
		t.Fatalf("TryUnmarshalPlan failed: %v", err)
	}

	if len(plan.Commands) != 0 {
		t.Errorf("expected 0 commands, got %d", len(plan.Commands))
	}
}

func TestTryUnmarshalPlan_InvalidJSON(t *testing.T) {
	invalidJSON := `{
		"summary": "Invalid
		"commands": [
	}`

	_, err := TryUnmarshalPlan(invalidJSON)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestTryUnmarshalPlan_WrongSchema(t *testing.T) {
	wrongSchema := `{
		"wrong_field": "value"
	}`

	plan, err := TryUnmarshalPlan(wrongSchema)
	if err != nil {
		t.Fatalf("TryUnmarshalPlan failed: %v", err)
	}

	if len(plan.Commands) != 0 {
		t.Errorf("expected 0 commands for wrong schema, got %d", len(plan.Commands))
	}
}

func TestPlannedCommand_JSONMarshaling(t *testing.T) {
	cmd := PlannedCommand{
		Command:     []string{"uci", "set", "network.lan.ipaddr=192.168.1.1"},
		Description: "Set LAN IP address",
		NeedsRoot:   true,
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		t.Fatalf("failed to marshal PlannedCommand: %v", err)
	}

	var unmarshaled PlannedCommand
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal PlannedCommand: %v", err)
	}

	if len(unmarshaled.Command) != len(cmd.Command) {
		t.Errorf("expected %d command args, got %d", len(cmd.Command), len(unmarshaled.Command))
	}
	for i, arg := range cmd.Command {
		if unmarshaled.Command[i] != arg {
			t.Errorf("command arg %d: expected %q, got %q", i, arg, unmarshaled.Command[i])
		}
	}
	if unmarshaled.Description != cmd.Description {
		t.Errorf("expected description %q, got %q", cmd.Description, unmarshaled.Description)
	}
	if unmarshaled.NeedsRoot != cmd.NeedsRoot {
		t.Errorf("expected needs_root %v, got %v", cmd.NeedsRoot, unmarshaled.NeedsRoot)
	}
}

func TestPlan_JSONMarshaling(t *testing.T) {
	plan := Plan{
		Summary: "Test plan summary",
		Commands: []PlannedCommand{
			{
				Command:     []string{"ls", "-la"},
				Description: "List files",
				NeedsRoot:   false,
			},
		},
		Warnings: []string{"Warning 1", "Warning 2"},
	}

	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("failed to marshal Plan: %v", err)
	}

	var unmarshaled Plan
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal Plan: %v", err)
	}

	if unmarshaled.Summary != plan.Summary {
		t.Errorf("expected summary %q, got %q", plan.Summary, unmarshaled.Summary)
	}
	if len(unmarshaled.Commands) != len(plan.Commands) {
		t.Errorf("expected %d commands, got %d", len(plan.Commands), len(unmarshaled.Commands))
	}
	if len(unmarshaled.Warnings) != len(plan.Warnings) {
		t.Errorf("expected %d warnings, got %d", len(plan.Warnings), len(unmarshaled.Warnings))
	}
}

func TestTryUnmarshalPlan_ComplexCommands(t *testing.T) {
	complexJSON := `{
		"summary": "Complex operations",
		"commands": [
			{
				"command": ["opkg", "update"],
				"description": "Update package lists"
			},
			{
				"command": ["opkg", "install", "luci-app-firewall"],
				"description": "Install firewall app",
				"needs_root": true
			},
			{
				"command": ["uci", "set", "firewall.@rule[0].enabled=1"],
				"description": "Enable firewall rule",
				"needs_root": true
			},
			{
				"command": ["uci", "commit", "firewall"],
				"description": "Commit firewall changes",
				"needs_root": true
			},
			{
				"command": ["fw4", "reload"],
				"description": "Reload firewall",
				"needs_root": true
			}
		],
		"warnings": [
			"This will modify firewall configuration",
			"Ensure you have backup access to the router"
		]
	}`

	plan, err := TryUnmarshalPlan(complexJSON)
	if err != nil {
		t.Fatalf("TryUnmarshalPlan failed: %v", err)
	}

	if len(plan.Commands) != 5 {
		t.Errorf("expected 5 commands, got %d", len(plan.Commands))
	}

	if len(plan.Warnings) != 2 {
		t.Errorf("expected 2 warnings, got %d", len(plan.Warnings))
	}

	rootCommands := 0
	for _, cmd := range plan.Commands {
		if cmd.NeedsRoot {
			rootCommands++
		}
	}
	if rootCommands != 4 {
		t.Errorf("expected 4 commands needing root, got %d", rootCommands)
	}
}
