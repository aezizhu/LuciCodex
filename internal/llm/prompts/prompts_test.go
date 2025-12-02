package prompts

import (
	"strings"
	"testing"
)

func TestGenerateSurvivalPrompt(t *testing.T) {
	instruction := GenerateSurvivalPrompt(0)

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

func TestGenerateSurvivalPromptWithLimit(t *testing.T) {
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
			instruction := GenerateSurvivalPrompt(tt.maxCommands)

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
