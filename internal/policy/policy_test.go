package policy

import (
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
    cases := []struct{
        name string
        p plan.Plan
        ok bool
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


