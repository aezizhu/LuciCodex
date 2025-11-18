package openwrt

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"
)

// runFn defines the signature for a function that can execute a command.
type runFn func(ctx context.Context, name string, args ...string) string

// runCommand is a function variable that points to the default implementation.
// Tests can replace this with a mock implementation.
var runCommand runFn = defaultRun

// CollectFacts gathers lightweight, non-destructive environment information
// to improve planning quality. It tolerates missing tools and timeouts.
func CollectFacts(ctx context.Context) string {
	// Apply an overall cap
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var b bytes.Buffer
	add := func(name string, out string) {
		out = strings.TrimSpace(out)
		if out == "" {
			return
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(name)
		b.WriteString(":\n")
		// limit very large outputs
		const max = 4096
		if len(out) > max {
			out = out[:max]
		}
		b.WriteString(out)
	}

	add("/etc/os-release", runCommand(ctx, "cat", "/etc/os-release"))
	add("uname -a", runCommand(ctx, "uname", "-a"))
	add("ubus system board", runCommand(ctx, "ubus", "call", "system", "board", "{}"))
	add("uci show network", runCommand(ctx, "uci", "-q", "show", "network"))
	add("uci show wireless", runCommand(ctx, "uci", "-q", "show", "wireless"))
	add("fw4 print", runCommand(ctx, "fw4", "print"))
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
