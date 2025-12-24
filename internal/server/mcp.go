package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aezizhu/LuciCodex/internal/executor"
	"github.com/aezizhu/LuciCodex/internal/openwrt"
	"github.com/aezizhu/LuciCodex/internal/plan"
	"github.com/aezizhu/LuciCodex/internal/policy"
)

// MCP (Model Context Protocol) implementation
// See: https://modelcontextprotocol.io/

// MCPRequest represents a JSON-RPC 2.0 request
type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// MCPResponse represents a JSON-RPC 2.0 response
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError represents a JSON-RPC 2.0 error
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MCP error codes
const (
	MCPParseError     = -32700
	MCPInvalidRequest = -32600
	MCPMethodNotFound = -32601
	MCPInvalidParams  = -32602
	MCPInternalError  = -32603
)

// MCPServerInfo represents server information
type MCPServerInfo struct {
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Capabilities []string `json:"capabilities"`
}

// MCPTool represents a tool definition
type MCPTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// MCPResource represents a resource definition
type MCPResource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// handleMCP handles MCP JSON-RPC requests
func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req MCPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendMCPError(w, nil, MCPParseError, "Parse error", nil)
		return
	}

	if req.JSONRPC != "2.0" {
		sendMCPError(w, req.ID, MCPInvalidRequest, "Invalid JSON-RPC version", nil)
		return
	}

	// Route to appropriate handler
	var result interface{}
	var mcpErr *MCPError

	switch req.Method {
	case "initialize":
		result, mcpErr = s.mcpInitialize(req.Params)
	case "tools/list":
		result, mcpErr = s.mcpListTools()
	case "tools/call":
		result, mcpErr = s.mcpCallTool(r.Context(), req.Params)
	case "resources/list":
		result, mcpErr = s.mcpListResources()
	case "resources/read":
		result, mcpErr = s.mcpReadResource(req.Params)
	case "ping":
		result = map[string]string{"status": "ok"}
	default:
		mcpErr = &MCPError{Code: MCPMethodNotFound, Message: "Method not found: " + req.Method}
	}

	if mcpErr != nil {
		sendMCPError(w, req.ID, mcpErr.Code, mcpErr.Message, mcpErr.Data)
		return
	}

	sendMCPResponse(w, req.ID, result)
}

// mcpInitialize handles the initialize request
func (s *Server) mcpInitialize(params json.RawMessage) (interface{}, *MCPError) {
	return map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"serverInfo": MCPServerInfo{
			Name:    "lucicodex",
			Version: "1.0.0",
			Capabilities: []string{
				"tools",
				"resources",
			},
		},
		"capabilities": map[string]interface{}{
			"tools":     map[string]bool{"listChanged": false},
			"resources": map[string]bool{"subscribe": false, "listChanged": false},
		},
	}, nil
}

// mcpListTools returns available tools
func (s *Server) mcpListTools() (interface{}, *MCPError) {
	tools := []MCPTool{
		{
			Name:        "uci_get",
			Description: "Read UCI configuration value",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"config":  map[string]string{"type": "string", "description": "Config file name (e.g., network, wireless)"},
					"section": map[string]string{"type": "string", "description": "Section name"},
					"option":  map[string]string{"type": "string", "description": "Option name (optional)"},
				},
				"required": []string{"config", "section"},
			},
		},
		{
			Name:        "uci_set",
			Description: "Set UCI configuration value (requires approval)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"config":  map[string]string{"type": "string", "description": "Config file name"},
					"section": map[string]string{"type": "string", "description": "Section name"},
					"option":  map[string]string{"type": "string", "description": "Option name"},
					"value":   map[string]string{"type": "string", "description": "Value to set"},
				},
				"required": []string{"config", "section", "option", "value"},
			},
		},
		{
			Name:        "uci_commit",
			Description: "Commit UCI changes and optionally reload services",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"config": map[string]string{"type": "string", "description": "Config file to commit"},
					"reload": map[string]string{"type": "boolean", "description": "Whether to reload the service"},
				},
				"required": []string{"config"},
			},
		},
		{
			Name:        "exec",
			Description: "Execute a command (validated against policy)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "array",
						"items":       map[string]string{"type": "string"},
						"description": "Command as array of arguments",
					},
					"description": map[string]string{"type": "string", "description": "Description of what the command does"},
				},
				"required": []string{"command"},
			},
		},
		{
			Name:        "diagnostics",
			Description: "Run network diagnostics",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"type":   map[string]string{"type": "string", "description": "Diagnostic type: ping, traceroute, nslookup, ifconfig"},
					"target": map[string]string{"type": "string", "description": "Target host or interface (optional)"},
				},
				"required": []string{"type"},
			},
		},
		{
			Name:        "facts",
			Description: "Collect system facts (hostname, interfaces, etc.)",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}

	return map[string]interface{}{"tools": tools}, nil
}

