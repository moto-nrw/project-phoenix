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

// staffIDsFor collapses the flat pair list into the staff set of one group,
// which is what a producer looking for "who is responsible for this child"
// consumes.
func staffIDsFor(pairs []education.StaffGroupID, groupID int64) map[int64]struct{} {
	out := make(map[int64]struct{})
	for _, pair := range pairs {
		if pair.GroupID == groupID {
			out[pair.StaffID] = struct{}{}
		}
	}
	return out
}

// TestGroupRepository_ListStaffIDsByEducationGroupIDs pins who supervises a
// group on a given day: teacher assignments plus substitutions active on that
// day, and nobody else.
func TestGroupRepository_ListStaffIDsByEducationGroupIDs(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).Group
	ctx := testpkg.Ctx(t)
	today := timezone.TodayDate()

	t.Run("names the assigned teacher and the substitute of today", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "ByGroupAssigned")
		teacher := testpkg.CreateTestTeacher(t, db, "ByGroup", "Teacher")
		testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher.ID)
		substitute := testpkg.CreateTestStaff(t, db, "ByGroup", "Substitute")
		testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID,
			today.AddDays(-1), today.AddDays(1))

		pairs, err := repo.ListStaffIDsByEducationGroupIDs(ctx, []int64{group.ID}, today)
		require.NoError(t, err)

		staff := staffIDsFor(pairs, group.ID)
		assert.Contains(t, staff, teacher.StaffID)
		assert.Contains(t, staff, substitute.ID)
	})

	t.Run("a substitution outside the day is not counted", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "ByGroupPastSub")
		substitute := testpkg.CreateTestStaff(t, db, "Past", "Substitute")
		testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID,
			today.AddDays(-10), today.AddDays(-5))

		pairs, err := repo.ListStaffIDsByEducationGroupIDs(ctx, []int64{group.ID}, today)
		require.NoError(t, err)
		assert.NotContains(t, staffIDsFor(pairs, group.ID), substitute.ID,
			"a substitution that ended last week says nothing about today")
	})

	t.Run("no groups means no staff", func(t *testing.T) {
		pairs, err := repo.ListStaffIDsByEducationGroupIDs(ctx, nil, today)
		require.NoError(t, err)
		assert.Empty(t, pairs, "an empty request must not resolve to everybody")
	})

	t.Run("another school's group resolves to nobody", func(t *testing.T) {
		otherTenant := testpkg.UniqueTestTenantID(t)
		testpkg.EnsureTestTenant(t, db, otherTenant)

		foreignGroup := testpkg.CreateTestEducationGroupForTenant(t, db, otherTenant, "ByGroupForeign")

		pairs, err := repo.ListStaffIDsByEducationGroupIDs(ctx, []int64{foreignGroup.ID}, today)
		require.NoError(t, err)
		assert.Empty(t, staffIDsFor(pairs, foreignGroup.ID))
	})
}
