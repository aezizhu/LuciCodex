package policy

import (
	"strings"
	"testing"

	"github.com/aezizhu/LuciCodex/internal/config"
	"github.com/aezizhu/LuciCodex/internal/plan"
)

func TestValidatePlan(t *testing.T) {
	cfg := config.Config{
		Allowlist: []string{`^uci(\s|$)`, `^ubus(\s|$)`},
		Denylist:  []string{`^rm\s+-rf\s+/`},
	}
	e := New(cfg)
	cases := []struct {
		name string
		p    plan.Plan
		ok   bool
	}{
		{"ok uci", plan.Plan{Commands: []plan.PlannedCommand{{Command: []string{"uci", "show"}}}}, true},
		{"deny rm", plan.Plan{Commands: []plan.PlannedCommand{{Command: []string{"rm", "-rf", "/"}}}}, false},
		{"not allowed", plan.Plan{Commands: []plan.PlannedCommand{{Command: []string{"echo", "hi"}}}}, false},
	}
	for _, c := range cases {
		err := e.ValidatePlan(c.p)
		if c.ok && err != nil {
			t.Fatalf("%s unexpected error: %v", c.name, err)
		}
		if !c.ok && err == nil {
			t.Fatalf("%s expected error", c.name)
		}
	}
}

func TestValidatePlan_EdgeCases(t *testing.T) {
	e := New(config.Config{})

	cases := []struct {
		name string
		p    plan.Plan
		err  string
	}{
		{
			"empty command",
			plan.Plan{Commands: []plan.PlannedCommand{{Command: []string{}}}},
			"command 0 is empty",
		},
		{
			"empty arg",
			plan.Plan{Commands: []plan.PlannedCommand{{Command: []string{"echo", ""}}}},
			"command 0 arg 1 is empty",
		},
		{
			"nul char",
			plan.Plan{Commands: []plan.PlannedCommand{{Command: []string{"echo", "foo\x00bar"}}}},
			"contains NUL",
		},
		{
			"shell meta in argv0",
			plan.Plan{Commands: []plan.PlannedCommand{{Command: []string{"foo|bar"}}}},
			"contains shell metacharacters",
		},
	}

	for _, c := range cases {
		err := e.ValidatePlan(c.p)
		if err == nil {
			t.Errorf("%s: expected error, got nil", c.name)
			continue
		}
		if !strings.Contains(err.Error(), c.err) {
			t.Errorf("%s: expected error containing %q, got %q", c.name, c.err, err.Error())
		}
	}
}

func TestNew_InvalidRegex(t *testing.T) {
	// Test that New doesn't panic with invalid regex
	cfg := config.Config{
		Allowlist: []string{"["}, // invalid regex
		Denylist:  []string{"("}, // invalid regex
	}
	e := New(cfg)
	if len(e.allowREs) != 0 {
		t.Error("expected 0 allowREs")
	}
	if len(e.denyREs) != 0 {
		t.Error("expected 0 denyREs")
	}
}
