package openwrt

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// runFn defines the signature for a function that can execute a command.
type runFn func(ctx context.Context, name string, args ...string) string

// runCommand is a function variable that points to the default implementation.
// Tests can replace this with a mock implementation.
var runCommand runFn = defaultRun

// factResult holds the result of a single fact collection.
type factResult struct {
	order int
	name  string
	value string
}

// CollectFacts gathers lightweight, non-destructive environment information
// to improve planning quality. It tolerates missing tools and timeouts.
// Commands run in parallel for faster collection on resource-constrained routers.
func CollectFacts(ctx context.Context) string {
	// Apply an overall cap
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// Define facts to collect with their order for deterministic output
	type factCmd struct {
		order int
		name  string
		cmd   string
		args  []string
	}
	commands := []factCmd{
		{0, "/etc/os-release", "cat", []string{"/etc/os-release"}},
		{1, "uname -a", "uname", []string{"-a"}},
		{2, "ubus system board", "ubus", []string{"call", "system", "board", "{}"}},
		{3, "uci show network", "uci", []string{"-q", "show", "network"}},
		{4, "uci show wireless", "uci", []string{"-q", "show", "wireless"}},
		{5, "fw4 print", "fw4", []string{"print"}},
	}

	// Collect facts in parallel
	results := make([]factResult, len(commands))
	var wg sync.WaitGroup
	wg.Add(len(commands))

	for i, fc := range commands {
		go func(idx int, f factCmd) {
			defer wg.Done()
			results[idx] = factResult{
				order: f.order,
				name:  f.name,
				value: runCommand(ctx, f.cmd, f.args...),
			}
		}(i, fc)
	}
	wg.Wait()

	// Build output in deterministic order
	var b bytes.Buffer
	b.Grow(8192) // Pre-allocate for typical fact size
	for _, r := range results {
		out := strings.TrimSpace(r.value)
		if out == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(r.name)
		b.WriteString(":\n")
		// Limit very large outputs
		const max = 4096
		if len(out) > max {
			out = out[:max]
		}
		b.WriteString(out)
	}
	return b.String()
}

func defaultRun(ctx context.Context, name string, args ...string) string {
	// short per-command timeout inside the overall budget
	cctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, name, args...)
	// minimal env: rely on PATH
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return string(out)
}

// GetRunCommand returns the current run command function.
func GetRunCommand() runFn {
	return runCommand
}

// SetRunCommand sets the run command function for testing.
func SetRunCommand(fn runFn) {
	runCommand = fn
}
