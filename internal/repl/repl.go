package repl

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aezizhu/LuciCodex/internal/config"
	"github.com/aezizhu/LuciCodex/internal/executor"
	"github.com/aezizhu/LuciCodex/internal/llm"
	"github.com/aezizhu/LuciCodex/internal/logging"
	"github.com/aezizhu/LuciCodex/internal/openwrt"
	"github.com/aezizhu/LuciCodex/internal/plan"
	"github.com/aezizhu/LuciCodex/internal/policy"
	"github.com/aezizhu/LuciCodex/internal/ui"
)

type REPL struct {
	cfg          config.Config
	provider     llm.Provider
	policyEngine *policy.Engine
	execEngine   *executor.Engine
	logger       *logging.Logger
	history      []string
	maxHistory   int
	reader       *bufio.Reader
	writer       io.Writer
}

func New(cfg config.Config, reader io.Reader, writer io.Writer) *REPL {
	return &REPL{
		cfg:          cfg,
		provider:     llm.NewProvider(cfg),
		policyEngine: policy.New(cfg),
		execEngine:   executor.New(cfg),
		logger:       logging.New(cfg.LogFile),
		history:      make([]string, 0),
		maxHistory:   100,
		reader:       bufio.NewReader(reader),
		writer:       writer,
	}
}

func (r *REPL) Run(ctx context.Context) error {
	fmt.Fprintf(r.writer, "LuciCodex interactive mode (provider: %s)\n", r.cfg.Provider)
	fmt.Fprintf(r.writer, "Type 'help' for commands, 'exit' to quit\n\n")

	for {
		fmt.Fprint(r.writer, "lucicodex> ")

		line, err := r.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil // Clean exit
			}
			return err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if line == "exit" || line == "quit" {
			break
		}

		if err := r.handleCommand(ctx, line, r.writer); err != nil {
			fmt.Fprintf(r.writer, "Error: %v\n", err)
		}

		fmt.Fprintln(r.writer)
	}

	return nil
}
func (r *REPL) handleCommand(ctx context.Context, line string, output io.Writer) error {
	switch {
	case line == "help":
		r.showHelp(output)
		return nil
	case line == "history":
		r.showHistory(output)
		return nil
	case line == "clear":
		r.clearHistory()
		fmt.Fprintln(output, "History cleared")
		return nil
	case line == "status":
		r.showStatus(output)
		return nil
	case strings.HasPrefix(line, "set "):
		return r.handleSet(line[4:], output)
	case strings.HasPrefix(line, "!"):
		return r.handleHistoryCommand(line[1:], ctx, output)
	default:
		return r.executePrompt(ctx, line, output)
	}
}

func (r *REPL) executePrompt(ctx context.Context, prompt string, output io.Writer) error {
	r.addToHistory(prompt)

	// Build instruction with facts
	instruction := plan.BuildInstructionWithLimit(r.cfg.MaxCommands)
	if true { // facts enabled by default in REPL
		factsCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		facts := openwrt.CollectFacts(factsCtx)
		if facts != "" {
			instruction += "\n\nEnvironment facts (read-only):\n" + facts
		}
	}

	fullPrompt := instruction + "\n\nUser request: " + prompt

	// Generate plan
	planCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	p, err := r.provider.GeneratePlan(planCtx, fullPrompt)
	if err != nil {
		return fmt.Errorf("LLM error: %w", err)
	}

	if len(p.Commands) == 0 {
		fmt.Fprintln(output, "No commands proposed.")
		return nil
	}

	if r.cfg.MaxCommands > 0 && len(p.Commands) > r.cfg.MaxCommands {
		p.Commands = p.Commands[:r.cfg.MaxCommands]
	}

	// Validate plan
	if err := r.policyEngine.ValidatePlan(p); err != nil {
		return fmt.Errorf("Plan rejected: %w", err)
	}

	// Show plan
	ui.PrintPlan(output, p)
	r.logger.Plan(prompt, p)

	if r.cfg.DryRun {
		fmt.Fprintln(output, "Dry run mode - no execution")
		return nil
	}

	// Confirm execution
	if !r.cfg.AutoApprove {
		ok, err := ui.Confirm(r.reader, output, "Execute these commands?")
		if err != nil || !ok {
			fmt.Fprintln(output, "Cancelled")
			return nil
		}
	}

	// Execute
	results := r.execEngine.RunPlan(ctx, p)
	ui.PrintResults(output, results)

	// Audit results
	items := make([]logging.ResultItem, 0, len(results.Items))
	for _, it := range results.Items {
		errStr := ""
		if it.Err != nil {
			errStr = it.Err.Error()
		}
		items = append(items, logging.ResultItem{
			Index:   it.Index,
			Command: it.Command,
			Output:  it.Output,
			Error:   errStr,
			Elapsed: it.Elapsed,
		})
	}
	r.logger.Results(items)

	return nil
}

