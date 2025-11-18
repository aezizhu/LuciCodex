package plugins

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/aezizhu/LuciCodex/internal/plan"
)

func TestManager_LoadPlugins(t *testing.T) {
	// Keep a reference to the original function and restore it later
	originalExecuteCommand := executeCommand
	defer func() { executeCommand = originalExecuteCommand }()

	// Mock the executeCommand function
	executeCommand = func(ctx context.Context, path string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "--metadata" {
			metadata := PluginMetadata{
				Name:        "Test Plugin",
				Description: "A plugin for testing",
				Keywords:    []string{"test", "example"},
			}
			return json.Marshal(metadata)
		}
		return nil, nil
	}

	tempDir := t.TempDir()
	// Create a dummy executable file
	dummyPluginPath := filepath.Join(tempDir, "test-plugin")
	if err := os.WriteFile(dummyPluginPath, []byte("#!/bin/sh"), 0755); err != nil {
		t.Fatalf("Failed to create dummy plugin file: %v", err)
	}

	// Create a manager and load plugins from the temp directory
	manager := NewManager([]string{tempDir})
	if err := manager.LoadPlugins(); err != nil {
		t.Fatalf("LoadPlugins failed: %v", err)
	}

	plugins := manager.ListPlugins()
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin to be loaded, but got %d", len(plugins))
	}

	if plugins[0].Name() != "Test Plugin" {
		t.Errorf("expected plugin name 'Test Plugin', got '%s'", plugins[0].Name())
	}
	if !plugins[0].CanHandle("this is a test prompt") {
		t.Error("expected plugin to handle prompt with 'test' keyword")
	}
}

func TestExternalPlugin_GeneratePlan(t *testing.T) {
	originalExecuteCommand := executeCommand
	defer func() { executeCommand = originalExecuteCommand }()

	// Mock the executeCommand function to return a plan
	executeCommand = func(ctx context.Context, path string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "--plan" {
			p := plan.Plan{
				Summary: "A test plan",
				Commands: []plan.PlannedCommand{
					{Command: []string{"echo", "hello from plugin"}},
				},
			}
			return json.Marshal(p)
		}
		return nil, nil
	}

	plugin := &ExternalPlugin{
		path: "/fake/path/to/plugin",
	}

	plan, err := plugin.GeneratePlan(context.Background(), "test prompt")
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}

	if plan.Summary != "A test plan" {
		t.Errorf("expected plan summary 'A test plan', got '%s'", plan.Summary)
	}
	if len(plan.Commands) != 1 || plan.Commands[0].Command[1] != "hello from plugin" {
		t.Error("plan commands are not as expected")
	}
}
