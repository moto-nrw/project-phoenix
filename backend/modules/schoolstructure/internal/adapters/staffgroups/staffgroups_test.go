package staffgroups_test

import (
	"context"
	"testing"

	membershipcompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	structurecompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	workforcecompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// substitutionDay is the fixed Berlin calendar day every substitution
// fixture is measured against.
var substitutionDay = calendar.NewDate(2026, 3, 10)

// buildStaffGroups composes the reads over the real School Structure, School
// Membership and Workforce owners.
func buildStaffGroups(t *testing.T, db *bun.DB) schoolstructure.StaffGroupQuery {
	t.Helper()
	groups, err := structurecompose.New(structurecompose.Dependencies{DB: db, Observe: func(structurecompose.Observation) {}})
	require.NoError(t, err)
	membership, err := membershipcompose.New(membershipcompose.Dependencies{DB: db, Observe: func(membershipcompose.Observation) {}, Employment: staffEmployment(t, db)})
	require.NoError(t, err)
	substitutions, err := workforcecompose.New(workforcecompose.Dependencies{
		DB:                  db,
		LiveStaffIDs:        func(context.Context, []int64) ([]int64, error) { return nil, nil },
		LockStaffAssignment: func(context.Context, int64) error { return nil },
		Observe:             func(workforcecompose.Observation) {},
	})
	require.NoError(t, err)
	query, err := structurecompose.NewStaffGroups(structurecompose.StaffGroupsDependencies{
		Groups: groups, Assignments: membership, Substitutions: substitutions,
	})
	require.NoError(t, err)
	return query
}

func inTenant(t *testing.T, db *bun.DB, tenantID int64, fn func(context.Context)) {
	t.Helper()
	require.NoError(t, testpkg.WithTenantTx(t, context.Background(), db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		fn(ctx)
		return nil
	}))
}

func TestNewStaffGroupsRequiresEveryDependency(t *testing.T) {
	t.Parallel()
	_, err := structurecompose.NewStaffGroups(structurecompose.StaffGroupsDependencies{})
	require.Error(t, err)
}

func TestListGroupsByTeacherReturnsTheAssignedGroups(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	query := buildStaffGroups(t, db)
	teacher := testpkg.CreateTestTeacher(t, db, "Tina", "Teacher")
	other := testpkg.CreateTestTeacher(t, db, "Otto", "Other")
	igel := testpkg.CreateTestEducationGroup(t, db, "Igel")
	fuchs := testpkg.CreateTestEducationGroup(t, db, "Fuchs")
	foreign := testpkg.CreateTestEducationGroup(t, db, "Dachs")
	testpkg.CreateTestGroupTeacher(t, db, igel.ID, teacher.ID)
	testpkg.CreateTestGroupTeacher(t, db, fuchs.ID, teacher.ID)
	testpkg.CreateTestGroupTeacher(t, db, foreign.ID, other.ID)

	inTenant(t, db, testpkg.Tenant(t), func(ctx context.Context) {
		groups, err := query.ListGroupsByTeacher(ctx, teacher.ID)
		require.NoError(t, err)
		require.Len(t, groups, 2)
		assert.Equal(t, fuchs.ID, groups[0].ID, "sorted by name")
		assert.Equal(t, igel.ID, groups[1].ID)
		assert.Equal(t, testpkg.Tenant(t), groups[0].TenantID)

		_, err = query.ListGroupsByTeacher(ctx, 0)
		require.ErrorIs(t, err, schoolstructure.ErrInvalidStaff)
	})
}

func TestListSubstitutedGroupsCoversOnlyTheRequestedDay(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	query := buildStaffGroups(t, db)
	substitute := testpkg.CreateTestStaff(t, db, "Susi", "Substitute")
	regular := testpkg.CreateTestStaff(t, db, "Rita", "Regular")
	covered := testpkg.CreateTestEducationGroup(t, db, "Igel")
	unassigned := testpkg.CreateTestEducationGroup(t, db, "Fuchs")
	expired := testpkg.CreateTestEducationGroup(t, db, "Dachs")
	upcoming := testpkg.CreateTestEducationGroup(t, db, "Eule")

	testpkg.CreateTestGroupSubstitution(t, db, covered.ID, &regular.ID, substitute.ID, substitutionDay, substitutionDay)
	testpkg.CreateTestGroupSubstitution(t, db, unassigned.ID, nil, substitute.ID, substitutionDay.AddDays(-2), substitutionDay.AddDays(2))
	testpkg.CreateTestGroupSubstitution(t, db, expired.ID, nil, substitute.ID, substitutionDay.AddDays(-5), substitutionDay.AddDays(-1))
	testpkg.CreateTestGroupSubstitution(t, db, upcoming.ID, nil, substitute.ID, substitutionDay.AddDays(1), substitutionDay.AddDays(3))

	inTenant(t, db, testpkg.Tenant(t), func(ctx context.Context) {
		groups, err := query.ListSubstitutedGroups(ctx, substitute.ID, substitutionDay.String())
		require.NoError(t, err)
		require.Len(t, groups, 2, "expired and upcoming substitutions are not active on the day")
		assert.Equal(t, unassigned.ID, groups[0].Group.ID, "sorted by name")
		assert.True(t, groups[0].ViaSubstitution, "regular staff slot is unassigned")
		assert.Equal(t, covered.ID, groups[1].Group.ID)
		assert.False(t, groups[1].ViaSubstitution, "regular staff slot is assigned")

		none, err := query.ListSubstitutedGroups(ctx, regular.ID, substitutionDay.String())
		require.NoError(t, err)
		assert.Empty(t, none, "the regular staff member is not a substitute")
	})
}

