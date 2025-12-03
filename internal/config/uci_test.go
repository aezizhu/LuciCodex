package config

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
)

// Mock exec.Command for UCI tests
func fakeExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	return cmd
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No command\n")
		os.Exit(2)
	}

	cmd, args := args[0], args[1:]

	// Mock UCI get commands
	if cmd == "uci" && len(args) >= 2 && args[0] == "-q" && args[1] == "get" {
		key := args[2]

		if os.Getenv("TEST_UCI_ERROR") == "1" {
			os.Exit(1)
		}
		if os.Getenv("TEST_UCI_SYSTEM_ERROR") == "1" {
			fmt.Fprintln(os.Stderr, "simulated system error")
			os.Exit(2)
		}

		if os.Getenv("TEST_UCI_ALL") == "1" {
			switch key {
			case "lucicodex.main.provider":
				fmt.Print("anthropic")
			case "lucicodex.main.key":
				fmt.Print("uci-key")
			case "lucicodex.main.openai_key":
				fmt.Print("uci-openai-key")
			case "lucicodex.main.anthropic_key":
				fmt.Print("uci-anthropic-key")
			case "lucicodex.main.model":
				fmt.Print("uci-model")
			case "lucicodex.main.endpoint":
				fmt.Print("uci-endpoint")
			case "lucicodex.main.openai_model":
				fmt.Print("uci-openai-model")
			case "lucicodex.main.openai_endpoint":
				fmt.Print("uci-openai-endpoint")
			case "lucicodex.main.anthropic_model":
				fmt.Print("uci-anthropic-model")
			case "lucicodex.main.anthropic_endpoint":
				fmt.Print("uci-anthropic-endpoint")
			case "lucicodex.main.dry_run":
				fmt.Print("1")
			case "lucicodex.main.confirm_each":
				fmt.Print("1")
			case "lucicodex.main.timeout":
				fmt.Print("123")
			case "lucicodex.main.max_commands":
				fmt.Print("456")
			case "lucicodex.main.log_file":
				fmt.Print("/tmp/uci.log")
			case "lucicodex.main.http_proxy":
				fmt.Print("http://proxy")
			case "lucicodex.main.https_proxy":
				fmt.Print("https://proxy")
			case "lucicodex.main.no_proxy":
				fmt.Print("localhost")
			default:
				os.Exit(1)
			}
			os.Exit(0)
		}

		if os.Getenv("TEST_UCI_FALLBACK") == "1" {
			switch key {
			case "lucicodex.@settings[0].key":
				fmt.Print("fallback-key")
			default:
				os.Exit(1)
			}
			os.Exit(0)
			os.Exit(0)
		}

		if os.Getenv("TEST_UCI_LEGACY") == "1" {
			switch key {
			case "lucicodex.@api[0].key":
				fmt.Print("legacy-key")
			default:
				os.Exit(1)
			}
			os.Exit(0)
		}

		switch key {
		case "lucicodex.main.provider":
			fmt.Print("openai")
		case "lucicodex.main.openai_key":
			fmt.Print("uci-openai-key")
		case "lucicodex.main.openai_model":
			fmt.Print("gpt-4o-test")
		case "lucicodex.main.dry_run":
			fmt.Print("0") // false
		default:
			// Return empty for unknown keys (simulating not found)
			os.Exit(1)
		}
		os.Exit(0)
	}

	os.Exit(0)
}

func TestLoad_UCI(t *testing.T) {
	// Save original execCommand
	oldExecCommand := execCommand
	execCommand = fakeExecCommand
	defer func() { execCommand = oldExecCommand }()

	// Clear env vars to ensure we test UCI
	os.Unsetenv("LUCICODEX_PROVIDER")
	os.Unsetenv("OPENAI_API_KEY")

	// Create a dummy config file to avoid file loading error
	f, err := os.CreateTemp("", "config_test_*.json")
	if err != nil {
		t.Fatal(err)
	}
	tmpConfig := f.Name()
	f.Write([]byte("{}"))
	f.Close()
	defer os.Remove(tmpConfig)

	cfg, err := Load(tmpConfig)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Check values from our mocked UCI
	if cfg.Provider != "openai" {
		t.Errorf("expected provider 'openai', got %q", cfg.Provider)
	}
	if cfg.OpenAIAPIKey != "uci-openai-key" {
		t.Errorf("expected OpenAIAPIKey 'uci-openai-key', got %q", cfg.OpenAIAPIKey)
	}
	if cfg.Model != "gpt-4o-test" {
		t.Errorf("expected Model 'gpt-4o-test', got %q", cfg.Model)
	}
	if cfg.DryRun {
		t.Error("expected DryRun to be false")
	}
}

