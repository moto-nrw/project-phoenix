package compose

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tenantRecordingAccountMFA records the tenant each operator admin write
// runs under.
type tenantRecordingAccountMFA struct {
	identityaccess.AccountMFA
	tenants []int64
}

func (m *tenantRecordingAccountMFA) record(ctx context.Context) {
	m.tenants = append(m.tenants, tenantRuntime{}.TenantID(ctx))
}

func (m *tenantRecordingAccountMFA) OperatorDisableMFA(ctx context.Context, _, _, _ int64, _ string) error {
	m.record(ctx)
	return nil
}

func (m *tenantRecordingAccountMFA) OperatorSetMFAOverride(ctx context.Context, _, _, _ int64, _, _ string) error {
	m.record(ctx)
	return nil
}

// The operator routes run outside the tenant middleware. The school-scoped
// MFA admin writes therefore run under the school they name, so the audit
// append sees it (#3231: moved from the operator HTTP adapter into the
// owner, which may name the tenant runtime).
func TestOperatorMFAAdminWritesRunUnderTheNamedSchool(t *testing.T) {
	t.Parallel()

	const schoolID = int64(4711)
	recorder := &tenantRecordingAccountMFA{}
	e := engine{mfaFlows: mfaFlows{capability: recorder}}

	require.NoError(t, e.OperatorDisableMFA(context.Background(), 1, schoolID, 2, "reason"))
	require.NoError(t, e.OperatorSetMFAOverride(context.Background(), 1, schoolID, 2, identityaccess.MFAAdminOverrideForceOff, "reason"))

	assert.Equal(t, []int64{schoolID, schoolID}, recorder.tenants)
}

// A missing school reaches the application unchanged, which refuses it;
// scoping the context must not panic on it first.
func TestOperatorMFAAdminWritesLeaveAMissingSchoolToTheApplication(t *testing.T) {
	t.Parallel()

	recorder := &tenantRecordingAccountMFA{}
	e := engine{mfaFlows: mfaFlows{capability: recorder}}

	require.NoError(t, e.OperatorDisableMFA(context.Background(), 1, 0, 2, "reason"))
	require.NoError(t, e.OperatorSetMFAOverride(context.Background(), 1, 0, 2, identityaccess.MFAAdminOverrideForceOff, "reason"))

	assert.Equal(t, []int64{0, 0}, recorder.tenants)
}
