package application

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// Every public sentinel has the internal twin's text; the domain tests pin
// the internal texts, this pins that every public sentinel is mapped.
func TestMapCallerErrorCoversEveryPublicSentinel(t *testing.T) {
	t.Parallel()

	public := []error{
		identityaccess.ErrCallerNotAuthenticated, identityaccess.ErrCallerNotFound, identityaccess.ErrCallerNotAuthorized,
		identityaccess.ErrCallerNotLinkedToPerson, identityaccess.ErrCallerNotLinkedToStaff,
		identityaccess.ErrCallerNotLinkedToTeacher, identityaccess.ErrCallerGroupNotFound,
		identityaccess.ErrCallerSessionNotFound,
	}
	require.Len(t, callerSentinels, len(public))
	for i, pair := range callerSentinels {
		assert.Same(t, public[i], pair[1])
		assert.Equal(t, pair[0].Error(), pair[1].Error(), "texts of %v", pair[1])
		assert.Same(t, pair[1], mapCallerError(pair[0]))
	}
	require.NoError(t, mapCallerError(nil))
	other := errors.New("SSE active service is not configured")
	assert.Same(t, other, mapCallerError(other))
}

// A use case's context around a caller-context error keeps its text on the
// way out (the review policy's "resolve ...: " prefixes), and errors.Is/As
// still reach the public error inside it.
func TestMapCallerErrorKeepsTheTextOfAWrappingContext(t *testing.T) {
	t.Parallel()

	for _, pair := range callerSentinels {
		wrapped := fmt.Errorf("resolve absence reviewer staff: %w", pair[0])
		mapped := mapCallerError(wrapped)
		assert.Equal(t, wrapped.Error(), mapped.Error())
		require.ErrorIs(t, mapped, pair[1])
	}

	wrapped := mapCallerError(fmt.Errorf("resolve request review groups: %w", errors.New("inner")))
	assert.Equal(t, "resolve request review groups: inner", wrapped.Error(), "nothing to translate leaves the error as it is")
}