// mcpCallTool executes a tool
func (s *Server) mcpCallTool(ctx context.Context, params json.RawMessage) (interface{}, *MCPError) {
	var req struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &MCPError{Code: MCPInvalidParams, Message: "Invalid params"}
	}

	switch req.Name {
	case "uci_get":
		return s.toolUCIGet(req.Arguments)
	case "uci_set":
		return s.toolUCISet(ctx, req.Arguments)
	case "uci_commit":
		return s.toolUCICommit(ctx, req.Arguments)
	case "exec":
		return s.toolExec(ctx, req.Arguments)
	case "diagnostics":
		return s.toolDiagnostics(ctx, req.Arguments)
	case "facts":
		return s.toolFacts(ctx)
	default:
		return nil, &MCPError{Code: MCPMethodNotFound, Message: "Unknown tool: " + req.Name}
	}
}

// toolUCIGet reads a UCI value
func (s *Server) toolUCIGet(args json.RawMessage) (interface{}, *MCPError) {
	var params struct {
		Config  string `json:"config"`
		Section string `json:"section"`
		Option  string `json:"option"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, &MCPError{Code: MCPInvalidParams, Message: err.Error()}
	}

	// Build UCI path
	path := params.Config + "." + params.Section
	if params.Option != "" {
		path += "." + params.Option
	}

	// Execute uci get
	output, err := executor.DefaultRunCommand(context.Background(), []string{"uci", "get", path})
	if err != nil {
		return map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": "Error: " + err.Error()}},
			"isError": true,
		}, nil
	}

	return map[string]interface{}{
		"content": []map[string]string{{"type": "text", "text": strings.TrimSpace(output)}},
	}, nil
}

// toolUCISet sets a UCI value (dry-run only, returns command for approval)
func (s *Server) toolUCISet(ctx context.Context, args json.RawMessage) (interface{}, *MCPError) {
	var params struct {
		Config  string `json:"config"`
		Section string `json:"section"`
		Option  string `json:"option"`
		Value   string `json:"value"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, &MCPError{Code: MCPInvalidParams, Message: err.Error()}
	}

	path := params.Config + "." + params.Section + "." + params.Option
	cmd := []string{"uci", "set", path + "=" + params.Value}

	// Return the command for approval (dry-run mode)
	return map[string]interface{}{
		"content": []map[string]string{
			{"type": "text", "text": fmt.Sprintf("Command prepared (requires approval): %s", executor.FormatCommand(cmd))},
		},
		"pendingCommand": cmd,
		"requiresApproval": true,
	}, nil
}

// toolUCICommit commits UCI changes
func (s *Server) toolUCICommit(ctx context.Context, args json.RawMessage) (interface{}, *MCPError) {
	var params struct {
		Config string `json:"config"`
		Reload bool   `json:"reload"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, &MCPError{Code: MCPInvalidParams, Message: err.Error()}
	}

	cmd := []string{"uci", "commit", params.Config}

	// Return for approval
	result := map[string]interface{}{
		"content": []map[string]string{
			{"type": "text", "text": fmt.Sprintf("Commit command prepared (requires approval): %s", executor.FormatCommand(cmd))},
		},
		"pendingCommands": [][]string{cmd},
		"requiresApproval": true,
	}

	if params.Reload {
		reloadCmd := []string{"/etc/init.d/" + params.Config, "reload"}
		result["pendingCommands"] = [][]string{cmd, reloadCmd}
	}

	return result, nil
}

// toolExec executes a validated command
func (s *Server) toolExec(ctx context.Context, args json.RawMessage) (interface{}, *MCPError) {
	var params struct {
		Command     []string `json:"command"`
		Description string   `json:"description"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, &MCPError{Code: MCPInvalidParams, Message: err.Error()}
	}

	if len(params.Command) == 0 {
		return nil, &MCPError{Code: MCPInvalidParams, Message: "Empty command"}
	}

	// Validate against policy
	p := plan.Plan{
		Commands: []plan.PlannedCommand{{
			Command:     params.Command,
			Description: params.Description,
		}},
	}

	policyEngine := policy.New(s.cfg)
	if err := policyEngine.ValidatePlan(p); err != nil {
		return map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": "Policy violation: " + err.Error()}},
			"isError": true,
		}, nil
	}

	// Execute
	execEngine := executor.New(s.cfg)
	results := execEngine.RunPlan(ctx, p)

	if len(results.Items) == 0 {
		return map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": "No output"}},
		}, nil
	}

	r := results.Items[0]
	if r.Err != nil {
		return map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": r.Output + "\nError: " + r.Err.Error()}},
			"isError": true,
		}, nil
	}

	return map[string]interface{}{
		"content": []map[string]string{{"type": "text", "text": r.Output}},
	}, nil
}

