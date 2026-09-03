package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/will2469/argus/shared/mcp"
	mcperrors "github.com/will2469/argus/shared/mcp/errors"
	"github.com/will2469/argus/shared/mcp/security"
	"github.com/will2469/argus/shared/mcp/telemetry"
	"github.com/will2469/argus/shared/mcp/tools"
)

// TestGauntlet_Concurrency stresses concurrent stdout writes and queue isolation.
func TestGauntlet_Concurrency(t *testing.T) {
	// 1. Stress Test: 50 concurrent goroutines writing responses to SynchronizedWriter
	// Output MUST have exactly 50 valid single-line JSON responses with zero interleaving!
	t.Run("concurrent_stdout_writes_no_interleaving", func(t *testing.T) {
		var out bytes.Buffer
		writer := mcp.NewSynchronizedWriter(&out)

		const count = 50
		var wg sync.WaitGroup
		wg.Add(count)

		for i := 0; i < count; i++ {
			go func(idx int) {
				defer wg.Done()
				resp := mcperrors.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      idx,
					Result:  map[string]any{"index": idx, "payload": strings.Repeat("A", 100)},
				}
				if err := writer.WriteResponse(resp); err != nil {
					t.Errorf("write error: %v", err)
				}
			}(i)
		}

		wg.Wait()

		lines := strings.Split(strings.TrimSpace(out.String()), "\n")
		if len(lines) != count {
			t.Fatalf("expected exactly %d lines, got %d", count, len(lines))
		}

		seen := make(map[int]bool)
		for lineIdx, line := range lines {
			var parsed mcperrors.JSONRPCResponse
			if err := json.Unmarshal([]byte(line), &parsed); err != nil {
				t.Fatalf("interleaving corruption detected at line %d: %v\ncontent: %s", lineIdx, err, line)
			}
			idx := int(parsed.ID.(float64))
			if seen[idx] {
				t.Fatalf("duplicate index %d found", idx)
			}
			seen[idx] = true
		}
	})

	// 2. Parallel scans bounded by expensive semaphore
	t.Run("parallel_scan_semaphore_bounded", func(t *testing.T) {
		var out bytes.Buffer
		disp := mcp.NewDispatcher(&out, 2, 10)
		defer func() {
			if err := disp.Shutdown(1 * time.Second); err != nil {
				t.Errorf("expected clean shutdown, got: %v", err)
			}
		}()

		var concurrentCount int32
		var maxObserved int32
		var mu sync.Mutex

		var wg sync.WaitGroup
		for i := 0; i < 4; i++ {
			wg.Add(1)
			disp.Dispatch(mcp.ParsedRequest{ID: i}, tools.CostExpensive, func(ctx context.Context, pr mcp.ParsedRequest) *mcperrors.JSONRPCResponse {
				defer wg.Done()
				mu.Lock()
				concurrentCount++
				if concurrentCount > maxObserved {
					maxObserved = concurrentCount
				}
				mu.Unlock()

				time.Sleep(50 * time.Millisecond)

				mu.Lock()
				concurrentCount--
				mu.Unlock()
				return &mcperrors.JSONRPCResponse{JSONRPC: "2.0", ID: pr.ID, Result: "ok"}
			})
		}

		wg.Wait()

		if maxObserved > 2 {
			t.Fatalf("expensive semaphore violated: observed %d concurrent executions, max allowed 2", maxObserved)
		}
	})
}

// TestGauntlet_Filesystem verifies path confinement, file inputs, and path deduplication.
func TestGauntlet_Filesystem(t *testing.T) {
	// 1. Current directory "." is valid
	t.Run("dot_directory_valid", func(t *testing.T) {
		pa, err := security.NewPathAuthority(".")
		if err != nil {
			t.Fatalf("failed to init authority with .: %v", err)
		}
		if _, err := pa.ValidatePath("."); err != nil {
			t.Fatalf("expected . to be valid, got: %v", err)
		}
	})

	// 2. Traversal ".." outside root is rejected
	t.Run("dot_dot_traversal_rejected", func(t *testing.T) {
		pa, _ := security.NewPathAuthority(".")
		if _, err := pa.ValidatePath("../../../etc"); err == nil {
			t.Fatal("expected traversal outside root to be rejected")
		}
	})

	// 3. Absolute path within root allowed, outside rejected
	t.Run("absolute_path_confinement", func(t *testing.T) {
		cwd, _ := os.Getwd()
		pa, _ := security.NewPathAuthority(cwd)
		if _, err := pa.ValidatePath(cwd); err != nil {
			t.Fatalf("expected cwd to be valid within authority, got: %v", err)
		}
		if _, err := pa.ValidatePath("/etc/shadow"); err == nil {
			t.Fatal("expected /etc/shadow to be rejected outside authority")
		}
	})

	// 4. File input instead of directory is scanned properly
	t.Run("file_instead_of_directory", func(t *testing.T) {
		req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"argus_scan","arguments":{"dirs":["authority.go"]}}}` + "\n"
		var out bytes.Buffer
		_ = mcp.Serve(strings.NewReader(req), &out)

		var resp mcperrors.JSONRPCResponse
		_ = json.Unmarshal(out.Bytes(), &resp)
		resMap, ok := resp.Result.(map[string]any)
		if !ok {
			t.Fatalf("expected result map, got: %v", resp)
		}
		content := resMap["content"].([]any)[0].(map[string]any)["text"].(string)
		if !strings.Contains(content, "Scanned 1 files") {
			t.Fatalf("expected 1 file scanned for single file target, got: %s", content)
		}
	})

	// 5. Duplicate paths in dirs are deduplicated
	t.Run("duplicate_paths_deduplicated", func(t *testing.T) {
		req := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"argus_scan","arguments":{"dirs":["authority.go", "authority.go"]}}}` + "\n"
		var out bytes.Buffer
		_ = mcp.Serve(strings.NewReader(req), &out)

		var resp mcperrors.JSONRPCResponse
		_ = json.Unmarshal(out.Bytes(), &resp)
		resMap := resp.Result.(map[string]any)
		content := resMap["content"].([]any)[0].(map[string]any)["text"].(string)
		if !strings.Contains(content, "Scanned 1 files") {
			t.Fatalf("expected duplicate files to be deduplicated to 1 file, got: %s", content)
		}
	})
}

// TestGauntlet_SideEffects verifies GH failure fallback with reason.
func TestGauntlet_SideEffects(t *testing.T) {
	t.Setenv("ARGUS_TELEMETRY", "true")

	// Verify submitViaURL explicitly states reason and does NOT claim submission succeeded
	t.Run("gh_failure_fallback_reason", func(t *testing.T) {
		resp := telemetry.SubmitViaURL("test-id", "Test Title", "Test Body", "bug", "gh: network timeout connecting to api.github.com")
		data, _ := json.Marshal(resp)
		var parsed mcperrors.JSONRPCResponse
		_ = json.Unmarshal(data, &parsed)
		resMap := parsed.Result.(map[string]any)
		content := resMap["content"].([]any)[0].(map[string]any)["text"].(string)

		if !strings.Contains(content, "STATUS: READY_FOR_SUBMISSION (NOT YET CREATED)") {
			t.Fatalf("expected READY_FOR_SUBMISSION status, got: %s", content)
		}
		if !strings.Contains(content, "network timeout") {
			t.Fatalf("expected failure reason in output, got: %s", content)
		}
		if strings.Contains(content, "Issue submitted successfully") {
			t.Fatalf("claimed success when submission failed: %s", content)
		}
	})
}
