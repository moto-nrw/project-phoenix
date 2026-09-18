package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The masking rule the operator ledger and the delivery logs share: an
// audit entry or a log line must not carry a full address, because the
// operator ledger is read by every operator and the log by everyone with
// access to it.
func TestMaskOperatorEmail(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		input    string
		expected string
	}{
		"normal address":       {"operator@example.com", "o***@example.com"},
		"two-character local":  {"ab@example.com", "***@example.com"},
		"one-character local":  {"a@example.com", "***@example.com"},
		"empty":                {"", "***"},
		"no at sign":           {"invalid-email", "***"},
		"empty local part":     {"@example.com", "***"},
		"long local part":      {"verylonglocalpart@domain.org", "v***@domain.org"},
		"three-character part": {"abc@domain.org", "a***@domain.org"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, MaskOperatorEmail(tc.input))
		})
	}
}

// The address an operator invitation or e-mail change is stored with is
// lower-cased, canonicalized to the bare mailbox and required to be one
// somebody can be reached at: mail.ParseAddress alone accepts "t@t".
func TestNormalizeOperatorEmail(t *testing.T) {
	t.Parallel()

	routable := func(address string) bool { return address != "invitee@localhost" }

	for name, tc := range map[string]struct {
		input   string
		want    string
		refused bool
	}{
		"normalized":     {input: "  Invitee@Example.COM  ", want: "invitee@example.com"},
		"display name":   {input: "Invitee <invitee@example.com>", want: "invitee@example.com"},
		"not an address": {input: "not-an-email", refused: true},
		"not routable":   {input: "invitee@localhost", refused: true},
		"empty":          {input: "", refused: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := NormalizeOperatorEmail(tc.input, routable)
			if tc.refused {
				var invalid *InvalidInputError
				require.ErrorAs(t, err, &invalid)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
