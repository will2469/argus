// Package mcp implements a native Model Context Protocol (MCP) server for Argus.
//
// The server communicates over JSON-RPC 2.0 via stdio and exposes Argus's
// 30-rule database hygiene engine as autonomous AI agent tools.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/will2469/argus/shared/mcp/telemetry"
	"github.com/will2469/argus/shared/mcp/tools"
	"github.com/will2469/argus/shared/version"
)

func init() {
	tools.RegisterTool(telemetry.NewReportIssueTool())
}

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
	scanner.Buffer(make([]byte, 0, 64*1024), MaxMessageSize)

	dispatcher := NewDispatcher(w, DefaultMaxConcurrentExpensive, DefaultMaxConcurrentCheap)
	var shutdownErr error
	defer func() { shutdownErr = dispatcher.Shutdown(DefaultShutdownTimeout) }()

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

		if req.Meta != nil && req.Meta.ProtocolVersion != "" {
			if !SupportedProtocolVersions[req.Meta.ProtocolVersion] {
				_ = dispatcher.WriteResponse(*UnsupportedProtocolVersionError(req.ID,
					"Unsupported protocol version in _meta"))
				continue
			}
		}

		isStateless := req.Meta != nil && isStatelessEra(req.Meta.ProtocolVersion)
		if !isStateless && cfg.strictLifecycle {
			// LEGACY PATH — statePreInit/stateInitializing/stateInitialized
			sess.mu.Lock()
			switch sess.state {
			case statePreInit:
				if req.Method == "initialize" {
					resp := handleRequest(context.Background(), *req)
					if resp == nil {
						sess.mu.Unlock()
						continue
					}
					if resp.Error != nil {
						sess.mu.Unlock()
						_ = dispatcher.WriteResponse(*resp)
						continue
					}
					sess.state = stateInitializing
					if resMap, ok := resp.Result.(map[string]any); ok {
						if v, ok := resMap["protocolVersion"].(string); ok {
							sess.protocolVersion = v
						}
					}
					sess.mu.Unlock()
					if err := dispatcher.WriteResponse(*resp); err != nil {
						return fmt.Errorf("failed to write initialize response: %w", err)
					}
					continue
				} else if req.Method != "ping" && req.Method != "server/discover" {
					sess.mu.Unlock()
					_ = dispatcher.WriteResponse(*ProtocolError(req.ID, CodeServerNotInitialized, "Server not initialized: 'initialize' must be called first"))
					continue
				}
			case stateInitializing:
				if req.Method == "initialize" {
					sess.mu.Unlock()
					_ = dispatcher.WriteResponse(*ProtocolError(req.ID, CodeInvalidRequest, "Server already initialized"))
					continue
				} else if req.Method != "ping" && req.Method != "server/discover" {
					sess.mu.Unlock()
					_ = dispatcher.WriteResponse(*ProtocolError(req.ID, CodeServerNotInitialized, "Server not initialized: awaiting 'notifications/initialized'"))
					continue
				}
			case stateInitialized:
				if req.Method == "initialize" {
					sess.mu.Unlock()
					_ = dispatcher.WriteResponse(*ProtocolError(req.ID, CodeInvalidRequest, "Server already initialized"))
					continue
				}
			}
			sess.mu.Unlock()
		}

		cost := CostCheap
		if req.Method == "tools/call" {
			var params struct {
				Name string `json:"name"`
			}
			if len(req.Params) == 0 || json.Unmarshal(req.Params, &params) != nil || params.Name == "" {
				_ = dispatcher.WriteResponse(*InvalidParamsError(req.ID, "Invalid params: 'name' is required"))
				continue
			}
			toolCost, ok := tools.DefaultRegistry.GetCost(params.Name)
			if !ok {
				_ = dispatcher.WriteResponse(*InvalidParamsError(req.ID, fmt.Sprintf("Unknown tool: %s", params.Name)))
				continue
			}
			cost = toolCost
		}

		dispatcher.Dispatch(*req, cost, handleRequest)
	}

	if err := scanner.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return fmt.Errorf("%w: limit is %d bytes", ErrOversizedMessage, MaxMessageSize)
		}
		return err
	}
	return errors.Join(dispatcher.Err(), shutdownErr)
}

func handleRequest(ctx context.Context, req ParsedRequest) *jsonrpcResponse {
	switch req.Method {
	case "server/discover":
		return &jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"resultType":        "complete",
				"protocolVersions":  []string{"2024-11-05", "2025-03-26", "2025-06-18", "2025-11-25", "2026-07-28"},
				"supportedVersions": []string{"2024-11-05", "2025-03-26", "2025-06-18", "2025-11-25", "2026-07-28"},
				"capabilities":      map[string]any{"tools": map[string]any{}, "extensions": map[string]any{}},
				"serverInfo":        map[string]any{"name": "argus", "version": version.Get()},
				"_meta":             map[string]any{"io.modelcontextprotocol/serverInfo": map[string]any{"name": "argus", "version": version.Get()}},
			},
		}

	case "initialize":
		var params struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		if len(req.Params) == 0 || json.Unmarshal(req.Params, &params) != nil || params.ProtocolVersion == "" {
			return InvalidParamsError(req.ID, "Invalid params: 'protocolVersion' is required")
		}

		return &jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"protocolVersion": NegotiateProtocolVersion(params.ProtocolVersion),
				"capabilities":    map[string]any{"tools": map[string]any{}, "extensions": map[string]any{}},
				"serverInfo":      map[string]any{"name": "argus", "version": version.Get()},
			},
		}

	case "ping":
		return &jsonrpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{}}

	case "tools/list":
		return &jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"resultType": "complete",
				"tools":      tools.DefaultRegistry.ListDefs(),
				"_meta": map[string]any{
					"ttlMs": 300000, "cacheScope": "private",
					"io.modelcontextprotocol/serverInfo": map[string]any{"name": "argus", "version": version.Get()},
				},
			},
		}

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