func (r *REPL) addToHistory(cmd string) {
    r.history = append(r.history, cmd)
    if len(r.history) > r.maxHistory {
        r.history = r.history[1:]
    }
}

func (r *REPL) showHelp(output io.Writer) {
    fmt.Fprintln(output, "Available commands:")
    fmt.Fprintln(output, "  help                    - Show this help")
    fmt.Fprintln(output, "  history                 - Show command history")
    fmt.Fprintln(output, "  clear                   - Clear history")
    fmt.Fprintln(output, "  status                  - Show current configuration")
    fmt.Fprintln(output, "  set <key>=<value>       - Change configuration")
    fmt.Fprintln(output, "  !<number>               - Re-run command from history")
    fmt.Fprintln(output, "  exit, quit              - Exit interactive mode")
    fmt.Fprintln(output, "  <natural language>      - Execute AI-planned commands")
}

func (r *REPL) showHistory(output io.Writer) {
    if len(r.history) == 0 {
        fmt.Fprintln(output, "No history")
        return
    }
    
    for i, cmd := range r.history {
        fmt.Fprintf(output, "%3d  %s\n", i+1, cmd)
    }
}

func (r *REPL) clearHistory() {
    r.history = r.history[:0]
}

func (r *REPL) showStatus(output io.Writer) {
    fmt.Fprintf(output, "Provider: %s\n", r.cfg.Provider)
    fmt.Fprintf(output, "Model: %s\n", r.cfg.Model)
    fmt.Fprintf(output, "Dry run: %t\n", r.cfg.DryRun)
    fmt.Fprintf(output, "Auto approve: %t\n", r.cfg.AutoApprove)
    fmt.Fprintf(output, "Max commands: %d\n", r.cfg.MaxCommands)
    fmt.Fprintf(output, "Timeout: %ds\n", r.cfg.TimeoutSeconds)
}

func (r *REPL) handleSet(setting string, output io.Writer) error {
    parts := strings.SplitN(setting, "=", 2)
    if len(parts) != 2 {
        return fmt.Errorf("usage: set key=value")
    }
    
    key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
    
    switch key {
    case "dry-run":
        r.cfg.DryRun = value == "true"
        fmt.Fprintf(output, "Set dry-run to %t\n", r.cfg.DryRun)
    case "auto-approve":
        r.cfg.AutoApprove = value == "true"
        fmt.Fprintf(output, "Set auto-approve to %t\n", r.cfg.AutoApprove)
    case "provider":
        r.cfg.Provider = value
        r.cfg.ApplyProviderSettings() // Apply provider-specific defaults
        r.provider = llm.NewProvider(r.cfg)
        fmt.Fprintf(output, "Set provider to %s (model: %s, endpoint: %s)\n", r.cfg.Provider, r.cfg.Model, r.cfg.Endpoint)
    case "model":
        r.cfg.Model = value
        r.provider = llm.NewProvider(r.cfg)
        fmt.Fprintf(output, "Set model to %s\n", r.cfg.Model)
    default:
        return fmt.Errorf("unknown setting: %s", key)
    }
    
    return nil
}

func (r *REPL) handleHistoryCommand(indexStr string, ctx context.Context, output io.Writer) error {
    if len(r.history) == 0 {
        return fmt.Errorf("no history")
    }
    
    var index int
    if indexStr == "" {
        index = len(r.history)
    } else {
        if _, err := fmt.Sscanf(indexStr, "%d", &index); err != nil {
            return fmt.Errorf("invalid history index")
        }
    }
    
    if index < 1 || index > len(r.history) {
        return fmt.Errorf("history index out of range")
    }
    
    cmd := r.history[index-1]
    fmt.Fprintf(output, "Re-running: %s\n", cmd)
    return r.executePrompt(ctx, cmd, output)
}
