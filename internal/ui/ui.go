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

// Colorize applies ANSI color codes to a message (exported for use in main).
func Colorize(color, msg string) string {
	return color + msg + Reset
}

// PrintResponse displays a conversational response when no commands are proposed.
// This is used for greetings, questions, or informational requests.
func PrintResponse(w io.Writer, p plan.Plan) {
	if p.Summary != "" {
		fmt.Fprintf(w, "%s\n", p.Summary)
	} else {
		fmt.Fprintln(w, "I understand your request, but no specific commands are needed.")
	}
	if len(p.Warnings) > 0 {
		fmt.Fprintln(w)
		for _, wmsg := range p.Warnings {
			fmt.Fprintf(w, "%s %s\n", colorize(Yellow, "Note:"), wmsg)
		}
	}
}

func PrintPlan(w io.Writer, p plan.Plan) {
	if p.Summary != "" {
		fmt.Fprintf(w, "%s %s\n\n", colorize(Blue+Bold, "Summary:"), p.Summary)
	}
	if len(p.Commands) == 0 {
		fmt.Fprintln(w, colorize(Yellow, "No commands to execute."))
		return
	}
	fmt.Fprintln(w, colorize(Bold, "Proposed commands:"))
	for i, c := range p.Commands {
		fmt.Fprintf(w, "%s %s\n", colorize(Green, fmt.Sprintf("[%d]", i+1)), executor.FormatCommand(c.Command))
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

// PrintSummary prints only the final summary line (used after streaming output).
func PrintSummary(w io.Writer, res Results) {
	total := len(res.Items)
	if res.Failed > 0 {
		fmt.Fprintf(w, "\n%s %d of %d command(s) failed.\n", colorize(Red+Bold, "FAILED:"), res.Failed, total)
	} else if total > 0 {
		fmt.Fprintf(w, "\n%s All %d command(s) executed successfully.\n", colorize(Green+Bold, "✓"), total)
	}
}

// PrintAnswer displays the AI's answer to the user's question based on command output.
func PrintAnswer(w io.Writer, summary string, details []string) {
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s\n", colorize(Blue+Bold, "Answer:"))
	fmt.Fprintf(w, "%s\n", summary)
	if len(details) > 0 {
		fmt.Fprintln(w)
		for _, detail := range details {
			fmt.Fprintf(w, "  %s %s\n", colorize(Blue, "•"), detail)
		}
	}
}
