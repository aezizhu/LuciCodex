package executor

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/aezizhu/LuciCodex/internal/config"
	"github.com/aezizhu/LuciCodex/internal/plan"
	"github.com/aezizhu/LuciCodex/internal/policy"
)

// Output size limits to prevent unbounded memory growth
const (
	MaxOutputSize     = 512 * 1024 // 512KB per command output
	MaxTotalOutputLen = 2 * 1024 * 1024 // 2MB total for all commands
)

// ErrOutputTruncated indicates command output was truncated due to size limits
var ErrOutputTruncated = errors.New("output truncated: exceeded maximum size limit")

type Result struct {
	Index     int
	Command   []string
	Output    string
	Err       error
	Elapsed   time.Duration
	Truncated bool // True if output was truncated due to size limits
}

type Results struct {
	Items  []Result
	Failed int
}

// stringBuilderPool reuses string builders to reduce allocations during streaming
var stringBuilderPool = sync.Pool{
	New: func() interface{} {
		b := &strings.Builder{}
		b.Grow(4096) // Pre-allocate for typical command output
		return b
	},
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
	// Truncate output if it exceeds the limit
	if len(out) > MaxOutputSize {
		truncated := out[:MaxOutputSize]
		return string(truncated) + "\n... [output truncated] ...", err
	}
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
	results := Results{
		Items: make([]Result, 0, len(p.Commands)), // Pre-allocate for efficiency
	}
	for i, pc := range p.Commands {
		r := e.runOne(ctx, i, pc)
		if r.Err != nil {
			results.Failed++
		}
		results.Items = append(results.Items, r)
	}
	return results
}

// RunPlanStreaming executes a plan with real-time output streaming.
// The onStart callback is called when a command begins execution.
// The onOutput callback is called for each line of output.
// The onComplete callback is called when a command finishes.
func (e *Engine) RunPlanStreaming(ctx context.Context, p plan.Plan, w io.Writer) Results {
	results := Results{
		Items: make([]Result, 0, len(p.Commands)), // Pre-allocate for efficiency
	}
	for i, pc := range p.Commands {
		r := e.runOneStreaming(ctx, i, pc, w)
		if r.Err != nil {
			results.Failed++
		}
		results.Items = append(results.Items, r)
	}
	return results
}

func (e *Engine) runOneStreaming(ctx context.Context, index int, pc plan.PlannedCommand, w io.Writer) Result {
	start := time.Now()
	r := Result{Index: index, Command: pc.Command}
	if len(pc.Command) == 0 {
		r.Err = errors.New("empty command")
		return r
	}

	// Show command being executed
	fmt.Fprintf(w, "\n\033[1m[%d] Executing:\033[0m %s\n", index+1, FormatCommand(pc.Command))

	timeout := time.Duration(e.cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	argv := pc.Command
	if pc.NeedsRoot && strings.TrimSpace(e.cfg.ElevateCommand) != "" {
		elev := fieldsSafe(e.cfg.ElevateCommand)
		if len(elev) > 0 {
			argv = append(elev, argv...)
		}
	}

	var cmd *exec.Cmd
	if len(argv) == 1 {
		cmd = exec.CommandContext(cctx, argv[0])
	} else {
		cmd = exec.CommandContext(cctx, argv[0], argv[1:]...)
	}
	cmd.Env = minimalEnv()

	// Create pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		r.Err = err
		r.Elapsed = time.Since(start)
		return r
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		r.Err = err
		r.Elapsed = time.Since(start)
		return r
	}

	if err := cmd.Start(); err != nil {
		r.Err = err
		r.Elapsed = time.Since(start)
		return r
	}

	// Collect output while streaming (protected by mutex for concurrent access)
	// Use pooled builder to reduce allocations
	outputBuf := stringBuilderPool.Get().(*strings.Builder)
	outputBuf.Reset()
	defer stringBuilderPool.Put(outputBuf)
	var outputMu sync.Mutex
	var wg sync.WaitGroup
	var truncated bool
	wg.Add(2)

	// Stream stdout with size limit
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			outputMu.Lock()
			if outputBuf.Len() < MaxOutputSize {
				outputBuf.WriteString(line)
				outputBuf.WriteByte('\n')
			} else if !truncated {
				truncated = true
				outputBuf.WriteString("\n... [output truncated] ...\n")
			}
			outputMu.Unlock()
			fmt.Fprintf(w, "  %s\n", line)
		}
		if err := scanner.Err(); err != nil {
			outputMu.Lock()
			outputBuf.WriteString(fmt.Sprintf("\n[scanner error: %v]\n", err))
			outputMu.Unlock()
		}
	}()

	// Stream stderr with size limit
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			outputMu.Lock()
			if outputBuf.Len() < MaxOutputSize {
				outputBuf.WriteString(line)
				outputBuf.WriteByte('\n')
			} else if !truncated {
				truncated = true
				outputBuf.WriteString("\n... [output truncated] ...\n")
			}
			outputMu.Unlock()
			fmt.Fprintf(w, "  \033[33m%s\033[0m\n", line) // Yellow for stderr
		}
		if err := scanner.Err(); err != nil {
			outputMu.Lock()
			outputBuf.WriteString(fmt.Sprintf("\n[scanner error: %v]\n", err))
			outputMu.Unlock()
		}
	}()

	wg.Wait()
	err = cmd.Wait()
	r.Output = outputBuf.String()
	r.Err = err
	r.Elapsed = time.Since(start)
	r.Truncated = truncated

	// Show completion status
	if r.Err != nil {
		fmt.Fprintf(w, "  \033[31m✗ Failed\033[0m (%s): %v\n", r.Elapsed, r.Err)
	} else {
		fmt.Fprintf(w, "  \033[32m✓ Done\033[0m (%s)\n", r.Elapsed)
	}

	return r
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

// pathEnvPrefix is pre-allocated to avoid string concatenation in hot path
const pathEnvPrefix = "PATH="

func minimalEnv() []string {
	path := os.Getenv("PATH")
	if path == "" {
		path = "/usr/sbin:/usr/bin:/sbin:/bin"
	}
	// Pre-allocate buffer for PATH=<value> to avoid concatenation
	buf := make([]byte, 0, len(pathEnvPrefix)+len(path))
	buf = append(buf, pathEnvPrefix...)
	buf = append(buf, path...)
	return []string{string(buf)}
}

// FormatCommand returns a shell-like string for logging only (no execution).
func FormatCommand(argv []string) string {
	if len(argv) == 0 {
		return ""
	}
	// Fast path: if no quoting needed, avoid allocations
	needsQuoting := false
	totalLen := len(argv) - 1 // spaces between args
	for _, a := range argv {
		totalLen += len(a)
		if strings.ContainsAny(a, " \t\n'") {
			needsQuoting = true
		}
	}
	if !needsQuoting {
		return strings.Join(argv, " ")
	}
	// Slow path: quote arguments that need it
	var b strings.Builder
	b.Grow(totalLen + 20) // Extra space for quotes
	for i, a := range argv {
		if i > 0 {
			b.WriteByte(' ')
		}
		if strings.ContainsAny(a, " \t\n'") {
			b.WriteString(fmt.Sprintf("%q", a))
		} else {
			b.WriteString(a)
		}
	}
	return b.String()
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
