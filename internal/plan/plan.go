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

// TryUnmarshalPlan attempts to decode a JSON string to Plan.
// It tries to extract JSON from markdown code blocks or raw text.
func TryUnmarshalPlan(s string) (Plan, error) {
	var p Plan

	// First try direct unmarshal
	if err := json.Unmarshal([]byte(s), &p); err == nil && len(p.Commands) > 0 {
		return p, nil
	}

	// Try extracting from markdown/text
	extracted := extractJSON(s)
	if err := json.Unmarshal([]byte(extracted), &p); err == nil {
		return p, nil
	}

	return p, fmt.Errorf("failed to parse plan from: %s", s)
}

func extractJSON(s string) string {
	// 1. Try to find markdown code block
	if start := strings.Index(s, "```json"); start != -1 {
		s = s[start+7:]
		if end := strings.Index(s, "```"); end != -1 {
			return strings.TrimSpace(s[:end])
		}
	} else if start := strings.Index(s, "```"); start != -1 {
		// Generic code block
		s = s[start+3:]
		if end := strings.Index(s, "```"); end != -1 {
			return strings.TrimSpace(s[:end])
		}
	}

	// 2. Fallback: find first outer brace pair
	start := -1
	depth := 0
	for i, ch := range s {
		if ch == '{' {
			if depth == 0 {
				start = i
			}
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 && start >= 0 {
				return s[start : i+1]
			}
		}
	}
	return s
}