func TestListSubstitutedGroupsRejectsAnInvalidDay(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	query := buildStaffGroups(t, db)
	substitute := testpkg.CreateTestStaff(t, db, "Susi", "Substitute")

	inTenant(t, db, testpkg.Tenant(t), func(ctx context.Context) {
		for _, day := range []string{"", "10.03.2026", "2026-02-30"} {
			_, err := query.ListSubstitutedGroups(ctx, substitute.ID, day)
			require.ErrorIs(t, err, schoolstructure.ErrInvalidDay, day)
		}
		_, err := query.ListSubstitutedGroups(ctx, 0, substitutionDay.String())
		require.ErrorIs(t, err, schoolstructure.ErrInvalidStaff)
	})
}

func TestListSubstitutedGroupsFlagsAGroupWithAnyUnassignedSlot(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	query := buildStaffGroups(t, db)
	substitute := testpkg.CreateTestStaff(t, db, "Susi", "Substitute")
	regular := testpkg.CreateTestStaff(t, db, "Rita", "Regular")
	group := testpkg.CreateTestEducationGroup(t, db, "Igel")
	testpkg.CreateTestGroupSubstitution(t, db, group.ID, &regular.ID, substitute.ID, substitutionDay, substitutionDay)
	testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, substitutionDay, substitutionDay)

	inTenant(t, db, testpkg.Tenant(t), func(ctx context.Context) {
		groups, err := query.ListSubstitutedGroups(ctx, substitute.ID, substitutionDay.String())
		require.NoError(t, err)
		require.Len(t, groups, 1, "one entry per group")
		assert.True(t, groups[0].ViaSubstitution)
	})
}

func TestListSchoolClassesByStaffKeepsClassOrder(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	query := buildStaffGroups(t, db)
	staff := testpkg.CreateTestStaff(t, db, "Klara", "Class")
	other := testpkg.CreateTestStaff(t, db, "Otto", "Other")
	testpkg.CreateTestClassTeacher(t, db, staff.ID, "3b")
	testpkg.CreateTestClassTeacher(t, db, staff.ID, "1a")
	testpkg.CreateTestClassTeacher(t, db, staff.ID, "2c")
	testpkg.CreateTestClassTeacher(t, db, other.ID, "4d")

	inTenant(t, db, testpkg.Tenant(t), func(ctx context.Context) {
		classes, err := query.ListSchoolClassesByStaff(ctx, staff.ID)
		require.NoError(t, err)
		assert.Equal(t, []string{"1a", "2c", "3b"}, classes)

		_, err = query.ListSchoolClassesByStaff(ctx, -1)
		require.ErrorIs(t, err, schoolstructure.ErrInvalidStaff)
	})
}

func TestStaffGroupReadsHideAnotherTenantsRows(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	query := buildStaffGroups(t, db)
	teacher := testpkg.CreateTestTeacher(t, db, "Tina", "Teacher")
	substitute := testpkg.CreateTestStaff(t, db, "Susi", "Substitute")
	assigned := testpkg.CreateTestEducationGroup(t, db, "Igel")
	substituted := testpkg.CreateTestEducationGroup(t, db, "Fuchs")
	testpkg.CreateTestGroupTeacher(t, db, assigned.ID, teacher.ID)
	testpkg.CreateTestGroupSubstitution(t, db, substituted.ID, nil, substitute.ID, substitutionDay, substitutionDay)
	testpkg.CreateTestClassTeacher(t, db, substitute.ID, "1a")

	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)

	inTenant(t, db, otherTenant, func(ctx context.Context) {
		groups, err := query.ListGroupsByTeacher(ctx, teacher.ID)
		require.NoError(t, err)
		assert.Empty(t, groups)

		substitutions, err := query.ListSubstitutedGroups(ctx, substitute.ID, substitutionDay.String())
		require.NoError(t, err)
		assert.Empty(t, substitutions)

		classes, err := query.ListSchoolClassesByStaff(ctx, substitute.ID)
		require.NoError(t, err)
		assert.Empty(t, classes)
	})
}
