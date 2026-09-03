package security

import (
	"errors"
	"sync"
	"time"
)

// Policy & Approval Constants
const (
	// DefaultApprovalTTL is the maximum validity duration for an approval token (10 minutes).
	DefaultApprovalTTL = 10 * time.Minute
	// MaxStoredApprovals limits the number of pending tokens to prevent memory exhaustion.
	MaxStoredApprovals = 256

	// MaxMigrationSQLBytes is the maximum allowed size of a migration file checked in-memory.
	MaxMigrationSQLBytes = 1024 * 1024 // 1MB
	// MaxReportTitleChars is the maximum length of an issue title.
	MaxReportTitleChars = 250
	// MaxReportPayloadBytes is the maximum allowed size of an issue report payload.
	MaxReportPayloadBytes = 512 * 1024 // 512KB
	// MaxScanDirsLimit is the maximum number of scan directories allowed in a single request.
	MaxScanDirsLimit = 50
)

// Sentinel Errors
var (
	// ErrTokenNotFound is returned when the approval token is absent or has already been consumed.
	ErrTokenNotFound = errors.New("approval token not found or already consumed")
	// ErrTokenExpired is returned when the approval token was presented after its TTL.
	ErrTokenExpired = errors.New("approval token has expired; please request a new preview")
	// ErrPayloadMismatch is returned when the payload hash at submission does not match the previewed payload.
	ErrPayloadMismatch = errors.New("payload mutation detected: issue contents were altered after approval was requested")
)

// Shared Singletons
var (
	// DefaultApprovalManager provides a shared instance for tool operations.
	DefaultApprovalManager = NewApprovalManager()
)

// Approval Types

type approvalEntry struct {
	payloadHash [32]byte
	expiresAt   time.Time
}

// ApprovalManager manages single-use, short-lived, payload-bound approval tokens for HITL operations.
type ApprovalManager struct {
	mu     sync.Mutex
	tokens map[string]approvalEntry
}

// Authority Types

// PathAuthority enforces strict filesystem isolation boundaries against explicitly
// allowed roots. It defends against directory traversal ("../") and symlink escapes
// via filepath.EvalSymlinks at validation time.
//
// TOCTOU Limitation: ValidatePath performs point-in-time symlink resolution.
// Between validation and actual file access, a symlink swap could redirect the
// path outside the allowed roots. For kernel-level containment immune to TOCTOU
// races, use SafeOpen / SafeStat which leverage os.Root (openat2 RESOLVE_BENEATH
// on Linux).
type PathAuthority struct {
	canonicalRoots []string
}

// Schema Types

// Schema represents the Argus MCP Schema Subset for tool input validation.
//
// This is an intentionally limited subset of JSON Schema, supporting only the
// features needed by Argus tool definitions. It is NOT a general-purpose JSON
// Schema validator and does not implement: minimum, maximum, pattern,
// additionalProperties, oneOf, anyOf, allOf, const, format, $ref, or
// recursive/nested object schema validation.
//
// Supported features:
//   - Top-level type: "object" (required)
//   - properties: map of property name -> Property (type, description, enum, items)
//   - required: list of required property names
//   - Property types: "string", "boolean", "number", "integer", "array", "object"
//   - enum: string-only enum constraint (on "string" properties)
//   - items: element type validation for "array" properties (all supported types)
type Schema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

// Property defines a single property within the Argus MCP Schema Subset.
type Property struct {
	Type        string    `json:"type"`
	Description string    `json:"description,omitempty"`
	Enum        []string  `json:"enum,omitempty"`
	Items       *Property `json:"items,omitempty"`
}
