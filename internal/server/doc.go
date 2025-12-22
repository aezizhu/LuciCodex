// Package server provides the HTTP daemon for LuciCodex.
//
// The server package implements a JSON API that enables the LuCI web
// interface to communicate with the LuciCodex backend. It listens on
// localhost only for security.
//
// Security features:
//   - Token-based authentication (token stored in /tmp/.lucicodex.token)
//   - Rate limiting (token bucket algorithm)
//   - Localhost-only binding (127.0.0.1)
//   - Request validation and sanitization
//
// API endpoints:
//   - POST /v1/plan      - Generate an execution plan from a prompt
//   - POST /v1/execute   - Execute commands from a plan
//   - POST /v1/summarize - Summarize command outputs
//   - GET  /health       - Health check (no auth required)
//
// Example usage:
//
//	cfg := config.Load("")
//	srv := server.New(cfg)
//	srv.Start(9999) // Blocks until server stops
package server
