package education_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/education"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// groupIDsFor collapses the flat pair list into the group set of one staff
// member, which is the shape every caller actually consumes.
func groupIDsFor(pairs []education.StaffGroupID, staffID int64) map[int64]struct{} {
	out := make(map[int64]struct{})
	for _, pair := range pairs {
		if pair.StaffID == staffID {
			out[pair.GroupID] = struct{}{}
		}
	}
	return out
}

// TestGroupRepository_ListSupervisedGroupIDsByStaff pins the read filter that
// decides which children a staff member may see. Every case here is a case
// where getting it wrong either hides a group from someone entitled to it or,
// far worse, exposes one to someone who is not.
func TestGroupRepository_ListSupervisedGroupIDsByStaff(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Group
	ctx := testpkg.TenantContext(1)
	today := timezone.TodayDate()

	t.Run("returns groups the staff member teaches", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SupervisedAssigned")
		teacher := testpkg.CreateTestTeacher(t, db, "Assigned", "Teacher")
		link := testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher.ID)

		defer testpkg.CleanupActivityFixtures(t, db, link.ID, group.ID)
		defer testpkg.CleanupTeacherFixtures(t, db, teacher.ID)
		defer testpkg.CleanupStaffFixtures(t, db, teacher.StaffID)

		pairs, err := repo.ListSupervisedGroupIDsByStaff(ctx, []int64{teacher.StaffID}, today)
		require.NoError(t, err)

		assert.Contains(t, groupIDsFor(pairs, teacher.StaffID), group.ID)
	})

	t.Run("returns groups covered by a substitution active today", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SupervisedSubstituted")
		substitute := testpkg.CreateTestStaff(t, db, "Active", "Substitute")
		sub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID,
			today.AddDays(-1), today.AddDays(1))

		defer testpkg.CleanupActivityFixtures(t, db, sub.ID, group.ID)
		defer testpkg.CleanupStaffFixtures(t, db, substitute.ID)

		pairs, err := repo.ListSupervisedGroupIDsByStaff(ctx, []int64{substitute.ID}, today)
		require.NoError(t, err)

		assert.Contains(t, groupIDsFor(pairs, substitute.ID), group.ID)
	})

	t.Run("substitution boundaries are inclusive on both ends", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SupervisedBoundary")
		substitute := testpkg.CreateTestStaff(t, db, "Boundary", "Substitute")
		// A one-day substitution that starts and ends today must count. The
		// helper this mirrors (Filter.DateBetween) is inclusive, and an
		// exclusive rewrite here would silently strip single-day cover.
		sub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, today, today)

		defer testpkg.CleanupActivityFixtures(t, db, sub.ID, group.ID)
		defer testpkg.CleanupStaffFixtures(t, db, substitute.ID)

		pairs, err := repo.ListSupervisedGroupIDsByStaff(ctx, []int64{substitute.ID}, today)
		require.NoError(t, err)

		assert.Contains(t, groupIDsFor(pairs, substitute.ID), group.ID)
	})

	t.Run("excludes an expired substitution", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SupervisedExpired")
		substitute := testpkg.CreateTestStaff(t, db, "Expired", "Substitute")
		sub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID,
			today.AddDays(-10), today.AddDays(-1))

		defer testpkg.CleanupActivityFixtures(t, db, sub.ID, group.ID)
		defer testpkg.CleanupStaffFixtures(t, db, substitute.ID)

		pairs, err := repo.ListSupervisedGroupIDsByStaff(ctx, []int64{substitute.ID}, today)
		require.NoError(t, err)

		assert.NotContains(t, groupIDsFor(pairs, substitute.ID), group.ID,
			"cover that ended yesterday must not grant access today")
	})

	t.Run("a staff member without a teacher record supervises nothing", func(t *testing.T) {
		// The dangerous failure mode: a join that yields no rows must mean
		// "sees nothing", never "sees everything".
		group := testpkg.CreateTestEducationGroup(t, db, "SupervisedUnrelated")
		teacher := testpkg.CreateTestTeacher(t, db, "Unrelated", "Teacher")
		link := testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher.ID)
		plainStaff := testpkg.CreateTestStaff(t, db, "Plain", "Staff")

		defer testpkg.CleanupActivityFixtures(t, db, link.ID, group.ID)
		defer testpkg.CleanupTeacherFixtures(t, db, teacher.ID)
		defer testpkg.CleanupStaffFixtures(t, db, teacher.StaffID, plainStaff.ID)

		pairs, err := repo.ListSupervisedGroupIDsByStaff(ctx, []int64{plainStaff.ID}, today)
		require.NoError(t, err)

		assert.Empty(t, groupIDsFor(pairs, plainStaff.ID),
			"no teacher record and no substitution means no groups at all")
	})

	t.Run("excludes a soft-deleted staff member", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SupervisedDeleted")
		teacher := testpkg.CreateTestTeacher(t, db, "Deleted", "Teacher")
		link := testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher.ID)

		defer testpkg.CleanupActivityFixtures(t, db, link.ID, group.ID)
		defer testpkg.CleanupTeacherFixtures(t, db, teacher.ID)
		defer testpkg.CleanupStaffFixtures(t, db, teacher.StaffID)

		_, err := db.NewUpdate().
			TableExpr("users.staff").
			Set("deleted_at = NOW()").
			Where("id = ?", teacher.StaffID).
			Exec(ctx)
		require.NoError(t, err)

		pairs, err := repo.ListSupervisedGroupIDsByStaff(ctx, []int64{teacher.StaffID}, today)
		require.NoError(t, err)

		assert.Empty(t, groupIDsFor(pairs, teacher.StaffID),
			"an offboarded staff member keeps no group access")
	})

	t.Run("keeps staff members apart within one batch", func(t *testing.T) {
		groupA := testpkg.CreateTestEducationGroup(t, db, "SupervisedBatchA")
		groupB := testpkg.CreateTestEducationGroup(t, db, "SupervisedBatchB")
		teacherA := testpkg.CreateTestTeacher(t, db, "BatchA", "Teacher")
		teacherB := testpkg.CreateTestTeacher(t, db, "BatchB", "Teacher")
		linkA := testpkg.CreateTestGroupTeacher(t, db, groupA.ID, teacherA.ID)
		linkB := testpkg.CreateTestGroupTeacher(t, db, groupB.ID, teacherB.ID)

		defer testpkg.CleanupActivityFixtures(t, db, linkA.ID, linkB.ID, groupA.ID, groupB.ID)
		defer testpkg.CleanupTeacherFixtures(t, db, teacherA.ID, teacherB.ID)
		defer testpkg.CleanupStaffFixtures(t, db, teacherA.StaffID, teacherB.StaffID)

		pairs, err := repo.ListSupervisedGroupIDsByStaff(ctx,
			[]int64{teacherA.StaffID, teacherB.StaffID}, today)
		require.NoError(t, err)

		forA := groupIDsFor(pairs, teacherA.StaffID)
		forB := groupIDsFor(pairs, teacherB.StaffID)

		assert.Contains(t, forA, groupA.ID)
		assert.NotContains(t, forA, groupB.ID, "batching must not merge two people's groups")
		assert.Contains(t, forB, groupB.ID)
		assert.NotContains(t, forB, groupA.ID, "batching must not merge two people's groups")
	})

	t.Run("empty input short-circuits", func(t *testing.T) {
		pairs, err := repo.ListSupervisedGroupIDsByStaff(ctx, nil, today)
		require.NoError(t, err)
		assert.Empty(t, pairs)
	})

	t.Run("does not leak another tenant's assignments", func(t *testing.T) {
		const otherTenant int64 = 99043
		testpkg.EnsureTestTenant(t, db, otherTenant)

		foreignGroup := testpkg.CreateTestEducationGroupForTenant(t, db, otherTenant, "ForeignSupervised")
		foreignStaff := testpkg.CreateTestStaffForTenant(t, db, otherTenant, "Foreign", "Teacher")

		defer testpkg.CleanupActivityFixturesForTenant(t, db, otherTenant, foreignGroup.ID)
		defer testpkg.CleanupStaffFixtures(t, db, foreignStaff.ID)

		// Asked from tenant 1, a foreign staff ID must resolve to nothing even
		// though the row exists in another school.
		pairs, err := repo.ListSupervisedGroupIDsByStaff(ctx, []int64{foreignStaff.ID}, today)
		require.NoError(t, err)

		assert.Empty(t, groupIDsFor(pairs, foreignStaff.ID))
	})
}
