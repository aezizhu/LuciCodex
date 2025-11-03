package security

import (
    "context"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "syscall"
    "time"

    "github.com/aezizhu/LuciCodex/internal/config"
    "github.com/aezizhu/LuciCodex/internal/plan"
)

// ResourceLimits defines execution constraints
type ResourceLimits struct {
    MaxMemoryMB     int           // Maximum memory in MB
    MaxCPUPercent   int           // Maximum CPU percentage
    MaxExecutionTime time.Duration // Maximum execution time
    MaxFileSize     int64         // Maximum file size in bytes
    MaxProcesses    int           // Maximum number of processes
}

// Sandbox provides isolated command execution
type Sandbox struct {
    cfg    config.Config
    limits ResourceLimits
    tmpDir string
}

func NewSandbox(cfg config.Config) *Sandbox {
    return &Sandbox{
        cfg: cfg,
        limits: ResourceLimits{
            MaxMemoryMB:     128,
            MaxCPUPercent:   50,
            MaxExecutionTime: time.Duration(cfg.TimeoutSeconds) * time.Second,
            MaxFileSize:     10 * 1024 * 1024, // 10MB
            MaxProcesses:    10,
        },
        tmpDir: "/tmp/lucicodex-sandbox",
    }
}

func (s *Sandbox) SetLimits(limits ResourceLimits) {
    s.limits = limits
}

func (s *Sandbox) ExecuteCommand(ctx context.Context, pc plan.PlannedCommand) (*exec.Cmd, error) {
    if len(pc.Command) == 0 {
        return nil, fmt.Errorf("empty command")
    }

    // Create isolated environment
    if err := s.setupEnvironment(); err != nil {
        return nil, fmt.Errorf("setup environment: %w", err)
    }

    // Apply timeout from context or limits
    timeout := s.limits.MaxExecutionTime
    if timeout <= 0 {
        timeout = 30 * time.Second
    }
    
    cmdCtx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()

    // Prepare command with resource limits
    cmd := exec.CommandContext(cmdCtx, pc.Command[0], pc.Command[1:]...)
    
    // Set restricted environment
    cmd.Env = s.getRestrictedEnv()
    
    // Set working directory to sandbox
    cmd.Dir = s.tmpDir
    
    // Apply process limits using SysProcAttr
    cmd.SysProcAttr = &syscall.SysProcAttr{
        // Create new process group for better isolation
        Setpgid: true,
        // Additional security attributes would go here
        // Note: Full sandboxing requires more complex setup
    }

    return cmd, nil
}

func (s *Sandbox) setupEnvironment() error {
    // Create sandbox directory
    if err := os.MkdirAll(s.tmpDir, 0o700); err != nil {
        return err
    }

    // Clean up old files
    if err := s.cleanup(); err != nil {
        return err
    }

    return nil
}

func (s *Sandbox) cleanup() error {
    // Remove files older than 1 hour
    entries, err := os.ReadDir(s.tmpDir)
    if err != nil {
        return err
    }

    cutoff := time.Now().Add(-1 * time.Hour)
    for _, entry := range entries {
        info, err := entry.Info()
        if err != nil {
            continue
        }
        
        if info.ModTime().Before(cutoff) {
            path := filepath.Join(s.tmpDir, entry.Name())
            _ = os.RemoveAll(path)
        }
    }

    return nil
}

func (s *Sandbox) getRestrictedEnv() []string {
    // Minimal environment
    return []string{
        "PATH=/usr/sbin:/usr/bin:/sbin:/bin",
        "HOME=" + s.tmpDir,
        "TMPDIR=" + s.tmpDir,
        "USER=nobody",
        "SHELL=/bin/sh",
    }
}

// ValidateCommand performs additional security checks
func (s *Sandbox) ValidateCommand(pc plan.PlannedCommand) error {
    if len(pc.Command) == 0 {
        return fmt.Errorf("empty command")
    }

    // Check for dangerous patterns
    dangerous := []string{
        "/dev/",
        "/proc/",
        "/sys/",
        "../",
        "&&",
        "||",
        "|",
        ">",
        "<",
        "$(",
        "`",
    }

    cmdline := fmt.Sprintf("%v", pc.Command)
    for _, pattern := range dangerous {
        if contains(cmdline, pattern) {
            return fmt.Errorf("command contains dangerous pattern: %s", pattern)
        }
    }

    return nil
}

func contains(s, substr string) bool {
    return len(s) >= len(substr) && 
           (s == substr || 
            (len(s) > len(substr) && 
             (s[:len(substr)] == substr || 
              s[len(s)-len(substr):] == substr ||
              containsMiddle(s, substr))))
}

func containsMiddle(s, substr string) bool {
    for i := 0; i <= len(s)-len(substr); i++ {
        if s[i:i+len(substr)] == substr {
            return true
        }
    }
    return false
}

// Monitor tracks resource usage during execution
type Monitor struct {
    cmd       *exec.Cmd
    limits    ResourceLimits
    startTime time.Time
}

func NewMonitor(cmd *exec.Cmd, limits ResourceLimits) *Monitor {
    return &Monitor{
        cmd:       cmd,
        limits:    limits,
        startTime: time.Now(),
    }
}

func (m *Monitor) Start(ctx context.Context) error {
    // Start monitoring in background
    go m.monitorResources(ctx)
    return m.cmd.Start()
}

func (m *Monitor) Wait() error {
    return m.cmd.Wait()
}

func (m *Monitor) monitorResources(ctx context.Context) {
    ticker := time.NewTicker(100 * time.Millisecond)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if m.cmd.Process == nil {
                return
            }

            // Check execution time
            if time.Since(m.startTime) > m.limits.MaxExecutionTime {
                _ = m.cmd.Process.Kill()
                return
            }

            // Additional resource checks would go here
            // (memory usage, CPU usage, etc.)
        }
    }
}
