package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// NewApprovalManager initializes an ApprovalManager.
func NewApprovalManager() *ApprovalManager {
	return &ApprovalManager{
		tokens: make(map[string]approvalEntry),
	}
}

// ComputeIssueHash creates a deterministic SHA-256 digest of the complete issue payload.
func ComputeIssueHash(ruleCode, title, description, snippet, category string) [32]byte {
	h := sha256.New()
	h.Write([]byte(ruleCode))
	h.Write([]byte{0})
	h.Write([]byte(title))
	h.Write([]byte{0})
	h.Write([]byte(description))
	h.Write([]byte{0})
	h.Write([]byte(snippet))
	h.Write([]byte{0})
	h.Write([]byte(category))
	var digest [32]byte
	copy(digest[:], h.Sum(nil))
	return digest
}

// CreateToken creates a single-use token bound to payloadHash with DefaultApprovalTTL.
func (m *ApprovalManager) CreateToken(payloadHash [32]byte) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.pruneExpiredLocked()

	if len(m.tokens) >= MaxStoredApprovals {
		for k := range m.tokens {
			delete(m.tokens, k)
			break
		}
	}

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}
	token := "appr_" + hex.EncodeToString(b)

	m.tokens[token] = approvalEntry{
		payloadHash: payloadHash,
		expiresAt:   time.Now().Add(DefaultApprovalTTL),
	}

	return token, nil
}

// ConsumeToken atomically verifies and destroys the token.
func (m *ApprovalManager) ConsumeToken(token string, currentPayloadHash [32]byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.tokens[token]
	if !ok {
		return ErrTokenNotFound
	}

	delete(m.tokens, token)

	if time.Now().After(entry.expiresAt) {
		return ErrTokenExpired
	}

	if entry.payloadHash != currentPayloadHash {
		return ErrPayloadMismatch
	}

	return nil
}

func (m *ApprovalManager) pruneExpiredLocked() {
	now := time.Now()
	for k, v := range m.tokens {
		if now.After(v.expiresAt) {
			delete(m.tokens, k)
		}
	}
}
