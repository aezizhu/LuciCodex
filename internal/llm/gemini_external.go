package llm

import (
    "bytes"
    "context"
    "errors"
    "os/exec"
    "strings"
    "time"

    "github.com/aezizhu/LuciCodex/internal/config"
    "github.com/aezizhu/LuciCodex/internal/plan"
)

type ExternalGeminiClient struct {
    cfg config.Config
}

func NewExternalGeminiClient(cfg config.Config) *ExternalGeminiClient { return &ExternalGeminiClient{cfg: cfg} }

func (c *ExternalGeminiClient) GeneratePlan(ctx context.Context, prompt string) (plan.Plan, error) {
    var zero plan.Plan
    path := c.cfg.ExternalGeminiPath
    if strings.TrimSpace(path) == "" {
        path = "/usr/bin/gemini"
    }
    argv := []string{path, prompt}
    cctx, cancel := context.WithTimeout(ctx, 45*time.Second)
    defer cancel()
    cmd := exec.CommandContext(cctx, argv[0], argv[1:]...)
    var out bytes.Buffer
    cmd.Stdout = &out
    cmd.Stderr = &out
    _ = cmd.Run()
    text := out.String()
    // Try parse JSON plan from output
    p, err := plan.TryUnmarshalPlan(text)
    if err == nil && len(p.Commands) > 0 { return p, nil }
    // try extract
    p2, err2 := plan.TryUnmarshalPlan(extractJSON(text))
    if err2 == nil && len(p2.Commands) > 0 { return p2, nil }
    return zero, errors.New("external gemini did not return a valid plan")
}

func (c *ExternalGeminiClient) GenerateErrorFix(ctx context.Context, originalCommand string, errorOutput string, attempt int) (plan.Plan, error) {
    prompt := "You are a router command error fixer for OpenWrt systems.\n\nThe following command failed:\nCommand: " + originalCommand + "\nError output: " + errorOutput + "\n\nAnalyze the error and provide a corrected plan to fix the issue. Output strict JSON with commands array."
    return c.GeneratePlan(ctx, prompt)
}


