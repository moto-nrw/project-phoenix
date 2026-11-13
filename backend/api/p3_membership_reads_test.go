package api

import (
	"fmt"
	"testing"
	"time"

	owners "github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestP3MembershipOnlyInboxAndBirthdayExport(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	database := testpkg.NewDatabase(t, db)
	staffRows := NewPeopleRepositorySuiteFactory(database).Staff
	membership, err := owners.NewSchoolMembership(database)
	require.NoError(t, err)
	inbox := owners.ParentStaffMembers{Membership: membership}
	count := testpkg.CaptureQueries(t, db)
	var birthdayCount, inboxCount int
	for _, size := range []int{3, 8} {
		start := 0
		if size == 8 {
			start = 3
		}
		for i := start; i < size; i++ {
			staff := testpkg.CreateTestStaff(t, db, "Birthday", fmt.Sprintf("Staff %d", i))
			_, err := db.NewRaw("UPDATE users.persons SET birthday = ? WHERE id = ?", calendar.NewDate(1990, time.May, 17), staff.PersonID).Exec(testpkg.Ctx(t))
			require.NoError(t, err)
		}
		count.Reset()
		birthdays, err := testpkg.TenantQuery0(t, testpkg.Ctx(t), db, staffRows.ListBirthdaysForExport)
		require.NoError(t, err)
		require.Len(t, birthdays, size)
		t.Logf("P3 #5889 birthday export N%d: %d statements", size, len(count.Queries()))
		if size == 3 {
			birthdayCount = len(count.Queries())
		} else {
			require.Len(t, count.Queries(), birthdayCount, "birthday export must stay flat")
		}
		require.Empty(t, count.Selects("staff_employment_profiles"))
		count.Reset()
		members, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, inbox.List, []int64{testpkg.Tenant(t)})
		require.NoError(t, err)
		require.Len(t, members, size)
		for _, member := range members {
			require.Equal(t, testpkg.Tenant(t), member.TenantID)
			require.Positive(t, member.PersonID)
		}
		t.Logf("P3 #5889 parent inbox N%d: %d statements", size, len(count.Queries()))
		if size == 3 {
			inboxCount = len(count.Queries())
		} else {
			require.Len(t, count.Queries(), inboxCount, "parent inbox must stay flat")
		}
		require.Empty(t, count.Selects("staff_employment_profiles"))
	}
}
