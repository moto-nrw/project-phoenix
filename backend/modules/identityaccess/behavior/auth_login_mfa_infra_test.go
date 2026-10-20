package behavior_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// The gate's failure paths break one statement of the record port, so they
// run beside the flows in modules/identityaccess/internal/application
// (account_mfa_record_faults_internal_test.go). The composed login keeps the
// legitimate "not enrolled" case.

// A missing enrollment row is the legitimate "not enrolled" signal every
// fresh account hits: the gate proceeds instead of refusing.
func TestLoginWithMFAGate_MissingEnrollmentRow_TreatedAsNotEnrolled(t *testing.T) {
	t.Parallel()

	sc := newLoginGateScenario(t)

	result, err := sc.svc.LoginWithMFAGate(
		context.Background(), sc.email, loginGatePassword, "127.0.0.1", "ua-test", "", "",
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, identityaccess.LoginStatusAuthenticated, result.Status)
}
