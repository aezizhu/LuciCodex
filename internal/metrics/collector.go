package metrics

import (
    "encoding/json"
    "fmt"
    "os"
    "sync"
    "time"

    "github.com/aezizhu/LuciCodex/internal/plan"
)

// Metrics tracks usage statistics
type Metrics struct {
    mu sync.RWMutex
    
    // Counters
    TotalRequests    int64             `json:"total_requests"`
    TotalCommands    int64             `json:"total_commands"`
    SuccessfulRuns   int64             `json:"successful_runs"`
    FailedRuns       int64             `json:"failed_runs"`
    
    // Timing
    TotalDuration    time.Duration     `json:"total_duration_ns"`
    AverageDuration  time.Duration     `json:"average_duration_ns"`
    
    // Provider usage
    ProviderUsage    map[string]int64  `json:"provider_usage"`
    
    // Command patterns
    CommandPatterns  map[string]int64  `json:"command_patterns"`
    
    // Error tracking
    ErrorTypes       map[string]int64  `json:"error_types"`
    
    // Session info
    StartTime        time.Time         `json:"start_time"`
    LastRequestTime  time.Time         `json:"last_request_time"`
    
    // Recent activity (circular buffer)
    RecentRequests   []RequestMetric   `json:"recent_requests"`
    maxRecent        int
}

// RequestMetric tracks individual request statistics
type RequestMetric struct {
    Timestamp    time.Time     `json:"timestamp"`
    Provider     string        `json:"provider"`
    Prompt       string        `json:"prompt"`
    NumCommands  int           `json:"num_commands"`
    Duration     time.Duration `json:"duration_ns"`
    Success      bool          `json:"success"`
    Error        string        `json:"error,omitempty"`
}

// Collector manages metrics collection
type Collector struct {
    metrics  *Metrics
    filePath string
    saveInterval time.Duration
    stopChan chan struct{}
}

func NewCollector(filePath string) *Collector {
    c := &Collector{
        metrics: &Metrics{
            ProviderUsage:   make(map[string]int64),
            CommandPatterns: make(map[string]int64),
            ErrorTypes:      make(map[string]int64),
            RecentRequests:  make([]RequestMetric, 0, 100),
            maxRecent:       100,
            StartTime:       time.Now(),
        },
        filePath:     filePath,
        saveInterval: 5 * time.Minute,
        stopChan:     make(chan struct{}),
    }
    
    // Load existing metrics
    c.Load()
    
    // Start periodic saving
    go c.periodicSave()
    
    return c
}

func (c *Collector) RecordRequest(provider, prompt string, p plan.Plan, duration time.Duration, err error) {
    c.metrics.mu.Lock()
    defer c.metrics.mu.Unlock()
    
    // Update counters
    c.metrics.TotalRequests++
    c.metrics.TotalCommands += int64(len(p.Commands))
    c.metrics.LastRequestTime = time.Now()
    
    // Update provider usage
    c.metrics.ProviderUsage[provider]++
    
    // Update timing
    c.metrics.TotalDuration += duration
    c.metrics.AverageDuration = time.Duration(int64(c.metrics.TotalDuration) / c.metrics.TotalRequests)
    
    // Track success/failure
    success := err == nil
    if success {
        c.metrics.SuccessfulRuns++
    } else {
        c.metrics.FailedRuns++
        errorType := "unknown"
        if err != nil {
            errorType = fmt.Sprintf("%T", err)
        }
        c.metrics.ErrorTypes[errorType]++
    }
    
    // Track command patterns
    for _, cmd := range p.Commands {
        if len(cmd.Command) > 0 {
            pattern := cmd.Command[0] // First word of command
            c.metrics.CommandPatterns[pattern]++
        }
    }
    
    // Add to recent requests (circular buffer)
    req := RequestMetric{
        Timestamp:   time.Now(),
        Provider:    provider,
        Prompt:      truncateString(prompt, 100),
        NumCommands: len(p.Commands),
        Duration:    duration,
        Success:     success,
    }
    if err != nil {
        req.Error = err.Error()
    }
    
    c.addRecentRequest(req)
}

