package compose

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestStudentCareProfileJoinsCallerRollbackAndTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "CareProfile", "Rollback", "1a")
	var membershipID int64
	require.NoError(t, db.NewRaw("SELECT id FROM users.student_school_memberships WHERE tenant_id = ? AND student_profile_id = ? AND deleted_at IS NULL", testpkg.Tenant(t), student.ID).Scan(ctx, &membershipID))
	commands, err := NewStudentProfiles(db, func(Observation) {})
	require.NoError(t, err)
	input := careplan.StudentCareProfile{MembershipID: membershipID, HealthInfo: testpkg.StrPtr("original")}
	require.NoError(t, commands.SaveStudentCareProfile(ctx, input, nil))
	failure := errors.New("next owner failed")
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		input.HealthInfo = testpkg.StrPtr("must roll back")
		input.Sick = true
		require.NoError(t, commands.SaveStudentCareProfile(txCtx, input, nil))
		return failure
	})
	require.ErrorIs(t, err, failure)
	var health string
	var sick bool
	require.NoError(t, db.NewRaw("SELECT health_info, sick FROM users.student_care_profiles WHERE tenant_id = ? AND membership_id = ?", testpkg.Tenant(t), membershipID).Scan(ctx, &health, &sick))
	require.Equal(t, "original", health)
	require.False(t, sick)
	foreign := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreign)
	foreignCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), foreign)
	require.Error(t, commands.SaveStudentCareProfile(foreignCtx, input, nil), "membership ID alone must not grant another school access")
	require.NoError(t, db.NewRaw("SELECT health_info FROM users.student_care_profiles WHERE tenant_id = ? AND membership_id = ?", testpkg.Tenant(t), membershipID).Scan(ctx, &health))
	require.Equal(t, "original", health)
}
