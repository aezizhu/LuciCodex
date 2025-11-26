package plugins

import (
	"context"
	"testing"
)

func TestNetworkPlugin_CanHandle(t *testing.T) {
	p := &NetworkPlugin{}
	testCases := []struct {
		prompt    string
		canHandle bool
	}{
		{"restart the wifi", true},
		{"show me the ip address", true},
		{"what is my dns", true},
		{"open a port", false},
		{"show me the logs", false},
	}

	for _, tc := range testCases {
		if p.CanHandle(tc.prompt) != tc.canHandle {
			t.Errorf("for prompt '%s', expected CanHandle to be %v", tc.prompt, tc.canHandle)
		}
	}
}

func TestNetworkPlugin_GeneratePlan(t *testing.T) {
	p := &NetworkPlugin{}
	plan, err := p.GeneratePlan(context.Background(), "restart wifi")
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}

	if len(plan.Commands) != 6 {
		t.Errorf("expected 6 commands for 'restart wifi', but got %d", len(plan.Commands))
	}
	if plan.Commands[0].Command[0] != "uci" {
		t.Errorf("expected first command to be 'uci', got '%s'", plan.Commands[0].Command[0])
	}
}

func TestFirewallPlugin_CanHandle(t *testing.T) {
	p := &FirewallPlugin{}
	testCases := []struct {
		prompt    string
		canHandle bool
	}{
		{"open port 8080", true},
		{"block an ip", true},
		{"show firewall rules", true},
		{"restart the wifi", false},
		{"show me the ip address", false},
	}

	for _, tc := range testCases {
		if p.CanHandle(tc.prompt) != tc.canHandle {
			t.Errorf("for prompt '%s', expected CanHandle to be %v", tc.prompt, tc.canHandle)
		}
	}
}

func TestFirewallPlugin_GeneratePlan(t *testing.T) {
	p := &FirewallPlugin{}
	plan, err := p.GeneratePlan(context.Background(), "open port 80")
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}

	if len(plan.Commands) != 8 {
		t.Errorf("expected 8 commands for 'open port 80', but got %d", len(plan.Commands))
	}
	if plan.Commands[0].Command[0] != "uci" {
		t.Errorf("expected first command to be 'uci', got '%s'", plan.Commands[0].Command[0])
	}
	// Check that the port was correctly identified
	foundPort := false
	for _, cmd := range plan.Commands {
		if cmd.Command[0] == "uci" && cmd.Command[1] == "set" && cmd.Command[2] == "firewall.@rule[-1].dest_port=80" {
			foundPort = true
			break
		}
	}
	if !foundPort {
		t.Error("did not find command to set destination port to 80")
	}
}

func TestNetworkPlugin_GeneratePlan_ShowInterface(t *testing.T) {
	p := &NetworkPlugin{}
	plan, err := p.GeneratePlan(context.Background(), "show interface")
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}

	if len(plan.Commands) != 2 {
		t.Errorf("expected 2 commands, got %d", len(plan.Commands))
	}
	if plan.Commands[0].Command[2] != "network" {
		t.Errorf("expected uci show network, got %v", plan.Commands[0].Command)
	}
}

func TestFirewallPlugin_GeneratePlan_Ports(t *testing.T) {
	p := &FirewallPlugin{}

	tests := []struct {
		prompt string
		port   string
	}{
		{"open port 22", "22"},
		{"open port 443", "443"},
		{"open ssh port", "22"}, // default
	}

	for _, tt := range tests {
		plan, err := p.GeneratePlan(context.Background(), tt.prompt)
		if err != nil {
			t.Errorf("GeneratePlan(%q) failed: %v", tt.prompt, err)
			continue
		}

		found := false
		expected := "firewall.@rule[-1].dest_port=" + tt.port
		for _, cmd := range plan.Commands {
			if len(cmd.Command) > 2 && cmd.Command[2] == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("GeneratePlan(%q) did not contain command with %q", tt.prompt, expected)
		}
	}
}
