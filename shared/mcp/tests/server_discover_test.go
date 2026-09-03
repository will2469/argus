package tests

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/will2469/argus/shared/mcp"
	mcperrors "github.com/will2469/argus/shared/mcp/errors"
)

func TestServerDiscover_StatelessPreInit(t *testing.T) {
	// 1. Calling server/discover during statePreInit must succeed statelessly
	req := `{"jsonrpc":"2.0","id":1,"method":"server/discover"}` + "\n"
	var out bytes.Buffer
	err := mcp.Serve(strings.NewReader(req), &out, mcp.WithStrictLifecycle(true))
	if err != nil {
		t.Fatalf("serve failed: %v", err)
	}

	var resp mcperrors.JSONRPCResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v\nraw: %s", err, out.String())
	}
	if resp.Error != nil {
		t.Fatalf("expected server/discover to succeed, got error: %v", resp.Error)
	}

	resMap, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got: %T", resp.Result)
	}

	if resMap["resultType"] != "complete" {
		t.Fatalf("expected resultType=complete, got: %v", resMap["resultType"])
	}

	versions, ok := resMap["supportedVersions"].([]any)
	if !ok || len(versions) < 2 {
		t.Fatalf("expected supportedVersions array with >=2 versions, got: %v", resMap["supportedVersions"])
	}

	has2026 := false
	has2024 := false
	for _, v := range versions {
		if v == "2026-07-28" {
			has2026 = true
		}
		if v == "2024-11-05" {
			has2024 = true
		}
	}
	if !has2026 || !has2024 {
		t.Fatalf("expected supportedVersions to contain 2026-07-28 and 2024-11-05, got: %v", versions)
	}

	meta, ok := resMap["_meta"].(map[string]any)
	if !ok {
		t.Fatalf("expected _meta in server/discover response, got: %v", resMap["_meta"])
	}
	serverInfo, ok := meta["io.modelcontextprotocol/serverInfo"].(map[string]any)
	if !ok || serverInfo["name"] != "argus" {
		t.Fatalf("expected serverInfo name=argus, got: %v", serverInfo)
	}
}

func TestToolsList_StatelessWithMetaCaching(t *testing.T) {
	// 2. Calling tools/list with 2026-07-28 _meta in statePreInit must succeed and include cache headers
	req := `{"jsonrpc":"2.0","id":10,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}` + "\n"
	var out bytes.Buffer
	err := mcp.Serve(strings.NewReader(req), &out, mcp.WithStrictLifecycle(true))
	if err != nil {
		t.Fatalf("serve failed: %v", err)
	}

	var resp mcperrors.JSONRPCResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v\nraw: %s", err, out.String())
	}
	if resp.Error != nil {
		t.Fatalf("expected stateless tools/list to succeed, got error: %v", resp.Error)
	}

	resMap, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got: %T", resp.Result)
	}

	toolsList, ok := resMap["tools"].([]any)
	if !ok || len(toolsList) != 4 {
		t.Fatalf("expected 4 registered tools, got: %v", resMap["tools"])
	}

	meta, ok := resMap["_meta"].(map[string]any)
	if !ok {
		t.Fatalf("expected _meta caching metadata, got: %v", resMap["_meta"])
	}
	if meta["cacheScope"] != "workspace" {
		t.Fatalf("expected cacheScope=workspace, got: %v", meta["cacheScope"])
	}
	ttl, ok := meta["ttlMs"].(float64)
	if !ok || ttl <= 0 {
		t.Fatalf("expected positive ttlMs, got: %v", meta["ttlMs"])
	}
}

func TestToolsCall_StatelessWithMeta(t *testing.T) {
	// 3. Calling tools/call with 2026-07-28 _meta during statePreInit must execute statelessly
	req := `{"jsonrpc":"2.0","id":20,"method":"tools/call","params":{"name":"argus_explain_rule","arguments":{"rule_code":"A01"},"_meta":{"protocolVersion":"2026-07-28"}}}` + "\n"
	var out bytes.Buffer
	err := mcp.Serve(strings.NewReader(req), &out, mcp.WithStrictLifecycle(true))
	if err != nil {
		t.Fatalf("serve failed: %v", err)
	}

	var resp mcperrors.JSONRPCResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v\nraw: %s", err, out.String())
	}
	if resp.Error != nil {
		t.Fatalf("expected stateless tools/call to succeed, got error: %v", resp.Error)
	}

	resMap, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got: %T", resp.Result)
	}
	content, ok := resMap["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("expected content in result, got: %v", resMap)
	}
}

func TestUnsupportedProtocolVersionInMeta(t *testing.T) {
	// 4. Declaring an unsupported protocol version in _meta must return -32022
	req := `{"jsonrpc":"2.0","id":30,"method":"tools/list","params":{"_meta":{"protocolVersion":"2099-01-01"}}}` + "\n"
	var out bytes.Buffer
	_ = mcp.Serve(strings.NewReader(req), &out, mcp.WithStrictLifecycle(true))

	var resp mcperrors.JSONRPCResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v\nraw: %s", err, out.String())
	}
	if resp.Error == nil {
		t.Fatal("expected error on unsupported protocol version in _meta")
	}

	errMap, ok := resp.Error.(map[string]any)
	if !ok {
		t.Fatalf("expected map error, got: %T", resp.Error)
	}
	code := int(errMap["code"].(float64))
	if code != mcp.CodeInvalidParams && code != mcp.CodeUnsupportedProtocolVersion {
		t.Fatalf("expected error code %d (-32602) or %d (-32022), got: %d", mcp.CodeInvalidParams, mcp.CodeUnsupportedProtocolVersion, code)
	}
}

func TestLegacyClientWithoutMeta_StrictLifecycleEnforced(t *testing.T) {
	// 5. A request without 2026-07-28 _meta or initialize must still be rejected with -32002
	req := `{"jsonrpc":"2.0","id":40,"method":"tools/list"}` + "\n"
	var out bytes.Buffer
	_ = mcp.Serve(strings.NewReader(req), &out, mcp.WithStrictLifecycle(true))

	var resp mcperrors.JSONRPCResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v\nraw: %s", err, out.String())
	}
	if resp.Error == nil {
		t.Fatal("expected error on uninitialized tools/list without _meta")
	}

	errMap, ok := resp.Error.(map[string]any)
	if !ok {
		t.Fatalf("expected map error, got: %T", resp.Error)
	}
	code := int(errMap["code"].(float64))
	if code != mcp.CodeServerNotInitialized {
		t.Fatalf("expected error code %d (-32002), got: %d", mcp.CodeServerNotInitialized, code)
	}
}