func (c *Collector) addRecentRequest(req RequestMetric) {
    if len(c.metrics.RecentRequests) >= c.metrics.maxRecent {
        // Shift left to remove oldest
        copy(c.metrics.RecentRequests, c.metrics.RecentRequests[1:])
        c.metrics.RecentRequests[len(c.metrics.RecentRequests)-1] = req
    } else {
        c.metrics.RecentRequests = append(c.metrics.RecentRequests, req)
    }
}

func (c *Collector) snapshot() *Metrics {
    c.metrics.mu.RLock()
    defer c.metrics.mu.RUnlock()
    
    out := &Metrics{
        TotalRequests:   c.metrics.TotalRequests,
        TotalCommands:   c.metrics.TotalCommands,
        SuccessfulRuns:  c.metrics.SuccessfulRuns,
        FailedRuns:      c.metrics.FailedRuns,
        TotalDuration:   c.metrics.TotalDuration,
        AverageDuration: c.metrics.AverageDuration,
        StartTime:       c.metrics.StartTime,
        LastRequestTime: c.metrics.LastRequestTime,
        maxRecent:       c.metrics.maxRecent,
    }
    
    out.ProviderUsage = make(map[string]int64, len(c.metrics.ProviderUsage))
    out.CommandPatterns = make(map[string]int64, len(c.metrics.CommandPatterns))
    out.ErrorTypes = make(map[string]int64, len(c.metrics.ErrorTypes))
    
    for k, v := range c.metrics.ProviderUsage {
        out.ProviderUsage[k] = v
    }
    for k, v := range c.metrics.CommandPatterns {
        out.CommandPatterns[k] = v
    }
    for k, v := range c.metrics.ErrorTypes {
        out.ErrorTypes[k] = v
    }
    
    out.RecentRequests = append([]RequestMetric(nil), c.metrics.RecentRequests...)
    
    return out
}

func (c *Collector) GetMetrics() *Metrics {
    return c.snapshot()
}

func (c *Collector) Save() error {
    m := c.snapshot()
    data, err := json.MarshalIndent(m, "", "  ")
    if err != nil {
        return err
    }
    
    return os.WriteFile(c.filePath, data, 0o600)
}

func (c *Collector) Load() error {
    data, err := os.ReadFile(c.filePath)
    if err != nil {
        if os.IsNotExist(err) {
            return nil // No existing metrics file
        }
        return err
    }
    
    c.metrics.mu.Lock()
    defer c.metrics.mu.Unlock()
    
    return json.Unmarshal(data, c.metrics)
}

func (c *Collector) periodicSave() {
    ticker := time.NewTicker(c.saveInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            _ = c.Save()
        case <-c.stopChan:
            _ = c.Save() // Final save
            return
        }
    }
}

func (c *Collector) Stop() {
    close(c.stopChan)
}

func (c *Collector) GetSummary() Summary {
    m := c.GetMetrics()
    
    return Summary{
        TotalRequests:   m.TotalRequests,
        SuccessRate:     float64(m.SuccessfulRuns) / float64(m.TotalRequests) * 100,
        AverageDuration: m.AverageDuration,
        TopProvider:     getTopKey(m.ProviderUsage),
        TopCommand:      getTopKey(m.CommandPatterns),
        Uptime:          time.Since(m.StartTime),
    }
}

// Summary provides a quick overview
type Summary struct {
    TotalRequests   int64         `json:"total_requests"`
    SuccessRate     float64       `json:"success_rate"`
    AverageDuration time.Duration `json:"average_duration"`
    TopProvider     string        `json:"top_provider"`
    TopCommand      string        `json:"top_command"`
    Uptime          time.Duration `json:"uptime"`
}

func getTopKey(m map[string]int64) string {
    var topKey string
    var topCount int64
    
    for k, v := range m {
        if v > topCount {
            topCount = v
            topKey = k
        }
    }
    
    return topKey
}

func truncateString(s string, maxLen int) string {
    if len(s) <= maxLen {
        return s
    }
    return s[:maxLen-3] + "..."
}