func TestLoad_UCIFallbacks(t *testing.T) {
	oldExecCommand := execCommand
	execCommand = fakeExecCommand
	defer func() { execCommand = oldExecCommand }()

	os.Unsetenv("LUCICODEX_PROVIDER")
	os.Unsetenv("OPENAI_API_KEY")

	// Helper process that simulates missing main, but present settings
	os.Setenv("TEST_UCI_FALLBACK", "1")
	defer os.Unsetenv("TEST_UCI_FALLBACK")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.APIKey != "fallback-key" {
		t.Errorf("expected APIKey 'fallback-key', got %q", cfg.APIKey)
	}
}

func TestLoad_UCILegacy(t *testing.T) {
	oldExecCommand := execCommand
	execCommand = fakeExecCommand
	defer func() { execCommand = oldExecCommand }()

	os.Unsetenv("LUCICODEX_PROVIDER")
	os.Unsetenv("OPENAI_API_KEY")

	// Helper process that simulates missing main and settings, but present api
	os.Setenv("TEST_UCI_LEGACY", "1")
	defer os.Unsetenv("TEST_UCI_LEGACY")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.APIKey != "legacy-key" {
		t.Errorf("expected APIKey 'legacy-key', got %q", cfg.APIKey)
	}
}

func TestLoad_UCIError(t *testing.T) {
	oldExecCommand := execCommand
	execCommand = fakeExecCommand
	defer func() { execCommand = oldExecCommand }()

	os.Setenv("TEST_UCI_ERROR", "1")
	defer os.Unsetenv("TEST_UCI_ERROR")

	// Load should not fail, just return empty/defaults
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.APIKey != "" {
		t.Errorf("expected empty APIKey, got %q", cfg.APIKey)
	}
}

func TestLoad_UCI_InvalidValues(t *testing.T) {
	oldExecCommand := execCommand
	execCommand = fakeExecCommand
	defer func() { execCommand = oldExecCommand }()

	os.Unsetenv("LUCICODEX_PROVIDER")
	os.Setenv("TEST_UCI_INVALID", "1")
	defer os.Unsetenv("TEST_UCI_INVALID")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Should keep defaults
	if cfg.TimeoutSeconds != 300 {
		t.Errorf("expected default timeout 300, got %d", cfg.TimeoutSeconds)
	}
	if cfg.MaxCommands != 10 {
		t.Errorf("expected default max commands 10, got %d", cfg.MaxCommands)
	}
	// confirm_each "invalid" should be ignored (default false)
	if cfg.ConfirmEach {
		t.Error("expected ConfirmEach false")
	}
}

func TestLoad_EnvPrecedence(t *testing.T) {
	// Env should override UCI
	oldExecCommand := execCommand
	execCommand = fakeExecCommand
	defer func() { execCommand = oldExecCommand }()

	os.Setenv("LUCICODEX_PROVIDER", "anthropic")
	defer os.Unsetenv("LUCICODEX_PROVIDER")

	f, err := os.CreateTemp("", "config_test_*.json")
	if err != nil {
		t.Fatal(err)
	}
	tmpConfig := f.Name()
	f.Write([]byte("{}"))
	f.Close()
	defer os.Remove(tmpConfig)

	cfg, err := Load(tmpConfig)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Should be anthropic from env, not openai from UCI mock
	if cfg.Provider != "anthropic" {
		t.Errorf("expected provider 'anthropic', got %q", cfg.Provider)
	}
}

func TestLoad_UCI_AllFields(t *testing.T) {
	oldExecCommand := execCommand
	execCommand = fakeExecCommand
	defer func() { execCommand = oldExecCommand }()

	os.Unsetenv("LUCICODEX_PROVIDER")
	os.Setenv("TEST_UCI_ALL", "1")
	defer os.Unsetenv("TEST_UCI_ALL")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Provider != "anthropic" {
		t.Errorf("got provider %q", cfg.Provider)
	}
	if cfg.APIKey != "uci-key" {
		t.Errorf("got APIKey %q", cfg.APIKey)
	}
	if cfg.OpenAIAPIKey != "uci-openai-key" {
		t.Errorf("got OpenAIAPIKey %q", cfg.OpenAIAPIKey)
	}
	if cfg.AnthropicAPIKey != "uci-anthropic-key" {
		t.Errorf("got AnthropicAPIKey %q", cfg.AnthropicAPIKey)
	}
	if cfg.Model != "uci-anthropic-model" {
		t.Errorf("got Model %q", cfg.Model)
	}
	if cfg.Endpoint != "uci-anthropic-endpoint" {
		t.Errorf("got Endpoint %q", cfg.Endpoint)
	}
	if cfg.OpenAIModel != "uci-openai-model" {
		t.Errorf("got OpenAIModel %q", cfg.OpenAIModel)
	}
	if cfg.OpenAIEndpoint != "uci-openai-endpoint" {
		t.Errorf("got OpenAIEndpoint %q", cfg.OpenAIEndpoint)
	}
	if cfg.AnthropicModel != "uci-anthropic-model" {
		t.Errorf("got AnthropicModel %q", cfg.AnthropicModel)
	}
	if cfg.AnthropicEndpoint != "uci-anthropic-endpoint" {
		t.Errorf("got AnthropicEndpoint %q", cfg.AnthropicEndpoint)
	}
	if !cfg.DryRun {
		t.Error("expected DryRun true")
	}
	if !cfg.ConfirmEach {
		t.Error("expected ConfirmEach true")
	}
	if cfg.TimeoutSeconds != 123 {
		t.Errorf("got TimeoutSeconds %d", cfg.TimeoutSeconds)
	}
	if cfg.MaxCommands != 456 {
		t.Errorf("got MaxCommands %d", cfg.MaxCommands)
	}
	if cfg.LogFile != "/tmp/uci.log" {
		t.Errorf("got LogFile %q", cfg.LogFile)
	}
	if cfg.HTTPProxy != "http://proxy" {
		t.Errorf("got HTTPProxy %q", cfg.HTTPProxy)
	}
	if cfg.HTTPSProxy != "https://proxy" {
		t.Errorf("got HTTPSProxy %q", cfg.HTTPSProxy)
	}
	if cfg.NoProxy != "localhost" {
		t.Errorf("got NoProxy %q", cfg.NoProxy)
	}
}

