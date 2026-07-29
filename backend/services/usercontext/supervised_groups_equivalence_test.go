package usercontext_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSupervisedGroupsEquivalence is the guard for the read filter that decides
// which children a staff member may see.
//
// There are two implementations of "which education groups does this person
// supervise": usercontext.GetMyGroups, which resolves the person from JWT
// claims and is used on every request, and
// education.GroupRepository.ListSupervisedGroupIDsByStaff, which answers the
// same question by staff ID and without an authenticated user, for callers that
// need many people at once.
//
// Two implementations of one authorization rule is exactly the shape that drifts
// silently, and the direction it drifts matters: a wider bulk result exposes
// children to staff who may not read them. So the two are compared directly,
// over the cases where they could plausibly disagree, instead of each being
// checked against a hand-written expectation.
func TestSupervisedGroupsEquivalence(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupUserContextService(t, db)
	groupRepo := repositories.NewFactory(db).Group
	tenantCtx := testpkg.TenantContext(1)
	today := timezone.TodayDate()

	// assertAgrees compares both implementations for one staff member and
	// reports the group sets on failure.
	assertAgrees := func(t *testing.T, accountID int64, staffID int64, what string) {
		t.Helper()

		groups, err := service.GetMyGroups(contextWithClaims(int(accountID)))
		require.NoError(t, err)

		viaClaims := make(map[int64]struct{}, len(groups))
		for _, group := range groups {
			viaClaims[group.ID] = struct{}{}
		}

		pairs, err := groupRepo.ListSupervisedGroupIDsByStaff(tenantCtx, []int64{staffID}, today)
		require.NoError(t, err)

		viaBulk := make(map[int64]struct{}, len(pairs))
		for _, pair := range pairs {
			if pair.StaffID == staffID {
				viaBulk[pair.GroupID] = struct{}{}
			}
		}

		assert.Equal(t, viaClaims, viaBulk,
			"%s: the bulk read filter must resolve exactly the groups GetMyGroups resolves", what)
	}

	t.Run("teacher with assigned groups", func(t *testing.T) {
		teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "EquivAssigned", "Teacher")
		groupOne := testpkg.CreateTestEducationGroup(t, db, "EquivAssignedOne")
		groupTwo := testpkg.CreateTestEducationGroup(t, db, "EquivAssignedTwo")
		linkOne := testpkg.CreateTestGroupTeacher(t, db, groupOne.ID, teacher.ID)
		linkTwo := testpkg.CreateTestGroupTeacher(t, db, groupTwo.ID, teacher.ID)

		defer testpkg.CleanupActivityFixtures(t, db, linkOne.ID, linkTwo.ID, groupOne.ID, groupTwo.ID)
		defer testpkg.CleanupTeacherFixtures(t, db, teacher.ID)
		defer testpkg.CleanupStaffFixtures(t, db, teacher.StaffID)
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		assertAgrees(t, account.ID, teacher.StaffID, "teacher with two groups")
	})

	t.Run("staff member without a teacher record", func(t *testing.T) {
		staff, account := testpkg.CreateTestStaffWithAccount(t, db, "EquivPlain", "Staff")

		defer testpkg.CleanupStaffFixtures(t, db, staff.ID)
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		assertAgrees(t, account.ID, staff.ID, "staff without teacher record")
	})

	t.Run("active substitute", func(t *testing.T) {
		staff, account := testpkg.CreateTestStaffWithAccount(t, db, "EquivActiveSub", "Staff")
		group := testpkg.CreateTestEducationGroup(t, db, "EquivActiveSubGroup")
		sub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, staff.ID,
			today.AddDays(-1), today.AddDays(3))

		defer testpkg.CleanupActivityFixtures(t, db, sub.ID, group.ID)
		defer testpkg.CleanupStaffFixtures(t, db, staff.ID)
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		assertAgrees(t, account.ID, staff.ID, "active substitute")
	})

	t.Run("expired substitute", func(t *testing.T) {
		staff, account := testpkg.CreateTestStaffWithAccount(t, db, "EquivExpiredSub", "Staff")
		group := testpkg.CreateTestEducationGroup(t, db, "EquivExpiredSubGroup")
		sub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, staff.ID,
			today.AddDays(-10), today.AddDays(-2))

		defer testpkg.CleanupActivityFixtures(t, db, sub.ID, group.ID)
		defer testpkg.CleanupStaffFixtures(t, db, staff.ID)
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		assertAgrees(t, account.ID, staff.ID, "expired substitute")
	})

	t.Run("substitution starting and ending today", func(t *testing.T) {
		staff, account := testpkg.CreateTestStaffWithAccount(t, db, "EquivOneDaySub", "Staff")
		group := testpkg.CreateTestEducationGroup(t, db, "EquivOneDaySubGroup")
		sub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, staff.ID, today, today)

		defer testpkg.CleanupActivityFixtures(t, db, sub.ID, group.ID)
		defer testpkg.CleanupStaffFixtures(t, db, staff.ID)
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		assertAgrees(t, account.ID, staff.ID, "one-day substitution")
	})

	t.Run("teacher who is also an active substitute elsewhere", func(t *testing.T) {
		// The union case: both branches contribute, and the bulk reader must
		// neither drop nor duplicate either source.
		teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "EquivBoth", "Teacher")
		ownGroup := testpkg.CreateTestEducationGroup(t, db, "EquivBothOwn")
		coveredGroup := testpkg.CreateTestEducationGroup(t, db, "EquivBothCovered")
		link := testpkg.CreateTestGroupTeacher(t, db, ownGroup.ID, teacher.ID)
		sub := testpkg.CreateTestGroupSubstitution(t, db, coveredGroup.ID, nil, teacher.StaffID,
			today.AddDays(-1), today.AddDays(1))

		defer testpkg.CleanupActivityFixtures(t, db, sub.ID, link.ID, ownGroup.ID, coveredGroup.ID)
		defer testpkg.CleanupTeacherFixtures(t, db, teacher.ID)
		defer testpkg.CleanupStaffFixtures(t, db, teacher.StaffID)
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		assertAgrees(t, account.ID, teacher.StaffID, "teacher and substitute at once")
	})

	t.Run("teacher substituting in a group they already teach", func(t *testing.T) {
		// Both branches yield the same group. GetMyGroups dedupes through a map;
		// the bulk reader must not report the pair twice.
		teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "EquivOverlap", "Teacher")
		group := testpkg.CreateTestEducationGroup(t, db, "EquivOverlapGroup")
		link := testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher.ID)
		sub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, teacher.StaffID,
			today.AddDays(-1), today.AddDays(1))

		defer testpkg.CleanupActivityFixtures(t, db, sub.ID, link.ID, group.ID)
		defer testpkg.CleanupTeacherFixtures(t, db, teacher.ID)
		defer testpkg.CleanupStaffFixtures(t, db, teacher.StaffID)
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		assertAgrees(t, account.ID, teacher.StaffID, "overlapping sources")

		pairs, err := groupRepo.ListSupervisedGroupIDsByStaff(tenantCtx,
			[]int64{teacher.StaffID}, today)
		require.NoError(t, err)

		matches := 0
		for _, pair := range pairs {
			if pair.StaffID == teacher.StaffID && pair.GroupID == group.ID {
				matches++
			}
		}
		assert.Equal(t, 1, matches, "an overlapping group must be reported exactly once")
	})
}
