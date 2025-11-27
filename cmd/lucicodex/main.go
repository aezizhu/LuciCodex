package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/aezizhu/LuciCodex/internal/config"
	"github.com/aezizhu/LuciCodex/internal/executor"
	"github.com/aezizhu/LuciCodex/internal/llm"
	"github.com/aezizhu/LuciCodex/internal/logging"
	"github.com/aezizhu/LuciCodex/internal/openwrt"
	"github.com/aezizhu/LuciCodex/internal/plan"
	"github.com/aezizhu/LuciCodex/internal/policy"
	"github.com/aezizhu/LuciCodex/internal/repl"
	"github.com/aezizhu/LuciCodex/internal/ui"
	"github.com/aezizhu/LuciCodex/internal/wizard"
)

const version = "0.4.35"

var lockPaths = []string{"/var/lock/lucicodex.lock", "/tmp/lucicodex.lock"}

func acquireLock() (*os.File, string, error) {
	var lastErr error

	for i, path := range lockPaths {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err == nil {
			if i > 0 {
				fmt.Fprintf(os.Stderr, "Note: falling back to %s\n", path)
			}
			return f, path, nil
		}
		lastErr = err
		if os.IsExist(err) {
			return nil, "", fmt.Errorf("execution in progress (lock file exists: %s)", path)
		}
	}

	return nil, "", fmt.Errorf("failed to acquire lock: %w", lastErr)
}