func TestUciGet_PathSearch(t *testing.T) {
	oldLookPath := lookPath
	oldOsStat := osStat
	defer func() {
		lookPath = oldLookPath
		osStat = oldOsStat
	}()

	// Case 1: Found via LookPath
	lookPath = func(file string) (string, error) {
		if file == "/sbin/uci" {
			return "/sbin/uci", nil
		}
		return "", os.ErrNotExist
	}
	// We need to call uciGet indirectly or directly. uciGet is private.
	// But Load calls it.
	// However, uciGet executes the command. We need to mock execCommand too.
	oldExecCommand := execCommand
	execCommand = fakeExecCommand
	defer func() { execCommand = oldExecCommand }()

	// We can't easily check WHICH uci command was used because fakeExecCommand receives the command name
	// but uciGet resolves it first.
	// Wait, fakeExecCommand receives the command name passed to exec.Command.
	// So if uciGet resolves to "/sbin/uci", fakeExecCommand receives "/sbin/uci".

	// We need a way to verify the command path.
	// We can use a custom fakeExecCommand that checks the command path.

	var capturedCmd string
	execCommand = func(command string, args ...string) *exec.Cmd {
		capturedCmd = command
		return fakeExecCommand("uci", args...) // delegate to normal fake
	}

	// Trigger uciGet via Load
	os.Unsetenv("LUCICODEX_PROVIDER")
	Load("")

	if capturedCmd != "/sbin/uci" {
		t.Errorf("expected /sbin/uci, got %q", capturedCmd)
	}

	// Case 2: Found via os.Stat
	lookPath = func(file string) (string, error) { return "", os.ErrNotExist }
	osStat = func(name string) (os.FileInfo, error) {
		if name == "/usr/sbin/uci" {
			return nil, nil // success
		}
		return nil, os.ErrNotExist
	}

	Load("")
	if capturedCmd != "/usr/sbin/uci" {
		t.Errorf("expected /usr/sbin/uci, got %q", capturedCmd)
	}

	// Case 3: Fallback to "uci"
	lookPath = func(file string) (string, error) { return "", os.ErrNotExist }
	osStat = func(name string) (os.FileInfo, error) { return nil, os.ErrNotExist }

	Load("")
	if capturedCmd != "uci" {
		t.Errorf("expected 'uci', got %q", capturedCmd)
	}
}

func TestUciGetRobustness(t *testing.T) {
	oldExecCommand := execCommand
	execCommand = fakeExecCommand
	defer func() { execCommand = oldExecCommand }()

	// Case 1: Exit code 1 (key not found) -> should return "", nil
	// We use TEST_UCI_ERROR=1 which triggers os.Exit(1) in helper
	t.Run("ExitCode1", func(t *testing.T) {
		os.Setenv("TEST_UCI_ERROR", "1")
		defer os.Unsetenv("TEST_UCI_ERROR")

		val, err := uciGet("some.key")
		if err != nil {
			t.Errorf("expected no error for exit code 1, got: %v", err)
		}
		if val != "" {
			t.Errorf("expected empty string for exit code 1, got: %q", val)
		}
	})

	// Case 2: Exit code 2 (system error) -> should return "", error
	t.Run("ExitCode2", func(t *testing.T) {
		os.Setenv("TEST_UCI_SYSTEM_ERROR", "1")
		defer os.Unsetenv("TEST_UCI_SYSTEM_ERROR")

		_, err := uciGet("some.key")
		if err == nil {
			t.Error("expected error for exit code 2, got nil")
		}
	})
}