// toolDiagnostics runs network diagnostics
func (s *Server) toolDiagnostics(ctx context.Context, args json.RawMessage) (interface{}, *MCPError) {
	var params struct {
		Type   string `json:"type"`
		Target string `json:"target"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, &MCPError{Code: MCPInvalidParams, Message: err.Error()}
	}

	var cmd []string
	switch params.Type {
	case "ping":
		target := params.Target
		if target == "" {
			target = "8.8.8.8"
		}
		cmd = []string{"ping", "-c", "4", target}
	case "traceroute":
		target := params.Target
		if target == "" {
			target = "8.8.8.8"
		}
		cmd = []string{"traceroute", "-m", "10", target}
	case "nslookup":
		target := params.Target
		if target == "" {
			target = "google.com"
		}
		cmd = []string{"nslookup", target}
	case "ifconfig":
		if params.Target != "" {
			cmd = []string{"ifconfig", params.Target}
		} else {
			cmd = []string{"ifconfig"}
		}
	default:
		return nil, &MCPError{Code: MCPInvalidParams, Message: "Unknown diagnostic type: " + params.Type}
	}

	output, err := executor.DefaultRunCommand(ctx, cmd)
	if err != nil {
		return map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": output + "\nError: " + err.Error()}},
			"isError": true,
		}, nil
	}

	return map[string]interface{}{
		"content": []map[string]string{{"type": "text", "text": output}},
	}, nil
}

// toolFacts collects system facts
func (s *Server) toolFacts(ctx context.Context) (interface{}, *MCPError) {
	factsCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	facts := openwrt.CollectFacts(factsCtx)
	return map[string]interface{}{
		"content": []map[string]string{{"type": "text", "text": facts}},
	}, nil
}

// mcpListResources returns available resources
func (s *Server) mcpListResources() (interface{}, *MCPError) {
	resources := []MCPResource{
		{
			URI:         "config://network",
			Name:        "Network Configuration",
			Description: "OpenWrt network configuration (sanitized)",
			MimeType:    "text/plain",
		},
		{
			URI:         "config://wireless",
			Name:        "Wireless Configuration",
			Description: "OpenWrt wireless configuration (sanitized, no passwords)",
			MimeType:    "text/plain",
		},
		{
			URI:         "config://firewall",
			Name:        "Firewall Configuration",
			Description: "OpenWrt firewall rules",
			MimeType:    "text/plain",
		},
		{
			URI:         "syslog://recent",
			Name:        "Recent System Logs",
			Description: "Last 50 lines of system log",
			MimeType:    "text/plain",
		},
	}

	return map[string]interface{}{"resources": resources}, nil
}

// mcpReadResource reads a resource
func (s *Server) mcpReadResource(params json.RawMessage) (interface{}, *MCPError) {
	var req struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &MCPError{Code: MCPInvalidParams, Message: err.Error()}
	}

	var content string
	var mimeType = "text/plain"

	switch {
	case strings.HasPrefix(req.URI, "config://"):
		configName := strings.TrimPrefix(req.URI, "config://")
		output, err := executor.DefaultRunCommand(context.Background(), []string{"uci", "export", configName})
		if err != nil {
			return nil, &MCPError{Code: MCPInternalError, Message: err.Error()}
		}
		// Sanitize sensitive data
		content = sanitizeConfig(output)

	case req.URI == "syslog://recent":
		output, err := executor.DefaultRunCommand(context.Background(), []string{"logread", "-l", "50"})
		if err != nil {
			// Try dmesg as fallback
			output, _ = executor.DefaultRunCommand(context.Background(), []string{"dmesg"})
		}
		content = output

	default:
		return nil, &MCPError{Code: MCPInvalidParams, Message: "Unknown resource: " + req.URI}
	}

	return map[string]interface{}{
		"contents": []map[string]string{
			{
				"uri":      req.URI,
				"mimeType": mimeType,
				"text":     content,
			},
		},
	}, nil
}

// sanitizeConfig removes sensitive data from configuration
func sanitizeConfig(config string) string {
	lines := strings.Split(config, "\n")
	var result []string

	sensitiveKeys := []string{"password", "key", "secret", "psk", "wpakey", "encryption_key"}

	for _, line := range lines {
		sanitized := line
		lineLower := strings.ToLower(line)

		for _, key := range sensitiveKeys {
			if strings.Contains(lineLower, "option "+key) || strings.Contains(lineLower, "option\t"+key) {
				// Replace value with redacted marker
				parts := strings.SplitN(line, "'", 3)
				if len(parts) >= 3 {
					sanitized = parts[0] + "'<REDACTED>'"
				}
				break
			}
		}

		result = append(result, sanitized)
	}

	return strings.Join(result, "\n")
}

// sendMCPResponse sends a successful MCP response
func sendMCPResponse(w http.ResponseWriter, id interface{}, result interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	})
}

// sendMCPError sends an MCP error response
func sendMCPError(w http.ResponseWriter, id interface{}, code int, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &MCPError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	})
}

// readFileContent reads a file safely
func readFileContent(path string, maxSize int64) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Limit read size
	limited := io.LimitReader(f, maxSize)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
