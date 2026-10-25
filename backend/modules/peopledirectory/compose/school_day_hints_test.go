package compose

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestSchoolDayHintsRequireCallerTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := notesModule(t, db)
	ctx := testpkg.Ctx(t)
	child := testpkg.CreateTestStudent(t, db, "Mila", "DayHint", "3a")
	author := noteAuthor(t, db, "Sara", "DayHint")
	day := calendar.NewDate(2099, 3, 2)
	write := peopledirectory.SetSchoolDayHint{
		AuthorAccountID: author, Date: day, StudentIDs: []int64{child.ID}, Body: "Bring sports kit",
	}
	clear := peopledirectory.ClearSchoolDayHint{AuthorAccountID: author, Date: day, StudentID: child.ID}

	require.ErrorIs(t, module.SetSchoolDayHint(ctx, write), tenant.ErrTransactionRequired)
	_, err := module.ListSchoolDayHints(ctx, day, []int64{child.ID})
	require.ErrorIs(t, err, tenant.ErrTransactionRequired)
	require.ErrorIs(t, module.ClearSchoolDayHint(ctx, clear), tenant.ErrTransactionRequired)

	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, module.SetSchoolDayHint, write))
	require.ErrorIs(t, module.ClearSchoolDayHint(ctx, clear), tenant.ErrTransactionRequired)
	hints, err := testpkg.TenantQuery2(t, ctx, db, module.ListSchoolDayHints, day, []int64{child.ID})
	require.NoError(t, err)
	require.Len(t, hints, 1)
	require.Equal(t, write.Body, hints[0].Body)

	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, module.ClearSchoolDayHint, clear))
	hints, err = testpkg.TenantQuery2(t, ctx, db, module.ListSchoolDayHints, day, []int64{child.ID})
	require.NoError(t, err)
	require.Empty(t, hints)
}
