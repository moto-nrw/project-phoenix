package compose

import (
	"errors"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every lifecycle sentinel reaches consumers as its public twin with the same
// text: the kiosk maps the staff PIN texts, the HTTP layers the rest.
func TestLifecycleErrorTranslatesEverySentinel(t *testing.T) {
	t.Parallel()

	for _, sentinel := range lifecycleSentinels {
		require.Equal(t, sentinel.internal.Error(), sentinel.public.Error(), "texts of %v", sentinel.public)
		assert.Same(t, sentinel.public, lifecycleError(sentinel.internal))

		wrapped := lifecycleError(fmt.Errorf("context: %w", sentinel.internal))
		require.ErrorIs(t, wrapped, sentinel.public)
		assert.Equal(t, "context: "+sentinel.internal.Error(), wrapped.Error())
	}
}

// The account, tenant and session sentinels the preview and offboarding share
// with the login flows fall through to the session mapping.
func TestLifecycleErrorFallsThroughToTheSessionMapping(t *testing.T) {
	t.Parallel()

	for _, sentinel := range authenticationSentinels {
		assert.Same(t, sentinel.public, lifecycleError(sentinel.internal))
	}
	other := errors.New("database is down")
	assert.Same(t, other, lifecycleError(other))
	assert.NoError(t, lifecycleError(nil))
}

func TestModuleWithoutLifecycleReportsUnavailable(t *testing.T) {
	t.Parallel()

	module := identityaccess.NewModule(engine{})
	_, err := module.AuthenticateStaffPIN(t.Context(), 7, 8, "1234")
	require.ErrorIs(t, err, identityaccess.ErrAccountLifecycleUnavailable)
	require.ErrorIs(t, module.RevokeAccess(t.Context(), identityaccess.RevokeAccessRequest{}), identityaccess.ErrAccountLifecycleUnavailable)
}
