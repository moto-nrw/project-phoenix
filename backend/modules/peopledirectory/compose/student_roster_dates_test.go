package compose_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRosterDateSetKeepsPointMembershipAndTenantScope(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := peopleCompose.New(peopleCompose.Dependencies{
		DB: db, StudentDirectory: testpkg.StudentDirectory{}, Observe: func(peopleCompose.Observation) {},
	}, nil)
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	for _, tc := range []struct{ name, status, from, until, class string }{
		{"regular", "active", "", "", "2a"},
		{"immediate", "active", "2026-09-14", "", "2a"},
		{"later entrant", "pending", "2026-09-09", "", "3b"},
		{"ended", "inactive", "2026-09-01", "2026-09-08", "2a"},
		{"unbounded inactive", "inactive", "", "", "2a"},
		{"graduate", "alumnus", "2026-09-01", "2026-09-30", "2a"},
		{"guest arrived", "active", "2026-09-08", "2026-09-11", peopledirectory.HolidayGuestSchoolClass},
		{"guest later", "active", "2026-09-10", "2026-09-11", peopledirectory.HolidayGuestSchoolClass},
	} {
		child := testpkg.CreateTestStudent(t, db, tc.name, "Roster", tc.class)
		var from, until *calendar.Date
		if tc.from != "" {
			value := calendar.Date(tc.from)
			from = &value
		}
		if tc.until != "" {
			value := calendar.Date(tc.until)
			until = &value
		}
		_, err := db.ExecContext(ctx, `UPDATE users.student_school_memberships SET status = ?, enrolled_from = ?, enrolled_until = ? WHERE student_profile_id = ? AND tenant_id = ?`, tc.status, from, until, child.ID, testpkg.Tenant(t))
		require.NoError(t, err)
	}
	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	otherCtx := tenant.WithTenantID(ctx, otherTenant)
	foreign := testpkg.CreateTestStudentForTenant(t, db, otherTenant, "Foreign", "Roster", "2a")
	dates := []string{"2026-09-07", "2026-09-08", "2026-09-09", "2026-09-11"}
	for _, today := range []string{"2026-09-08", "2026-09-12"} {
		batch, err := module.ListStudentRosterOnDates(ctx, dates, today)
		require.NoError(t, err)
		for _, date := range dates {
			point, err := module.ListStudentRosterOverlapping(ctx, date, date, today)
			require.NoError(t, err)
			assert.Equal(t, point, batch[date], "date %s, today %s", date, today)
			for _, entry := range batch[date] {
				assert.NotEqual(t, foreign.ID, entry.Record.ID)
			}
		}
	}
	batch, err := module.ListStudentRosterOnDates(otherCtx, dates, "2026-09-08")
	require.NoError(t, err)
	for _, date := range dates {
		require.Len(t, batch[date], 1)
		assert.Equal(t, foreign.ID, batch[date][0].Record.ID)
	}
}
