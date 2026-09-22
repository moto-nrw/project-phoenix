package securityruntime

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The root binds this sentinel as the review policy's refusal of
// users:absence without users:read (#2267 A4); its text reaches the client.
func TestAbsenceReadRequiredNamesTheMissingPermission(t *testing.T) {
	t.Parallel()

	require.ErrorContains(t, ErrAbsenceReadRequired, "users:read permission is required")
}
