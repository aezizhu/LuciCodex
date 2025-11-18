package openwrt

import (
	"context"
	"strings"
	"testing"
)

func TestCollectFacts(t *testing.T) {
	// Keep a reference to the original runCommand function
	originalRunCommand := runCommand
	// At the end of the test, restore the original function
	defer func() { runCommand = originalRunCommand }()

	// Replace runCommand with a mock implementation
	runCommand = func(ctx context.Context, name string, args ...string) string {
		switch name {
		case "cat":
			return "PRETTY_NAME=\"Test OpenWrt\""
		case "uname":
			return "Linux test 5.10.0"
		case "ubus":
			return `{"system": {"board_name": "test-board"}}`
		case "uci":
			// Return different output based on args to test both calls
			if len(args) > 2 && args[2] == "network" {
				return "network.lan.proto='static'"
			}
			if len(args) > 2 && args[2] == "wireless" {
				return "wireless.radio0.disabled='0'"
			}
			return ""
		case "fw4":
			return "table inet fw4 {}"
		default:
			return ""
		}
	}

	// Run CollectFacts, which will now use the mock
	facts := CollectFacts(context.Background())

	// Verify the output
	if !strings.Contains(facts, "/etc/os-release:\nPRETTY_NAME=\"Test OpenWrt\"") {
		t.Errorf("facts do not contain expected os-release output. Got:\n%s", facts)
	}
	if !strings.Contains(facts, "uname -a:\nLinux test 5.10.0") {
		t.Errorf("facts do not contain expected uname output. Got:\n%s", facts)
	}
	if !strings.Contains(facts, "ubus system board:\n{\"system\": {\"board_name\": \"test-board\"}}") {
		t.Errorf("facts do not contain expected ubus output. Got:\n%s", facts)
	}
	if !strings.Contains(facts, "uci show network:\nnetwork.lan.proto='static'") {
		t.Errorf("facts do not contain expected uci network output. Got:\n%s", facts)
	}
	if !strings.Contains(facts, "uci show wireless:\nwireless.radio0.disabled='0'") {
		t.Errorf("facts do not contain expected uci wireless output. Got:\n%s", facts)
	}
	if !strings.Contains(facts, "fw4 print:\ntable inet fw4 {}") {
		t.Errorf("facts do not contain expected fw4 output. Got:\n%s", facts)
	}
}

func TestCollectFacts_Truncation(t *testing.T) {
	originalRunCommand := runCommand
	defer func() { runCommand = originalRunCommand }()

	longString := strings.Repeat("a", 5000)

	runCommand = func(ctx context.Context, name string, args ...string) string {
		if name == "cat" {
			return longString
		}
		// Return empty for other commands to isolate the test
		return ""
	}

	facts := CollectFacts(context.Background())

	// The logic in the source code truncates the output string `out` to 4096,
	// then prepends the header.
	expectedHeader := "/etc/os-release:\n"
	expectedOutput := expectedHeader + longString[:4096]
	if facts != expectedOutput {
		t.Errorf("expected facts to be truncated to length %d, but got length %d", len(expectedOutput), len(facts))
	}
}

func TestCollectFacts_EmptyOutput(t *testing.T) {
	originalRunCommand := runCommand
	defer func() { runCommand = originalRunCommand }()

	// Mock all commands to return empty strings
	runCommand = func(ctx context.Context, name string, args ...string) string {
		return ""
	}

	facts := CollectFacts(context.Background())

	if facts != "" {
		t.Errorf("expected facts to be empty when all commands fail, but got:\n%s", facts)
	}
}
