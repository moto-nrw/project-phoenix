package behavior_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The demo environment drops the operator's sign-in code, so the demo process
// that seeds every demo school could never finish the second factor. The
// demo's operator login mints the session directly (#3460); the demo host
// keeps the operator surface off the internet. Every other composition,
// including the demo access alone, keeps the second factor mandatory.
func TestOperatorLogin_OnlyTheDemoEnvironmentSkipsTheSecondFactor(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	gated := newOperatorMFATestModule(t, db).AccountAuthentication
	operator := testpkg.CreateTestOperatorWithPassword(t, db, operatorLoginEmail("operator-gated"), testPassword)
	result, err := gated.LoginOperatorWithMFAGate(ctx, operator.Email, testPassword, "127.0.0.1", "ua", "")
	require.NoError(t, err)
	assert.Equal(t, identityaccess.LoginStatusMFAEnrollmentRequired, result.Status)
	assert.Empty(t, result.RefreshToken)

	demo := newOperatorMFATestModule(t, db, services.WithDemoOperatorWithoutSecondFactor()).AccountAuthentication
	operator = testpkg.CreateTestOperatorWithPassword(t, db, operatorLoginEmail("operator-demo"), testPassword)
	result, err = demo.LoginOperatorWithMFAGate(ctx, operator.Email, testPassword, "127.0.0.1", "ua", "")
	require.NoError(t, err)
	assert.False(t, result.MFAEnrollmentRequired)
	assert.NotEmpty(t, result.AccessToken)
	assert.NotEmpty(t, result.RefreshToken)
	assert.Equal(t, 1, countOperatorSessions(t, db, operator.ID), "the demo login opens the operator session")
}
