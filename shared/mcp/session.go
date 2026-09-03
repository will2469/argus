package mcp

import (
	"encoding/json"
	"sync"
)

// serverSession tracks connection lifecycle state exclusively for legacy clients (2024-11-05 through 2025-11-25).
// IMPORTANT: Under MCP 2026-07-28 stateless core, requests are self-describing via _meta.
// Stateless requests MUST NOT read from or write to serverSession, as servers must not rely on prior requests.
type serverSession struct {
	mu              sync.Mutex
	state           lifecycleState
	protocolVersion string
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
		if sess.state == stateInitializing {
			sess.state = stateInitialized
		}
		sess.mu.Unlock()
	}
}
