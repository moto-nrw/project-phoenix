package jwt

import (
	"encoding/hex"
	"testing"
)

func TestOpaqueCapabilityToken(t *testing.T) {
	t.Parallel()

	raw, hash, err := NewOpaqueCapabilityToken()
	if err != nil {
		t.Fatalf("NewOpaqueCapabilityToken() error = %v", err)
	}
	if len(raw) != 43 {
		t.Fatalf("raw token length = %d, want 43", len(raw))
	}
	if len(hash) != 64 {
		t.Fatalf("fingerprint length = %d, want 64", len(hash))
	}
	if _, err := hex.DecodeString(hash); err != nil {
		t.Fatalf("fingerprint is not hexadecimal: %v", err)
	}
	if hash != OpaqueCapabilityFingerprint(raw) {
		t.Fatal("fingerprint does not match raw token")
	}
}

func TestOpaqueCapabilityTokensDiffer(t *testing.T) {
	t.Parallel()

	first, _, err := NewOpaqueCapabilityToken()
	if err != nil {
		t.Fatalf("first NewOpaqueCapabilityToken() error = %v", err)
	}
	second, _, err := NewOpaqueCapabilityToken()
	if err != nil {
		t.Fatalf("second NewOpaqueCapabilityToken() error = %v", err)
	}
	if first == second {
		t.Fatal("two generated capability tokens are equal")
	}
}
