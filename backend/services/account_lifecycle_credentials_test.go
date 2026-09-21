package services

import "testing"

func TestPINHasher_RoundTrip(t *testing.T) {
	t.Parallel()

	hasher := pinHasher{}
	hash, err := hasher.HashPIN("1234")
	if err != nil {
		t.Fatalf("HashPIN() error = %v", err)
	}
	if hash == "" || hash == "1234" {
		t.Fatal("HashPIN() must return a nonempty hash, not the plain PIN")
	}
	if !hasher.VerifyPIN("1234", hash) {
		t.Error("VerifyPIN() should accept the correct PIN")
	}
	if hasher.VerifyPIN("9999", hash) {
		t.Error("VerifyPIN() should reject an incorrect PIN")
	}
}

func TestPINHasher_RejectsMissingOrMalformedHash(t *testing.T) {
	t.Parallel()

	for _, hash := range []string{"", "not-an-argon2-hash"} {
		if (pinHasher{}).VerifyPIN("1234", hash) {
			t.Errorf("VerifyPIN() accepted invalid hash %q", hash)
		}
	}
}
