package ui

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/aezizhu/LuciCodex/internal/executor"
	"github.com/aezizhu/LuciCodex/internal/plan"
)

const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Bold   = "\033[1m"
)

func colorize(color, msg string) string {
	return color + msg + Reset
}

func PrintPlan(w io.Writer, p plan.Plan) {
	if p.Summary != "" {
		fmt.Fprintf(w, "%s %s\n\n", colorize(Blue+Bold, "Summary:"), p.Summary)
	}
	for i, c := range p.Commands {
		fmt.Fprintf(w, "%s %s\n", colorize(Bold, fmt.Sprintf("[%d]", i+1)), executor.FormatCommand(c.Command))
		if strings.TrimSpace(c.Description) != "" {
			fmt.Fprintf(w, "    %s %s\n", colorize(Blue, "→"), c.Description)
		}
	}
	if len(p.Warnings) > 0 {
		fmt.Fprintln(w, "\n"+colorize(Yellow+Bold, "Warnings:"))
		for _, wmsg := range p.Warnings {
			fmt.Fprintf(w, "%s %s\n", colorize(Yellow, "⚠"), wmsg)
		}
	}
}

func Confirm(r *bufio.Reader, w io.Writer, msg string) (bool, error) {
	fmt.Fprintf(w, "%s %s ", colorize(Bold, msg), colorize(Blue, "[y/N]:"))
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
		status := colorize(Green, "ok")
		if item.Err != nil {
			status = colorize(Red, "error")
		}
		fmt.Fprintf(w, "%s (%s, %s) %s\n", colorize(Bold, fmt.Sprintf("[%d]", item.Index+1)), status, item.Elapsed, executor.FormatCommand(item.Command))
		if strings.TrimSpace(item.Output) != "" {
			fmt.Fprintln(w, indent(item.Output, 2))
		}
		if item.Err != nil {
			fmt.Fprintf(w, "  %s %v\n", colorize(Red, "Error:"), item.Err)
		}
	}
	if res.Failed > 0 {
		fmt.Fprintf(w, "\n%s %d command(s) failed.\n", colorize(Red+Bold, "FAILED:"), res.Failed)
	} else {
		fmt.Fprintln(w, "\n"+colorize(Green+Bold, "All commands executed successfully."))
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
