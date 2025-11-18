package security

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aezizhu/LuciCodex/internal/config"
	"github.com/aezizhu/LuciCodex/internal/plan"
)

func TestSandbox_ValidateCommand(t *testing.T) {
	s := NewSandbox(config.Config{})

	testCases := []struct {
		name        string
		command     []string
		expectError bool
	}{
		{"Valid command", []string{"ls", "-l"}, false},
		{"Valid uci command", []string{"uci", "show", "network"}, false},
		{"Dangerous pattern &&", []string{"ls", "&&", "rm", "-rf"}, true},
		{"Dangerous pattern |", []string{"cat", "/etc/passwd", "|", "grep", "root"}, true},
		{"Dangerous pattern ../", []string{"cat", "../../../../etc/hosts"}, true},
		{"Dangerous pattern /dev/", []string{"cat", "/dev/zero"}, true},
		{"Dangerous pattern `", []string{"echo", "`reboot`"}, true},
		{"Empty command", []string{}, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := s.ValidateCommand(plan.PlannedCommand{Command: tc.command})
			if tc.expectError && err == nil {
				t.Error("expected an error, but got none")
			}
			if !tc.expectError && err != nil {
				t.Errorf("expected no error, but got: %v", err)
			}
		})
	}
}

func TestSandbox_ExecuteCommand(t *testing.T) {
	cfg := config.Config{TimeoutSeconds: 15}
	s := NewSandbox(cfg)

	pc := plan.PlannedCommand{Command: []string{"/bin/echo", "hello", "world"}}

	cmd, err := s.ExecuteCommand(context.Background(), pc)
	if err != nil {
		t.Fatalf("ExecuteCommand failed: %v", err)
	}

	if cmd.Path != "/bin/echo" {
		t.Errorf("expected command path '/bin/echo', got '%s'", cmd.Path)
	}
	if len(cmd.Args) != 3 || cmd.Args[0] != "/bin/echo" || cmd.Args[1] != "hello" || cmd.Args[2] != "world" {
		t.Errorf("unexpected command args: %v", cmd.Args)
	}
	if cmd.Dir != s.tmpDir {
		t.Errorf("expected working directory '%s', got '%s'", s.tmpDir, cmd.Dir)
	}
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setpgid {
		t.Error("expected Setpgid to be true in SysProcAttr")
	}
}

func TestSandbox_Cleanup(t *testing.T) {
	s := NewSandbox(config.Config{})
	s.tmpDir = t.TempDir() // Use a temporary directory for the test

	// Create test files
	oldFile := filepath.Join(s.tmpDir, "old_file.txt")
	newFile := filepath.Join(s.tmpDir, "new_file.txt")
	oldTime := time.Now().Add(-2 * time.Hour)

	if err := os.WriteFile(oldFile, []byte("old"), 0600); err != nil {
		t.Fatalf("Failed to create old file: %v", err)
	}
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatalf("Failed to change mod time of old file: %v", err)
	}

	if err := os.WriteFile(newFile, []byte("new"), 0600); err != nil {
		t.Fatalf("Failed to create new file: %v", err)
	}

	// Run cleanup
	if err := s.cleanup(); err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}

	// Verify results
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("expected old file to be deleted, but it still exists")
	}
	if _, err := os.Stat(newFile); os.IsNotExist(err) {
		t.Error("expected new file to exist, but it was deleted")
	}
}
