package logging

import (
    "encoding/json"
    "fmt"
    "os"
    "sync"
    "time"

    "github.com/aezizhu/LuciCodex/internal/plan"
)

type Logger struct {
    path string
    mu   sync.Mutex
}

func New(path string) *Logger { return &Logger{path: path} }

func (l *Logger) writeJSON(event string, data any) {
    if l.path == "" {
        return
    }
    l.mu.Lock()
    defer l.mu.Unlock()
    f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
    if err != nil {
        return
    }
    defer f.Close()
    entry := map[string]any{
        "ts":    time.Now().UTC().Format(time.RFC3339Nano),
        "event": event,
        "data":  data,
    }
    b, err := json.Marshal(entry)
    if err != nil {
        return
    }
    _, _ = fmt.Fprintln(f, string(b))
}

func (l *Logger) Plan(prompt string, p plan.Plan) {
    l.writeJSON("plan", map[string]any{"prompt": prompt, "plan": p})
}

type ResultItem struct {
    Index   int           `json:"index"`
    Command []string      `json:"command"`
    Output  string        `json:"output"`
    Error   string        `json:"error,omitempty"`
    Elapsed time.Duration `json:"elapsed"`
}

func (l *Logger) Results(items []ResultItem) {
    l.writeJSON("results", items)
}


