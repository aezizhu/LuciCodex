package executor

import (
    "context"
    "errors"
    "fmt"
    "os"
    "os/exec"
    "strings"
    "syscall"
    "time"

    "github.com/aezizhu/LuciCodex/internal/config"
    "github.com/aezizhu/LuciCodex/internal/plan"
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

type Engine struct {
    cfg config.Config
}

func New(cfg config.Config) *Engine { return &Engine{cfg: cfg} }

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
    var cmd *exec.Cmd
    if len(argv) == 1 {
        cmd = exec.CommandContext(cctx, argv[0])
    } else {
        cmd = exec.CommandContext(cctx, argv[0], argv[1:]...)
    }
    // Drop env except PATH
    cmd.Env = minimalEnv()
    // Ensure hard kill on deadline
    cmd = commandWithContext(cctx, cmd)

    out, err := cmd.CombinedOutput()
    r.Output = string(out)
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

// commandWithContext ensures the process receives SIGKILL on context deadline.
func commandWithContext(ctx context.Context, cmd *exec.Cmd) *exec.Cmd {
    go func() {
        <-ctx.Done()
        if ctx.Err() != nil && cmd.Process != nil {
            // Try SIGTERM then SIGKILL
            _ = cmd.Process.Signal(syscall.SIGTERM)
            time.Sleep(500 * time.Millisecond)
            _ = cmd.Process.Kill()
        }
    }()
    return cmd
}

// FormatCommand returns a shell-like string for logging only (no execution).
func FormatCommand(argv []string) string {
    q := make([]string, 0, len(argv))
    for _, a := range argv {
        if strings.ContainsAny(a, " \t\n\"'") {
            q = append(q, fmt.Sprintf("%q", a))
        } else {
            q = append(q, a)
        }
    }
    return strings.Join(q, " ")
}


