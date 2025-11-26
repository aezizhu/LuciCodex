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

func TestManager_LoadPlugins_Errors(t *testing.T) {
	originalExecuteCommand := executeCommand
	defer func() { executeCommand = originalExecuteCommand }()

	// Mock executeCommand to return invalid JSON for metadata
	executeCommand = func(ctx context.Context, path string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "--metadata" {
			return []byte("invalid json"), nil
		}
		return nil, nil
	}

	tempDir := t.TempDir()
	// Create a dummy executable file
	dummyPluginPath := filepath.Join(tempDir, "bad-plugin")
	if err := os.WriteFile(dummyPluginPath, []byte("#!/bin/sh"), 0755); err != nil {
		t.Fatalf("Failed to create dummy plugin file: %v", err)
	}

	manager := NewManager([]string{tempDir})
	// LoadPlugins should not return error, but skip the bad plugin
	if err := manager.LoadPlugins(); err != nil {
		t.Fatalf("LoadPlugins failed: %v", err)
	}

	if len(manager.ListPlugins()) != 0 {
		t.Error("expected 0 plugins loaded")
	}
}

func TestExternalPlugin_GeneratePlan_Errors(t *testing.T) {
	originalExecuteCommand := executeCommand
	defer func() { executeCommand = originalExecuteCommand }()

	plugin := &ExternalPlugin{path: "/test"}

	// Test execution failure
	executeCommand = func(ctx context.Context, path string, args ...string) ([]byte, error) {
		return nil, context.DeadlineExceeded
	}
	_, err := plugin.GeneratePlan(context.Background(), "test")
	if err == nil {
		t.Error("expected error for execution failure")
	}

	// Test invalid JSON
	executeCommand = func(ctx context.Context, path string, args ...string) ([]byte, error) {
		return []byte("bad json"), nil
	}
	_, err = plugin.GeneratePlan(context.Background(), "test")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestManager_FindPlugin(t *testing.T) {
	manager := NewManager(nil)
	// Add a mock plugin manually
	p := &ExternalPlugin{
		name:     "Test",
		keywords: []string{"test"},
	}
	manager.plugins = append(manager.plugins, p)

	if found := manager.FindPlugin("this is a test"); found != p {
		t.Error("expected to find plugin")
	}
	if found := manager.FindPlugin("nothing"); found != nil {
		t.Error("expected nil for no match")
	}
}

func TestDefaultExecute(t *testing.T) {
	// defaultExecute is the initial value of executeCommand
	// We can access it via the unexported function directly

	ctx := context.Background()
	out, err := defaultExecute(ctx, "echo", "hello")
	if err != nil {
		t.Fatalf("defaultExecute failed: %v", err)
	}
	if string(out) != "hello\n" { // echo usually adds newline
		t.Errorf("expected 'hello\\n', got %q", string(out))
	}
}

func TestGetBuiltinPlugins(t *testing.T) {
	plugins := GetBuiltinPlugins()
	if len(plugins) < 2 {
		t.Errorf("expected at least 2 builtin plugins, got %d", len(plugins))
	}
}

func TestPluginDescriptions(t *testing.T) {
	p1 := &NetworkPlugin{}
	if p1.Name() == "" || p1.Description() == "" {
		t.Error("NetworkPlugin missing name or description")
	}

	p2 := &FirewallPlugin{}
	if p2.Name() == "" || p2.Description() == "" {
		t.Error("FirewallPlugin missing name or description")
	}

	p3 := &ExternalPlugin{name: "ext", description: "desc"}
	if p3.Name() != "ext" || p3.Description() != "desc" {
		t.Error("ExternalPlugin name/description mismatch")
	}
}
