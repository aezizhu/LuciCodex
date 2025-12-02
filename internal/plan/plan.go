package plan

import (
	"encoding/json"
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
func TryUnmarshalPlan(s string) (Plan, error) {
	var p Plan
	err := json.Unmarshal([]byte(s), &p)
	return p, err
}
