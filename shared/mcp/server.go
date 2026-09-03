// Package mcp implements a native Model Context Protocol (MCP) server for Argus.
//
// The server communicates over JSON-RPC 2.0 via stdio and exposes Argus's
// 30-rule database hygiene engine as autonomous AI agent tools.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/will2469/argus/shared/mcp/telemetry"
	"github.com/will2469/argus/shared/mcp/tools"
	"github.com/will2469/argus/shared/version"
)

func init() {
	tools.RegisterTool(telemetry.NewReportIssueTool())
}

const protocolVersion = "2024-11-05"

// ServerOption configures server runtime policies.
type ServerOption func(*serverConfig)

type serverConfig struct {
	strictLifecycle bool
}

// WithStrictLifecycle configures whether the server enforces the standard MCP lifecycle state machine.
func WithStrictLifecycle(strict bool) ServerOption {
	return func(cfg *serverConfig) {
		cfg.strictLifecycle = strict
	}
}

type serverSession struct {
	mu           sync.Mutex
	initializing bool
	initialized  bool
}

// Serve starts the MCP server reading from r and writing to w with optional configuration.
func Serve(r io.Reader, w io.Writer, opts ...ServerOption) error {
	return serve(r, w, opts...)
}

// ServeStdio starts the MCP server reading from stdin and writing to stdout with strict lifecycle enforcement.
func ServeStdio() error {
	return serve(os.Stdin, os.Stdout, WithStrictLifecycle(true))
}

func serve(r io.Reader, w io.Writer, opts ...ServerOption) error {
	var cfg serverConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	scanner := bufio.NewScanner(r)
	// MCP messages can be large; allow up to 10MB per line.
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	dispatcher := NewDispatcher(w, DefaultMaxConcurrentExpensive, DefaultMaxConcurrentCheap)
	defer dispatcher.Shutdown(DefaultShutdownTimeout)

	sess := &serverSession{}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		req, errResp := ValidateJSONRPC([]byte(line))
		if errResp != nil {
			if err := dispatcher.WriteResponse(*errResp); err != nil {
				return fmt.Errorf("failed to write jsonrpc error response: %w", err)
			}
			continue
		}

		if req.IsNotification {
			handleNotification(*req, dispatcher, sess)
			continue
		}

		if cfg.strictLifecycle {
			sess.mu.Lock()
			if req.Method == "initialize" {
				if sess.initialized || sess.initializing {
					sess.mu.Unlock()
					_ = dispatcher.WriteResponse(*ProtocolError(req.ID, CodeInvalidRequest, "Server already initialized"))
					continue
				}
				sess.initializing = true
			} else if req.Method != "ping" && !sess.initialized && !sess.initializing {
				sess.mu.Unlock()
				_ = dispatcher.WriteResponse(*ProtocolError(req.ID, -32002, "Server not initialized: 'initialize' must be called first"))
				continue
			}
			sess.mu.Unlock()
		}

		cost := determineRequestCost(*req)
		dispatcher.Dispatch(*req, cost, handleRequest)
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	return dispatcher.Err()
}

func determineRequestCost(req ParsedRequest) ResourceCost {
	if req.Method != "tools/call" {
		return CostCheap
	}
	var params struct {
		Name string `json:"name"`
	}
	if len(req.Params) > 0 && json.Unmarshal(req.Params, &params) == nil {
		return tools.DefaultRegistry.GetCost(params.Name)
	}
	return CostCheap
}

func handleNotification(req ParsedRequest, dispatcher *Dispatcher, sess *serverSession) {
	switch req.Method {
	case "notifications/cancelled":
		var params struct {
			RequestID any `json:"requestId"`
		}
		if len(req.Params) > 0 && json.Unmarshal(req.Params, &params) == nil && params.RequestID != nil {
			dispatcher.HandleCancel(params.RequestID)
		}
	case "notifications/initialized":
		sess.mu.Lock()
		sess.initialized = true
		sess.initializing = false
		sess.mu.Unlock()
	}
}

func handleRequest(ctx context.Context, req ParsedRequest) *jsonrpcResponse {
	switch req.Method {
	case "initialize":
		return &jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"protocolVersion": protocolVersion,
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo": map[string]any{
					"name":    "argus",
					"version": version.Get(),
				},
			},
		}

	case "ping":
		return &jsonrpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{}}

	case "tools/list":
		return &jsonrpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"tools": tools.DefaultRegistry.ListDefs()}}

	case "tools/call":
		return handleToolCall(ctx, req)

	default:
		return MethodNotFoundError(req.ID, req.Method)
	}
}

func handleToolCall(ctx context.Context, req ParsedRequest) *jsonrpcResponse {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if len(req.Params) == 0 || json.Unmarshal(req.Params, &params) != nil || params.Name == "" {
		return InvalidParamsError(req.ID, "Invalid params")
	}

	return tools.DefaultRegistry.Dispatch(ctx, params.Name, req.ID, params.Arguments)
}
