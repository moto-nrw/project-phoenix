package compose

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
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

type healthSchool struct {
	ctx                   context.Context
	studentID, membership int64
	siblingMembership     int64
}

// seedHealthSchool creates a school with two children whose care rows carry
// health info, so a write can be checked against a classmate.
func seedHealthSchool(t *testing.T, db *bun.DB, commands careplan.StudentProfileCommands, name string) healthSchool {
	t.Helper()
	testpkg.OwnTenant(t)
	ctx := testpkg.Ctx(t)
	seed := func(first string) (int64, int64) {
		student := testpkg.CreateTestStudent(t, db, "Health", first, "1a")
		var membershipID int64
		require.NoError(t, db.NewRaw("SELECT id FROM users.student_school_memberships WHERE tenant_id = ? AND student_profile_id = ? AND deleted_at IS NULL", testpkg.Tenant(t), student.ID).Scan(ctx, &membershipID))
		require.NoError(t, commands.SaveStudentCareProfile(ctx, careplan.StudentCareProfile{MembershipID: membershipID, HealthInfo: testpkg.StrPtr("Allergie " + first)}, nil))
		return student.ID, membershipID
	}
	studentID, membership := seed(name)
	_, sibling := seed(name + "-sibling")
	return healthSchool{ctx: ctx, studentID: studentID, membership: membership, siblingMembership: sibling}
}

func TestSetStudentHealthInfoWritesOnlyTheTenantsCareRow(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	commands, err := NewStudentProfiles(db, func(Observation) {})
	require.NoError(t, err)
	healthOf := func(t *testing.T, ctx context.Context, membershipID int64) *string {
		t.Helper()
		var health *string
		require.NoError(t, db.NewRaw("SELECT health_info FROM users.student_care_profiles WHERE membership_id = ?", membershipID).Scan(ctx, &health))
		return health
	}
	var own, foreign healthSchool
	t.Run("own", func(t *testing.T) { own = seedHealthSchool(t, db, commands, "own") })
	t.Run("foreign", func(t *testing.T) { foreign = seedHealthSchool(t, db, commands, "foreign") })

	changed, err := commands.SetStudentHealthInfo(own.ctx, own.studentID, testpkg.StrPtr("Asthma"))
	require.NoError(t, err)
	require.EqualValues(t, 1, changed)
	require.Equal(t, "Asthma", *healthOf(t, own.ctx, own.membership))
	require.Equal(t, "Allergie own-sibling", *healthOf(t, own.ctx, own.siblingMembership), "a classmate stays untouched")

	changed, err = commands.SetStudentHealthInfo(own.ctx, foreign.studentID, testpkg.StrPtr("fremd"))
	require.NoError(t, err)
	require.Zero(t, changed, "another school's child resolves to no care row")
	require.Equal(t, "Allergie foreign", *healthOf(t, foreign.ctx, foreign.membership))

	changed, err = commands.SetStudentHealthInfo(own.ctx, own.studentID, nil)
	require.NoError(t, err)
	require.EqualValues(t, 1, changed)
	require.Nil(t, healthOf(t, own.ctx, own.membership), "nil clears the column")

	failure := errors.New("next owner failed")
	err = tenant.WithinCurrentTenant(own.ctx, func(txCtx context.Context) error {
		changed, setErr := commands.SetStudentHealthInfo(txCtx, own.studentID, testpkg.StrPtr("must roll back"))
		require.NoError(t, setErr)
		require.EqualValues(t, 1, changed)
		return failure
	})
	require.ErrorIs(t, err, failure)
	require.Nil(t, healthOf(t, own.ctx, own.membership), "the write joins the caller's transaction")

	_, err = commands.SetStudentHealthInfo(own.ctx, 0, nil)
	require.Error(t, err)
}
