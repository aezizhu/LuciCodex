package server

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/aezizhu/LuciCodex/internal/config"
	"github.com/aezizhu/LuciCodex/internal/executor"
	"github.com/aezizhu/LuciCodex/internal/llm"
	"github.com/aezizhu/LuciCodex/internal/llm/prompts"
	"github.com/aezizhu/LuciCodex/internal/openwrt"
	"github.com/aezizhu/LuciCodex/internal/plan"
	"github.com/aezizhu/LuciCodex/internal/policy"
)

// WebSocket opcodes
const (
	wsOpText   = 1
	wsOpBinary = 2
	wsOpClose  = 8
	wsOpPing   = 9
	wsOpPong   = 10
)

// WSConn represents a WebSocket connection (minimal implementation)
type WSConn struct {
	conn   net.Conn
	reader *bufio.Reader
	mu     sync.Mutex
}

// WSMessage represents a WebSocket message
type WSMessage struct {
	Type    string          `json:"type"`
	ID      string          `json:"id,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// StreamEvent represents a streaming event sent to the client
type StreamEvent struct {
	Type    string      `json:"type"` // "token", "plan", "exec_start", "exec_output", "exec_end", "error", "done"
	Data    interface{} `json:"data,omitempty"`
	Index   int         `json:"index,omitempty"`   // Command index for exec events
	Command string      `json:"command,omitempty"` // Command being executed
}

// upgradeWebSocket performs the WebSocket handshake
func upgradeWebSocket(w http.ResponseWriter, r *http.Request) (*WSConn, error) {
	if r.Header.Get("Upgrade") != "websocket" {
		return nil, fmt.Errorf("not a websocket upgrade request")
	}

	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		return nil, fmt.Errorf("missing Sec-WebSocket-Key")
	}

	// Compute accept key
	h := sha1.New()
	h.Write([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	acceptKey := base64.StdEncoding.EncodeToString(h.Sum(nil))

	// Hijack the connection
	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, fmt.Errorf("hijacking not supported")
	}

	conn, buf, err := hj.Hijack()
	if err != nil {
		return nil, err
	}

	// Send upgrade response
	response := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + acceptKey + "\r\n\r\n"

	if _, err := conn.Write([]byte(response)); err != nil {
		conn.Close()
		return nil, err
	}

	return &WSConn{conn: conn, reader: buf.Reader}, nil
}

// ReadMessage reads a WebSocket frame
func (ws *WSConn) ReadMessage() ([]byte, error) {
	// Read first 2 bytes
	header := make([]byte, 2)
	if _, err := io.ReadFull(ws.reader, header); err != nil {
		return nil, err
	}

	fin := header[0]&0x80 != 0
	opcode := header[0] & 0x0F
	masked := header[1]&0x80 != 0
	payloadLen := int(header[1] & 0x7F)

	// Handle control frames
	if opcode == wsOpClose {
		return nil, io.EOF
	}
	if opcode == wsOpPing {
		// Read ping payload and send pong
		if payloadLen > 0 {
			payload := make([]byte, payloadLen)
			io.ReadFull(ws.reader, payload)
		}
		ws.writePong()
		return ws.ReadMessage()
	}

	// Extended payload length
	if payloadLen == 126 {
		ext := make([]byte, 2)
		if _, err := io.ReadFull(ws.reader, ext); err != nil {
			return nil, err
		}
		payloadLen = int(ext[0])<<8 | int(ext[1])
	} else if payloadLen == 127 {
		ext := make([]byte, 8)
		if _, err := io.ReadFull(ws.reader, ext); err != nil {
			return nil, err
		}
		payloadLen = int(ext[4])<<24 | int(ext[5])<<16 | int(ext[6])<<8 | int(ext[7])
	}

	// Read mask key if masked
	var maskKey []byte
	if masked {
		maskKey = make([]byte, 4)
		if _, err := io.ReadFull(ws.reader, maskKey); err != nil {
			return nil, err
		}
	}

	// Read payload
	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(ws.reader, payload); err != nil {
		return nil, err
	}

	// Unmask if needed
	if masked {
		for i := range payload {
			payload[i] ^= maskKey[i%4]
		}
	}

	_ = fin // Handle fragmentation if needed in future
	return payload, nil
}

// WriteMessage writes a WebSocket frame
func (ws *WSConn) WriteMessage(data []byte) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	frame := make([]byte, 0, 10+len(data))
	frame = append(frame, 0x81) // FIN + Text opcode

	if len(data) < 126 {
		frame = append(frame, byte(len(data)))
	} else if len(data) < 65536 {
		frame = append(frame, 126, byte(len(data)>>8), byte(len(data)))
	} else {
		frame = append(frame, 127, 0, 0, 0, 0,
			byte(len(data)>>24), byte(len(data)>>16),
			byte(len(data)>>8), byte(len(data)))
	}

	frame = append(frame, data...)
	_, err := ws.conn.Write(frame)
	return err
}

// WriteJSON writes a JSON message
func (ws *WSConn) WriteJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return ws.WriteMessage(data)
}

func (ws *WSConn) writePong() {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.conn.Write([]byte{0x8A, 0}) // Pong frame
}

// Close closes the WebSocket connection
func (ws *WSConn) Close() error {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	// Send close frame
	ws.conn.Write([]byte{0x88, 0})
	return ws.conn.Close()
}

// handleWebSocket handles WebSocket connections for streaming
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Authenticate via query param or header
	token := r.URL.Query().Get("token")
	if token == "" {
		token = r.Header.Get("X-Auth-Token")
	}
	if s.token != "" && token != s.token {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ws, err := upgradeWebSocket(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer ws.Close()

	fmt.Println("WebSocket client connected")

	// Message handling loop
	for {
		data, err := ws.ReadMessage()
		if err != nil {
			if err != io.EOF {
				fmt.Printf("WebSocket read error: %v\n", err)
			}
			break
		}

		var msg WSMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			ws.WriteJSON(WSMessage{Type: "error", Error: "Invalid JSON"})
			continue
		}

		// Handle message based on type
		switch msg.Type {
		case "plan":
			s.handleWSPlan(ws, msg)
		case "execute":
			s.handleWSExecute(ws, msg)
		case "chat":
			s.handleWSChat(ws, msg)
		case "ping":
			ws.WriteJSON(WSMessage{Type: "pong", ID: msg.ID})
		default:
			ws.WriteJSON(WSMessage{Type: "error", ID: msg.ID, Error: "Unknown message type"})
		}
	}

	fmt.Println("WebSocket client disconnected")
}

// handleWSPlan handles plan generation with streaming
func (s *Server) handleWSPlan(ws *WSConn, msg WSMessage) {
	var req PlanRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		ws.WriteJSON(WSMessage{Type: "error", ID: msg.ID, Error: "Invalid payload"})
		return
	}

	cfg := s.mergeConfig(req.Provider, req.Model, req.Config)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.TimeoutSeconds)*time.Second)
	defer cancel()

	// Stream status updates
	ws.WriteJSON(StreamEvent{Type: "status", Data: "Collecting environment facts..."})

	factsCtx, factsCancel := context.WithTimeout(ctx, 3*time.Second)
	envFacts := openwrt.CollectFacts(factsCtx)
	factsCancel()

	ws.WriteJSON(StreamEvent{Type: "status", Data: "Generating plan..."})

	instruction := prompts.GenerateSurvivalPrompt(cfg.MaxCommands)
	if envFacts != "" {
		instruction += "\n\nEnvironment facts (read-only):\n" + envFacts
	}
	fullPrompt := instruction + "\n\nUser request: " + req.Prompt

	llmProvider := llm.NewProvider(cfg)
	p, err := llmProvider.GeneratePlan(ctx, fullPrompt)
	if err != nil {
		ws.WriteJSON(WSMessage{Type: "error", ID: msg.ID, Error: err.Error()})
		return
	}

	ws.WriteJSON(StreamEvent{Type: "plan", Data: p})
	ws.WriteJSON(StreamEvent{Type: "done"})
}

// handleWSExecute handles execution with real-time streaming
func (s *Server) handleWSExecute(ws *WSConn, msg WSMessage) {
	var req ExecuteRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		ws.WriteJSON(WSMessage{Type: "error", ID: msg.ID, Error: "Invalid payload"})
		return
	}

	cfg := s.mergeConfig(req.Provider, req.Model, req.Config)
	cfg.DryRun = req.DryRun
	if req.Timeout > 0 {
		cfg.TimeoutSeconds = req.Timeout
	}

	ctx := context.Background()
	policyEngine := policy.New(cfg)

	var p plan.Plan
	if len(req.Commands) > 0 {
		p = plan.Plan{Summary: "Direct execution", Commands: req.Commands}
	} else {
		// Generate plan first
		ws.WriteJSON(StreamEvent{Type: "status", Data: "Generating plan..."})
		llmProvider := llm.NewProvider(cfg)

		factsCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		envFacts := openwrt.CollectFacts(factsCtx)
		cancel()

		instruction := prompts.GenerateSurvivalPrompt(cfg.MaxCommands)
		if envFacts != "" {
			instruction += "\n\nEnvironment facts (read-only):\n" + envFacts
		}
		fullPrompt := instruction + "\n\nUser request: " + req.Prompt

		planCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.TimeoutSeconds)*time.Second)
		var err error
		p, err = llmProvider.GeneratePlan(planCtx, fullPrompt)
		cancel()
		if err != nil {
			ws.WriteJSON(WSMessage{Type: "error", ID: msg.ID, Error: err.Error()})
			return
		}
		ws.WriteJSON(StreamEvent{Type: "plan", Data: p})
	}

	if len(p.Commands) == 0 {
		ws.WriteJSON(StreamEvent{Type: "done", Data: "No commands to execute"})
		return
	}

	// Validate
	if err := policyEngine.ValidatePlan(p); err != nil {
		ws.WriteJSON(WSMessage{Type: "error", ID: msg.ID, Error: "Policy: " + err.Error()})
		return
	}

	if cfg.DryRun {
		ws.WriteJSON(StreamEvent{Type: "dry_run", Data: p})
		ws.WriteJSON(StreamEvent{Type: "done"})
		return
	}

	// Execute with streaming output
	execEngine := executor.New(cfg)
	ws.WriteJSON(StreamEvent{Type: "exec_start", Data: len(p.Commands)})

	for i, cmd := range p.Commands {
		cmdStr := executor.FormatCommand(cmd.Command)
		ws.WriteJSON(StreamEvent{
			Type:    "exec_cmd",
			Index:   i,
			Command: cmdStr,
			Data:    cmd.Description,
		})

		// Create a writer that streams to WebSocket
		streamWriter := &wsStreamWriter{ws: ws, index: i}
		result := execEngine.RunPlanStreaming(ctx, plan.Plan{Commands: []plan.PlannedCommand{cmd}}, streamWriter)

		if len(result.Items) > 0 {
			r := result.Items[0]
			ws.WriteJSON(StreamEvent{
				Type:  "exec_result",
				Index: i,
				Data: map[string]interface{}{
					"success": r.Err == nil,
					"output":  r.Output,
					"elapsed": r.Elapsed.String(),
				},
			})
		}
	}

	ws.WriteJSON(StreamEvent{Type: "done"})
}

// handleWSChat handles interactive chat with streaming
func (s *Server) handleWSChat(ws *WSConn, msg WSMessage) {
	var req struct {
		Message  string            `json:"message"`
		Provider string            `json:"provider"`
		Model    string            `json:"model"`
		Config   map[string]string `json:"config"`
	}
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		ws.WriteJSON(WSMessage{Type: "error", ID: msg.ID, Error: "Invalid payload"})
		return
	}

	cfg := s.mergeConfig(req.Provider, req.Model, req.Config)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.TimeoutSeconds)*time.Second)
	defer cancel()

	// Collect facts
	factsCtx, factsCancel := context.WithTimeout(ctx, 3*time.Second)
	envFacts := openwrt.CollectFacts(factsCtx)
	factsCancel()

	instruction := prompts.GenerateSurvivalPrompt(cfg.MaxCommands)
	if envFacts != "" {
		instruction += "\n\nEnvironment facts (read-only):\n" + envFacts
	}
	fullPrompt := instruction + "\n\nUser request: " + req.Message

	llmProvider := llm.NewProvider(cfg)
	p, err := llmProvider.GeneratePlan(ctx, fullPrompt)
	if err != nil {
		ws.WriteJSON(WSMessage{Type: "error", ID: msg.ID, Error: err.Error()})
		return
	}

	// Stream the response
	ws.WriteJSON(StreamEvent{Type: "chat_response", Data: p})
	ws.WriteJSON(StreamEvent{Type: "done"})
}

// wsStreamWriter implements io.Writer for streaming to WebSocket
type wsStreamWriter struct {
	ws    *WSConn
	index int
}

func (w *wsStreamWriter) Write(p []byte) (n int, err error) {
	lines := strings.Split(string(p), "\n")
	for _, line := range lines {
		if line != "" {
			w.ws.WriteJSON(StreamEvent{
				Type:  "exec_output",
				Index: w.index,
				Data:  line,
			})
		}
	}
	return len(p), nil
}

// mergeConfig merges request config with server config
func (s *Server) mergeConfig(provider, model string, cfgMap map[string]string) config.Config {
	cfg := s.cfg
	if provider != "" {
		cfg.Provider = provider
	}
	if model != "" {
		cfg.Model = model
	}
	if cfgMap != nil {
		if val, ok := cfgMap["openai_key"]; ok && val != "" {
			cfg.OpenAIAPIKey = val
		}
		if val, ok := cfgMap["gemini_key"]; ok && val != "" {
			cfg.APIKey = val
		}
		if val, ok := cfgMap["anthropic_key"]; ok && val != "" {
			cfg.AnthropicAPIKey = val
		}
	}
	cfg.ApplyProviderSettings()
	return cfg
}
