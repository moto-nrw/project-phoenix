package parent

import (
	"errors"
	"testing"
)

func strPtr(s string) *string { return &s }

// TestValidateContactInputEmailNormalization pins that a submitted email is
// validated AND normalized to its bare addr-spec: mail.ParseAddress accepts
// display-name forms ("Oma <oma@x.de>"), and the wrapper must never be
// persisted as the email. Pure function, no DB.
func TestValidateContactInputEmailNormalization(t *testing.T) {
	cases := []struct {
		name      string
		email     *string
		wantErr   bool
		wantEmail string // expected *input.Email after validation (when no error)
	}{
		{name: "plain address unchanged", email: strPtr("oma@example.de"), wantEmail: "oma@example.de"},
		{name: "display name stripped", email: strPtr("Oma <oma@example.de>"), wantEmail: "oma@example.de"},
		{name: "display name with spaces stripped", email: strPtr("Oma Müller <oma@example.de>"), wantEmail: "oma@example.de"},
		{name: "surrounding whitespace trimmed then normalized", email: strPtr("  Oma <oma@example.de>  "), wantEmail: "oma@example.de"},
		{name: "blank email left as-is (cleared)", email: strPtr("   "), wantEmail: "   "},
		{name: "nil email untouched", email: nil},
		{name: "invalid rejected", email: strPtr("not-an-email"), wantErr: true},
		{name: "multiple addresses rejected", email: strPtr(`a@b.de, c@d.de`), wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := &GuardianContactInput{FirstName: "First", LastName: "Last", Email: tc.email}
			err := validateContactInput(input)
			if tc.wantErr {
				if !errors.Is(err, ErrGuardianContactInvalid) {
					t.Fatalf("expected ErrGuardianContactInvalid, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.email == nil {
				if input.Email != nil {
					t.Fatalf("nil email should stay nil, got %q", *input.Email)
				}
				return
			}
			if input.Email == nil {
				t.Fatalf("email became nil unexpectedly")
			}
			if *input.Email != tc.wantEmail {
				t.Fatalf("email = %q, want %q", *input.Email, tc.wantEmail)
			}
		})
	}
}
