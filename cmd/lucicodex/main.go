package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
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

const version = "0.3.0"

func acquireLock() (*os.File, string, error) {
	lockPaths := []string{"/var/lock/lucicodex.lock", "/tmp/lucicodex.lock"}
	var lastErr error
	
	for i, lockPath := range lockPaths {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err == nil {
			if i > 0 {
				fmt.Fprintf(os.Stderr, "Note: falling back to %s\n", lockPath)
			}
			return f, lockPath, nil
		}
		lastErr = err
		if os.IsExist(err) {
			return nil, "", fmt.Errorf("execution in progress (lock file exists: %s)", lockPath)
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
	var (
		configPath  = flag.String("config", "", "path to JSON config file")
		model       = flag.String("model", "", "model name")
		provider    = flag.String("provider", "", "provider name (gemini, openai, anthropic, gemini-cli)")
		dryRun      = flag.Bool("dry-run", true, "only print plan, do not execute")
		approve     = flag.Bool("approve", false, "auto-approve plan without confirmation")
		confirmEach = flag.Bool("confirm-each", false, "confirm each command before execution")
		timeout     = flag.Int("timeout", 0, "per-command timeout in seconds")
		maxCommands = flag.Int("max-commands", 0, "maximum number of commands to execute")
		maxRetries  = flag.Int("max-retries", -1, "maximum retry attempts for failed commands (-1 = use config)")
		autoRetry   = flag.Bool("auto-retry", true, "automatically retry failed commands with AI-generated fixes")
		logFile     = flag.String("log-file", "", "log file path")
		showVersion = flag.Bool("version", false, "print version and exit")
		jsonOutput  = flag.Bool("json", false, "emit JSON output for plan and results")
		facts       = flag.Bool("facts", true, "include environment facts in prompt")
		interactive = flag.Bool("interactive", false, "start interactive REPL mode")
		setup       = flag.Bool("setup", false, "run setup wizard")
		joinArgs    = flag.Bool("join-args", false, "join all arguments into single prompt (experimental)")
	)

	flag.Parse()

	if *showVersion {
		fmt.Printf("LuciCodex version %s\n", version)
		os.Exit(0)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		if !*setup {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
            fmt.Fprintf(os.Stderr, "Run with -setup to configure LuciCodex\n")
			os.Exit(1)
		}
		cfg = config.Config{}
	}

	if *model != "" {
		cfg.Model = *model
	}
	if *provider != "" {
		cfg.Provider = *provider
	}
	if *timeout > 0 {
		cfg.TimeoutSeconds = *timeout
	}
	if *maxCommands > 0 {
		cfg.MaxCommands = *maxCommands
	}
	if *maxRetries >= 0 {
		cfg.MaxRetries = *maxRetries
	}
	if *logFile != "" {
		cfg.LogFile = *logFile
	}
	cfg.DryRun = *dryRun
	cfg.AutoApprove = *approve
	cfg.AutoRetry = *autoRetry
	
	if !*confirmEach && cfg.ConfirmEach {
		*confirmEach = true
	}

	if *setup {
		w := wizard.New(os.Stdin, os.Stdout)
		if err := w.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Setup error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if *interactive {
		r := repl.New(cfg)
		ctx := context.Background()
		if err := r.Run(ctx, os.Stdin, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "REPL error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: lucicodex [flags] <prompt>\n")
		fmt.Fprintf(os.Stderr, "Run 'lucicodex -h' for help\n")
		os.Exit(1)
	}

	var prompt string
	if *joinArgs {
		prompt = strings.Join(args, " ")
	} else {
		prompt = args[0]
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
		fmt.Fprintf(os.Stderr, "LLM error: %v\n", err)
		os.Exit(1)
	}

	if len(p.Commands) == 0 {
		fmt.Println("No commands proposed.")
		os.Exit(0)
	}

	if cfg.MaxCommands > 0 && len(p.Commands) > cfg.MaxCommands {
		p.Commands = p.Commands[:cfg.MaxCommands]
	}

	// Validate plan
	if err := policyEngine.ValidatePlan(p); err != nil {
		fmt.Fprintf(os.Stderr, "Plan rejected by policy: %v\n", err)
		os.Exit(1)
	}

	if *jsonOutput {
		if err := ui.PrintPlanJSON(os.Stdout, p); err != nil {
			fmt.Fprintf(os.Stderr, "JSON output error: %v\n", err)
			os.Exit(1)
		}
	} else {
		ui.PrintPlan(os.Stdout, p)
	}

	logger.Plan(prompt, p)

	if cfg.DryRun {
		if !*jsonOutput {
			fmt.Println("\nDry run mode - no execution")
		}
		os.Exit(0)
	}

	if !cfg.AutoApprove {
		reader := bufio.NewReader(os.Stdin)
		ok, err := ui.Confirm(reader, os.Stdout, "Execute these commands?")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Confirmation error: %v\n", err)
			os.Exit(1)
		}
		if !ok {
			fmt.Println("Cancelled")
			os.Exit(0)
		}
	}

	lockFile, lockPath, err := acquireLock()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer releaseLock(lockFile)
	
	fmt.Fprintf(os.Stderr, "Acquired execution lock: %s\n", lockPath)
	
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigc
		releaseLock(lockFile)
		os.Exit(1)
	}()

	var results executor.Results
	if *confirmEach {
		reader := bufio.NewReader(os.Stdin)
		for i, cmd := range p.Commands {
			fmt.Printf("\nExecute command %d: %s\n", i+1, executor.FormatCommand(cmd.Command))
			ok, err := ui.Confirm(reader, os.Stdout, "Proceed?")
			if err != nil || !ok {
				fmt.Println("Skipped")
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
				fmt.Fprintf(os.Stderr, "\n??  Command failed: %s\n", executor.FormatCommand(failedResult.Command))
				fmt.Fprintf(os.Stderr, "Error: %v\n", failedResult.Err)
				fmt.Fprintf(os.Stderr, "Output: %s\n", failedResult.Output)
				fmt.Fprintf(os.Stderr, "?? Attempting automatic fix (attempt %d/%d)...\n", retryAttempt, cfg.MaxRetries)
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
					fmt.Fprintf(os.Stderr, "Failed to generate fix: %v\n", err)
				}
				break
			}

			if len(fixPlan.Commands) == 0 {
				if !*jsonOutput {
					fmt.Fprintf(os.Stderr, "No fix commands generated\n")
				}
				break
			}

			// Validate fix plan
			if err := policyEngine.ValidatePlan(fixPlan); err != nil {
				if !*jsonOutput {
					fmt.Fprintf(os.Stderr, "Fix plan rejected by policy: %v\n", err)
				}
				break
			}

			if !*jsonOutput {
				fmt.Fprintf(os.Stderr, "\n?? Fix plan: %s\n", fixPlan.Summary)
				for _, cmd := range fixPlan.Commands {
					fmt.Fprintf(os.Stderr, "  ? %s\n", executor.FormatCommand(cmd.Command))
				}
			}

			// Execute fix
			fixResults := execEngine.RunPlan(ctx, fixPlan)
			
			// Mark original failure as retried by removing the error if fix succeeded
			if fixResults.Failed == 0 {
				if !*jsonOutput {
					fmt.Fprintf(os.Stderr, "? Fix successful!\n")
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
					fmt.Fprintf(os.Stderr, "? Fix attempt failed\n")
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
		if err := ui.PrintResultsJSON(os.Stdout, results); err != nil {
			fmt.Fprintf(os.Stderr, "JSON output error: %v\n", err)
			os.Exit(1)
		}
	} else {
		ui.PrintResults(os.Stdout, results)
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
		os.Exit(1)
	}
}
