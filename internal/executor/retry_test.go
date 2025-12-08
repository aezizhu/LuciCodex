package executor

import (
	"context"
	"errors"
	"testing"

	"github.com/aezizhu/LuciCodex/internal/config"
	"github.com/aezizhu/LuciCodex/internal/plan"
	"github.com/aezizhu/LuciCodex/internal/policy"
)

type stubFixPlanner struct {
	plans map[string]plan.Plan
	calls []string
}

func (s *stubFixPlanner) GenerateErrorFix(ctx context.Context, originalCommand string, errorOutput string, attempt int) (plan.Plan, error) {
	s.calls = append(s.calls, originalCommand)
	if p, ok := s.plans[originalCommand]; ok {
		return p, nil
	}
	return plan.Plan{}, errors.New("no plan")
}

func TestAutoRetry_FixesMultipleFailures(t *testing.T) {
	ctx := context.Background()
	old := GetRunCommand()
	defer SetRunCommand(old)

	// Simulate failures for bad* commands and success for fix-* commands.
	SetRunCommand(func(ctx context.Context, argv []string) (string, error) {
		switch argv[0] {
		case "bad1":
			return "fail1", errors.New("fail1")
		case "bad2":
			return "fail2", errors.New("fail2")
		default:
			return "ok", nil
		}
	})

	cfg := config.Config{MaxRetries: 2, AutoRetry: true, TimeoutSeconds: 1}
	engine := New(cfg)
	pol := policy.New(config.Config{
		Allowlist: []string{`^bad1(\s|$)`, `^bad2(\s|$)`, `^fix-bad1(\s|$)`, `^fix-bad2(\s|$)`},
	})

	planInput := plan.Plan{
		Commands: []plan.PlannedCommand{
			{Command: []string{"bad1"}},
			{Command: []string{"ok"}},
			{Command: []string{"bad2"}},
		},
	}

	results := engine.RunPlan(ctx, planInput)
	if results.Failed != 2 {
		t.Fatalf("expected 2 failures initially, got %d", results.Failed)
	}

	fp := &stubFixPlanner{
		plans: map[string]plan.Plan{
			"bad1": {Commands: []plan.PlannedCommand{{Command: []string{"fix-bad1"}}}},
			"bad2": {Commands: []plan.PlannedCommand{{Command: []string{"fix-bad2"}}}},
		},
	}

	results = engine.AutoRetry(ctx, fp, pol, results, nil)

	if results.Failed != 0 {
		t.Fatalf("expected all failures fixed, got %d remaining", results.Failed)
	}
	// Original failed results should be cleared.
	for _, idx := range []int{0, 2} {
		if results.Items[idx].Err != nil {
			t.Fatalf("expected result %d error cleared, got %v", idx, results.Items[idx].Err)
		}
	}
	// Two fix commands appended.
	if len(results.Items) != 5 {
		t.Fatalf("expected 5 result items (3 original + 2 fixes), got %d", len(results.Items))
	}
	if len(fp.calls) != 2 {
		t.Fatalf("expected two fix requests, got %d", len(fp.calls))
	}
}

func TestAutoRetry_RespectsPolicy(t *testing.T) {
	ctx := context.Background()
	old := GetRunCommand()
	defer SetRunCommand(old)

	SetRunCommand(func(ctx context.Context, argv []string) (string, error) {
		if argv[0] == "bad-blocked" {
			return "fail", errors.New("fail")
		}
		return "ok", nil
	})

	cfg := config.Config{MaxRetries: 1, AutoRetry: true, TimeoutSeconds: 1}
	engine := New(cfg)
	pol := policy.New(config.Config{
		Allowlist: []string{`^bad-blocked(\s|$)`, `^fix-allowed(\s|$)`, `^fix-blocked(\s|$)`},
		Denylist:  []string{`^fix-blocked(\s|$)`},
	})

	planInput := plan.Plan{
		Commands: []plan.PlannedCommand{
			{Command: []string{"bad-blocked"}},
		},
	}

	results := engine.RunPlan(ctx, planInput)
	if results.Failed != 1 {
		t.Fatalf("expected 1 failure initially, got %d", results.Failed)
	}

	fp := &stubFixPlanner{
		plans: map[string]plan.Plan{
			"bad-blocked": {Commands: []plan.PlannedCommand{{Command: []string{"fix-blocked"}}}},
		},
	}

	results = engine.AutoRetry(ctx, fp, pol, results, nil)

	if results.Failed != 1 {
		t.Fatalf("expected failure to remain due to denylist, got %d", results.Failed)
	}
	if results.Items[0].Err == nil {
		t.Fatalf("expected original error to remain when fix plan blocked")
	}
	// No fix results should have been appended because policy blocked it.
	if len(results.Items) != 1 {
		t.Fatalf("expected only original result recorded, got %d", len(results.Items))
	}
}
