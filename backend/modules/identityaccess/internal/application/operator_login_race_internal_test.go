package application

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOperatorLoginMintsNothingAfterAPasswordChange pins the operator side
// of the #6289 login race: the sign-in checks the password, then mints in a
// later transaction. A password rotated in between ended every session; the
// old password must not earn a new one.
func TestOperatorLoginMintsNothingAfterAPasswordChange(t *testing.T) {
	t.Parallel()
	f := newOperatorFixture(t)
	f.seedOperator(42)
	ctx := context.Background()
	checked, err := f.auth.validateCredentials(ctx, "operator42@example.com", "secret")
	require.NoError(t, err)

	rotated := f.store.operators[42]
	rotated.PasswordHash = "hash:renewed"
	f.store.operators[42] = rotated

	_, err = f.auth.authenticatedResult(ctx, checked, "127.0.0.1")
	require.ErrorIs(t, err, identityaccess.ErrOperatorInvalidCredentials)
	assert.Empty(t, f.store.operatorSessionsOf(42), "the old password earns no session")
}