func releaseLock(f *os.File) {
	if f != nil {
		name := f.Name()
		f.Close()
		os.Remove(name)
	}
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lucicodex", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		configPath  = fs.String("config", "", "path to JSON config file")
		model       = fs.String("model", "", "model name")
		provider    = fs.String("provider", "", "provider name (gemini, openai, anthropic)")
		dryRun      = fs.Bool("dry-run", true, "only print plan, do not execute")
		approve     = fs.Bool("approve", false, "auto-approve plan without confirmation")
		confirmEach = fs.Bool("confirm-each", false, "confirm each command before execution")
		timeout     = fs.Int("timeout", 0, "per-command timeout in seconds")
		maxCommands = fs.Int("max-commands", 0, "maximum number of commands to execute")
		maxRetries  = fs.Int("max-retries", -1, "maximum retry attempts for failed commands (-1 = use config)")
		autoRetry   = fs.Bool("auto-retry", true, "automatically retry failed commands with AI-generated fixes")
		logFile     = fs.String("log-file", "", "log file path")
		showVersion = fs.Bool("version", false, "print version and exit")
		jsonOutput  = fs.Bool("json", false, "emit JSON output for plan and results")
		facts       = fs.Bool("facts", true, "include environment facts in prompt")
		interactive = fs.Bool("interactive", false, "start interactive REPL mode")
		setup       = fs.Bool("setup", false, "run setup wizard")
		joinArgs    = fs.Bool("join-args", false, "join all arguments into single prompt (experimental)")
	)

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *showVersion {
		fmt.Fprintf(stdout, "LuciCodex version %s\n", version)
		return 0
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		if !*setup {
			fmt.Fprintf(stderr, "Configuration error: %v\n", err)
			fmt.Fprintf(stderr, "Run with -setup to configure LuciCodex\n")
			return 1
		}
		cfg = config.Config{}
	}

	// Track which flags were explicitly set
	setFlags := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		setFlags[f.Name] = true
	})

	if setFlags["model"] {
		cfg.Model = *model
		// Prevent provider-specific settings from overriding the explicit CLI flag
		cfg.OpenAIModel = ""
		cfg.AnthropicModel = ""
	}
	if setFlags["provider"] {
		cfg.Provider = *provider
	}
	if setFlags["timeout"] {
		cfg.TimeoutSeconds = *timeout
	}
	if setFlags["max-commands"] {
		cfg.MaxCommands = *maxCommands
	}
	if setFlags["max-retries"] {
		cfg.MaxRetries = *maxRetries
	}
	if setFlags["log-file"] {
		cfg.LogFile = *logFile
	}
	if setFlags["dry-run"] {
		cfg.DryRun = *dryRun
	}
	if setFlags["approve"] {
		cfg.AutoApprove = *approve
	}
	if setFlags["auto-retry"] {
		cfg.AutoRetry = *autoRetry
	}

	// Re-apply provider settings after CLI flag overrides
	cfg.ApplyProviderSettings()

	if !*confirmEach && cfg.ConfirmEach {
		*confirmEach = true
	}

	if *setup {
		w := wizard.New(stdin, stdout)
		if err := w.Run(); err != nil {
			fmt.Fprintf(stderr, "Setup error: %v\n", err)
			return 1
		}
		return 0
	}

	if *interactive {
		r := repl.New(cfg, stdin, stdout)
		ctx := context.Background()
		if err := r.Run(ctx); err != nil {
			fmt.Fprintf(stderr, "REPL error: %v\n", err)
			return 1
		}
		return 0
	}

	promptArgs := fs.Args()
	if len(promptArgs) == 0 {
		fmt.Fprintf(stderr, "Usage: lucicodex [flags] <prompt>\n")
		fmt.Fprintf(stderr, "Run 'lucicodex -h' for help\n")
		return 1
	}

	var prompt string
	if *joinArgs {
		prompt = strings.Join(promptArgs, " ")
	} else {
		prompt = promptArgs[0]
	}
	ctx := context.Background()

	llmProvider := llm.NewProvider(cfg)
	policyEngine := policy.New(cfg)
	execEngine := executor.New(cfg)
	logger := logging.New(cfg.LogFile)

	instruction := plan.BuildInstructionWithLimit(cfg.MaxCommands)
	if *facts {
		factsCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		envFacts := openwrt.CollectFacts(factsCtx)
		if envFacts != "" {
			instruction += "\n\nEnvironment facts (read-only):\n" + envFacts
		}
	}

	fullPrompt := instruction + "\n\nUser request: " + prompt

	// Generate plan
	planCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	p, err := llmProvider.GeneratePlan(planCtx, fullPrompt)
	if err != nil {
		fmt.Fprintf(stderr, "LLM error: %v\n", err)
		return 1
	}

	if len(p.Commands) == 0 {
		fmt.Fprintln(stdout, "No commands proposed.")
		return 0
	}

	if cfg.MaxCommands > 0 && len(p.Commands) > cfg.MaxCommands {
		p.Commands = p.Commands[:cfg.MaxCommands]
	}

	// Validate plan
	if err := policyEngine.ValidatePlan(p); err != nil {
		fmt.Fprintf(stderr, "Plan rejected by policy: %v\n", err)
		return 1
	}

	if *jsonOutput {
		if err := ui.PrintPlanJSON(stdout, p); err != nil {
			fmt.Fprintf(stderr, "JSON output error: %v\n", err)
			return 1
		}
	} else {
		ui.PrintPlan(stdout, p)
	}

	logger.Plan(prompt, p)

	if cfg.DryRun {
		if !*jsonOutput {
			fmt.Fprintln(stdout, "\nDry run mode - no execution")
		}
		return 0
	}

	if !cfg.AutoApprove {
		reader := bufio.NewReader(stdin)
		ok, err := ui.Confirm(reader, stdout, "Execute these commands?")
		if err != nil {
			fmt.Fprintf(stderr, "Confirmation error: %v\n", err)
			return 1
		}
		if !ok {
			fmt.Fprintln(stdout, "Cancelled")
			return 0
		}
	}

	lockFile, lockPath, err := acquireLock()
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	defer releaseLock(lockFile)

	fmt.Fprintf(stderr, "Acquired execution lock: %s\n", lockPath)

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigc
		releaseLock(lockFile)
		os.Exit(1)
	}()

	var results executor.Results
	if *confirmEach {
		reader := bufio.NewReader(stdin)
		for i, cmd := range p.Commands {
			fmt.Fprintf(stdout, "\nExecute command %d: %s\n", i+1, executor.FormatCommand(cmd.Command))
			ok, err := ui.Confirm(reader, stdout, "Proceed?")
			if err != nil || !ok {
				fmt.Fprintln(stdout, "Skipped")
				continue
			}
			result := execEngine.RunCommand(ctx, i, cmd)
			results.Items = append(results.Items, result)
			if result.Err != nil {
				results.Failed++
			}
		}
	} else {
		results = execEngine.RunPlan(ctx, p)
	}

	// Retry logic for failed commands
	if cfg.AutoRetry && results.Failed > 0 && cfg.MaxRetries > 0 {
		for retryAttempt := 1; retryAttempt <= cfg.MaxRetries; retryAttempt++ {
			// Find first failed command
			var failedResult *executor.Result
			for i := range results.Items {
				if results.Items[i].Err != nil {
					failedResult = &results.Items[i]
					break
				}
			}

			if failedResult == nil {
				break // No more failures
			}

			if !*jsonOutput {
				fmt.Fprintf(stderr, "\n??  Command failed: %s\n", executor.FormatCommand(failedResult.Command))
				fmt.Fprintf(stderr, "Error: %v\n", failedResult.Err)
				fmt.Fprintf(stderr, "Output: %s\n", failedResult.Output)
				fmt.Fprintf(stderr, "?? Attempting automatic fix (attempt %d/%d)...\n", retryAttempt, cfg.MaxRetries)
			}

			// Generate fix plan
			fixCtx, fixCancel := context.WithTimeout(ctx, 30*time.Second)
			fixPlan, err := llmProvider.GenerateErrorFix(fixCtx,
				executor.FormatCommand(failedResult.Command),
				failedResult.Output,
				retryAttempt)
			fixCancel()

			if err != nil {
				if !*jsonOutput {
					fmt.Fprintf(stderr, "Failed to generate fix: %v\n", err)
				}
				break
			}

			if len(fixPlan.Commands) == 0 {
				if !*jsonOutput {
					fmt.Fprintf(stderr, "No fix commands generated\n")
				}
				break
			}

			// Validate fix plan
			if err := policyEngine.ValidatePlan(fixPlan); err != nil {
				if !*jsonOutput {
					fmt.Fprintf(stderr, "Fix plan rejected by policy: %v\n", err)
				}
				break
			}

			if !*jsonOutput {
				fmt.Fprintf(stderr, "\n?? Fix plan: %s\n", fixPlan.Summary)
				for _, cmd := range fixPlan.Commands {
					fmt.Fprintf(stderr, "  ? %s\n", executor.FormatCommand(cmd.Command))
				}
			}

			// Execute fix
			fixResults := execEngine.RunPlan(ctx, fixPlan)

			// Mark original failure as retried by removing the error if fix succeeded
			if fixResults.Failed == 0 {
				if !*jsonOutput {
					fmt.Fprintf(stderr, "? Fix successful!\n")
				}
				failedResult.Err = nil
				results.Failed--

				// Append fix results to overall results
				for _, fr := range fixResults.Items {
					results.Items = append(results.Items, fr)
				}
				break
			} else {
				if !*jsonOutput {
					fmt.Fprintf(stderr, "? Fix attempt failed\n")
				}
				// Update the failed result with the new error
				for _, fr := range fixResults.Items {
					if fr.Err != nil {
						failedResult.Output = fr.Output
						failedResult.Err = fr.Err
						break
					}
				}
			}
		}
	}

	if *jsonOutput {
		if err := ui.PrintResultsJSON(stdout, results); err != nil {
			fmt.Fprintf(stderr, "JSON output error: %v\n", err)
			return 1
		}
	} else {
		ui.PrintResults(stdout, results)
	}

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
	logger.Results(items)

	if results.Failed > 0 {
		return 1
	}
	return 0
}
