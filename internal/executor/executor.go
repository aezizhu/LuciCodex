package executor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/aezizhu/LuciCodex/internal/config"
	"github.com/aezizhu/LuciCodex/internal/plan"
	"github.com/aezizhu/LuciCodex/internal/policy"
)

type Result struct {
	Index   int
	Command []string
	Output  string
	Err     error
	Elapsed time.Duration
}

type Results struct {
	Items  []Result
	Failed int
}

// For testing, allow overriding command execution
type execFn func(ctx context.Context, argv []string) (string, error)

var runCommand execFn = defaultRunCommand

func defaultRunCommand(ctx context.Context, argv []string) (string, error) {
	var cmd *exec.Cmd
	if len(argv) == 1 {
		cmd = exec.CommandContext(ctx, argv[0])
	} else {
		cmd = exec.CommandContext(ctx, argv[0], argv[1:]...)
	}
	// Drop env except PATH
	cmd.Env = minimalEnv()

	out, err := cmd.CombinedOutput()
	return string(out), err
}

// GetRunCommand returns the current run command function.
func GetRunCommand() execFn {
	return runCommand
}

// SetRunCommand sets the run command function for testing.
func SetRunCommand(fn execFn) {
	runCommand = fn
}

type Engine struct {
	cfg config.Config
}

func New(cfg config.Config) *Engine { return &Engine{cfg: cfg} }

// FixPlanner provides fixes for failed commands.
type FixPlanner interface {
	GenerateErrorFix(ctx context.Context, originalCommand string, errorOutput string, attempt int) (plan.Plan, error)
}

func (e *Engine) RunPlan(ctx context.Context, p plan.Plan) Results {
	results := Results{}
	for i, pc := range p.Commands {
		r := e.runOne(ctx, i, pc)
		if r.Err != nil {
			results.Failed++
		}
		results.Items = append(results.Items, r)
	}
	return results
}

// RunCommand executes a single planned command and returns the result.
func (e *Engine) RunCommand(ctx context.Context, index int, pc plan.PlannedCommand) Result {
	return e.runOne(ctx, index, pc)
}

func (e *Engine) runOne(ctx context.Context, index int, pc plan.PlannedCommand) Result {
	start := time.Now()
	r := Result{Index: index, Command: pc.Command}
	if len(pc.Command) == 0 {
		r.Err = errors.New("empty command")
		return r
	}
	// Set a timeout per command
	timeout := time.Duration(e.cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	// No shell; exec argv directly. Optionally prefix with elevation tool.
	argv := pc.Command
	if pc.NeedsRoot && strings.TrimSpace(e.cfg.ElevateCommand) != "" {
		// Split elevate command into tokens (simple whitespace split; avoid shell features)
		elev := fieldsSafe(e.cfg.ElevateCommand)
		if len(elev) > 0 {
			argv = append(elev, argv...)
		}
	}

	out, err := runCommand(cctx, argv)
	r.Output = out
	r.Err = err
	r.Elapsed = time.Since(start)
	return r
}

func minimalEnv() []string {
	path := os.Getenv("PATH")
	if path == "" {
		path = "/usr/sbin:/usr/bin:/sbin:/bin"
	}
	return []string{"PATH=" + path}
}

// FormatCommand returns a shell-like string for logging only (no execution).
func FormatCommand(argv []string) string {
	q := make([]string, 0, len(argv))
	for _, a := range argv {
		if strings.ContainsAny(a, " 	\n'") {
			q = append(q, fmt.Sprintf("%q", a))
		} else {
			q = append(q, a)
		}
	}
	return strings.Join(q, " ")
}

// AutoRetry attempts to fix each failing command up to MaxRetries using the provided planner.
// It validates fix plans with the supplied policy engine (if non-nil) before execution.
// Optional logf can be provided to emit user-facing messages.
func (e *Engine) AutoRetry(ctx context.Context, planner FixPlanner, pol *policy.Engine, results Results, logf func(format string, args ...interface{})) Results {
	if !e.cfg.AutoRetry || e.cfg.MaxRetries <= 0 || results.Failed == 0 {
		return results
	}

	for attempt := 1; attempt <= e.cfg.MaxRetries && results.Failed > 0; attempt++ {
		// Snapshot failing indices to avoid re-processing appended fix results within the same attempt.
		failing := make([]int, 0, results.Failed)
		for i := range results.Items {
			if results.Items[i].Err != nil {
				failing = append(failing, i)
			}
		}
		for _, idx := range failing {
			res := &results.Items[idx]
			if res.Err == nil || results.Failed == 0 {
				continue
			}

			origCmd := FormatCommand(res.Command)
			if logf != nil {
				logf("\n??  Command failed: %s\n", origCmd)
				logf("Error: %v\n", res.Err)
				logf("Output: %s\n", res.Output)
				logf("?? Attempting automatic fix (attempt %d/%d)...\n", attempt, e.cfg.MaxRetries)
			}

			fixCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			fixPlan, err := planner.GenerateErrorFix(fixCtx, origCmd, res.Output, attempt)
			cancel()
			if err != nil || len(fixPlan.Commands) == 0 {
				if logf != nil {
					if err != nil {
						logf("Failed to generate fix: %v\n", err)
					} else {
						logf("No fix commands generated\n")
					}
				}
				continue
			}

			if pol != nil {
				if err := pol.ValidatePlan(fixPlan); err != nil {
					if logf != nil {
						logf("Fix plan rejected by policy: %v\n", err)
					}
					continue
				}
			}

			if logf != nil {
				if fixPlan.Summary != "" {
					logf("\n?? Fix plan: %s\n", fixPlan.Summary)
				}
				for _, cmd := range fixPlan.Commands {
					logf("  ? %s\n", FormatCommand(cmd.Command))
				}
			}

			fixResults := e.RunPlan(ctx, fixPlan)
			if fixResults.Failed == 0 {
				results.Items[idx].Err = nil
				results.Failed--
				if logf != nil {
					logf("? Fix successful!\n")
				}
			} else {
				for _, fr := range fixResults.Items {
					if fr.Err != nil {
						results.Items[idx].Output = fr.Output
						results.Items[idx].Err = fr.Err
						break
					}
				}
				if logf != nil {
					logf("? Fix attempt failed\n")
				}
			}
			results.Items = append(results.Items, fixResults.Items...)
		}
	}

	return results
}
