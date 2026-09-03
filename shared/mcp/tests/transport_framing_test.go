package tests

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/will2469/argus/shared/mcp"
)

func TestTransport_OversizedMessageTerminatesConnection(t *testing.T) {
	// Build a message that exceeds MaxMessageSize (10 MiB).
	// Use a valid JSON-RPC structure so the only rejection reason is size.
	oversizedPayload := `{"jsonrpc":"2.0","id":1,"method":"ping","params":{"data":"` +
		strings.Repeat("A", mcp.MaxMessageSize) + `"}}` + "\n"

	var out bytes.Buffer
	err := mcp.Serve(strings.NewReader(oversizedPayload), &out)

	// 1. Serve MUST return ErrOversizedMessage (connection-fatal).
	if !errors.Is(err, mcp.ErrOversizedMessage) {
		t.Fatalf("expected ErrOversizedMessage, got: %v", err)
	}

	// 2. No partial stdout: output buffer must be empty.
	// An oversized message breaks framing, so no JSON-RPC response should be written.
	if out.Len() != 0 {
		t.Fatalf("expected zero stdout on oversized message, got %d bytes: %s", out.Len(), out.String())
	}
}

func TestTransport_MessageAtLimitAccepted(t *testing.T) {
	// Build a message that is just under MaxMessageSize to prove the limit
	// is not rejecting valid large messages.
	// Padding size = MaxMessageSize - envelope overhead - newline
	envelope := `{"jsonrpc":"2.0","id":1,"method":"ping","params":{"data":"`
	suffix := `"}}` + "\n"
	overhead := len(envelope) + len(suffix)
	padding := mcp.MaxMessageSize - overhead - 100 // 100 byte margin to stay under

	payload := envelope + strings.Repeat("B", padding) + suffix

	var out bytes.Buffer
	err := mcp.Serve(strings.NewReader(payload), &out)

	// Must NOT return ErrOversizedMessage. The message is within limits.
	if errors.Is(err, mcp.ErrOversizedMessage) {
		t.Fatalf("message within limit should not be rejected as oversized")
	}

	// Output should contain a response (method not found or similar, but not empty).
	if out.Len() == 0 {
		t.Fatal("expected a response for a valid-sized message, got empty output")
	}
}

func TestTransport_OversizedDoesNotExecutePendingRequests(t *testing.T) {
	// Send a valid request followed by an oversized one.
	// The valid request should execute, but the oversized one must terminate the connection
	// without any partial execution or response from the oversized message.
	validRequest := `{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n"
	oversizedRequest := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"data":"` +
		strings.Repeat("X", mcp.MaxMessageSize) + `"}}` + "\n"

	combined := validRequest + oversizedRequest

	var out bytes.Buffer
	err := mcp.Serve(strings.NewReader(combined), &out)

	// Connection terminates with ErrOversizedMessage.
	if !errors.Is(err, mcp.ErrOversizedMessage) {
		t.Fatalf("expected ErrOversizedMessage, got: %v", err)
	}

	// The first (valid) request should have produced a response.
	if out.Len() == 0 {
		t.Fatal("expected the valid request to produce output before termination")
	}

	// But the output must NOT contain id:2 (the oversized request was never dispatched).
	if strings.Contains(out.String(), `"id":2`) {
		t.Fatal("oversized request was partially executed — framing invariant violated")
	}
}
