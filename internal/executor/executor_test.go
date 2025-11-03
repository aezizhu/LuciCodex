package executor

import "testing"

func TestFormatCommand(t *testing.T) {
    got := FormatCommand([]string{"echo", "hello world", "a&b"})
    if got == "" {
        t.Fatalf("empty")
    }
    if got == "echo hello world a&b" {
        t.Fatalf("expected quoting, got %q", got)
    }
}


