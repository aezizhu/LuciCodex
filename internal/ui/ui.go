package ui

import (
    "bufio"
    "fmt"
    "io"
    "strings"

    "github.com/aezizhu/LuciCodex/internal/executor"
    "github.com/aezizhu/LuciCodex/internal/plan"
)

func PrintPlan(w io.Writer, p plan.Plan) {
    if p.Summary != "" {
        fmt.Fprintf(w, "Summary: %s\n\n", p.Summary)
    }
    for i, c := range p.Commands {
        fmt.Fprintf(w, "[%d] %s\n", i+1, executor.FormatCommand(c.Command))
        if strings.TrimSpace(c.Description) != "" {
            fmt.Fprintf(w, "    - %s\n", c.Description)
        }
    }
    if len(p.Warnings) > 0 {
        fmt.Fprintln(w, "\nWarnings:")
        for _, wmsg := range p.Warnings {
            fmt.Fprintf(w, "- %s\n", wmsg)
        }
    }
}

func Confirm(r *bufio.Reader, w io.Writer, msg string) (bool, error) {
    fmt.Fprintf(w, "%s [y/N]: ", msg)
    line, err := r.ReadString('\n')
    if err != nil {
        return false, err
    }
    line = strings.TrimSpace(strings.ToLower(line))
    return line == "y" || line == "yes", nil
}

type Results = executor.Results

func PrintResults(w io.Writer, res Results) {
    for _, item := range res.Items {
        status := "ok"
        if item.Err != nil {
            status = "error"
        }
        fmt.Fprintf(w, "[%d] (%s, %s) %s\n", item.Index+1, status, item.Elapsed, executor.FormatCommand(item.Command))
        if strings.TrimSpace(item.Output) != "" {
            fmt.Fprintln(w, indent(item.Output, 2))
        }
    }
    if res.Failed > 0 {
        fmt.Fprintf(w, "\n%d command(s) failed.\n", res.Failed)
    } else {
        fmt.Fprintln(w, "\nAll commands executed successfully.")
    }
}

func indent(s string, n int) string {
    pad := strings.Repeat(" ", n)
    lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
    for i := range lines {
        lines[i] = pad + lines[i]
    }
    return strings.Join(lines, "\n")
}


