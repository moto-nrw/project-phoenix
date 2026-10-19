package parentportal_test

import (
	"errors"
	"testing"

	api "github.com/moto-nrw/project-phoenix/roots/owners"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	"github.com/stretchr/testify/require"
)

func TestNativeProfileMissingClassRetainsOriginalCause(t *testing.T) {
	t.Parallel()
	got := api.NewParentGuardianPolicy().MissingProfileError()
	require.ErrorIs(t, got, parentService.ErrGuardianProfileNotFound)
	cause := errors.Unwrap(got)
	require.Error(t, cause)
	require.Equal(t, cause.Error(), got.Error())
	require.ErrorIs(t, got, cause)
}
