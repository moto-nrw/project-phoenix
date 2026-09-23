package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
