package compose_test

import (
	"fmt"
	"testing"

	calendarSvc "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

// Only the number of visible appointments grows. Tenants, dates, staff,
// assignments and the CalDAV feature setting remain unchanged.
func TestCalendarCalDAVSnapshotPositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)
	repos, people, calendarRows, _, _, _, _ := calendarTestRepositories(t, db)
	cfg := calendarTestConfig(t, db)
	cfg = owners.BindCalendarFeedFacts(cfg, nil, repos.ParentCalendarFeed, repos.StaffCalendarFeedToken, people.Person)
	cfg.StaffFeedTombstoneRepo = calendarRows.CalendarStaffFeedTombstone
	cfg.CalDAVPolicy = fixedCalDAVSettings{enabled: true}
	cfg.FrontendURL = "https://moto.test"
	cfg.CalDAVURL = "https://api.moto.test"
	service := owners.NewCalendarPortal(cfg)
	_, account := testpkg.CreateTestCalendarStaff(t, db, "Growth", "CalDAV")
	ctx := calendarContext(t, account.ID)
	access, err := testpkg.TenantQuery0(t, ctx, db, service.StaffCalendarAccess)
	require.NoError(t, err)
	require.NotNil(t, access.CalDAV)
	count := 0
	for _, n := range []int{3, 8} {
		for count < n {
			_, err := testpkg.TenantQuery1(t, ctx, db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
				Title: fmt.Sprintf("Growth event %d", count), StartDate: portalDate(2026, 1, 8), EndDate: portalDate(2026, 1, 8),
				StartTime: wallClock(14, 0), EndTime: wallClock(15, 0), DeliveryMode: "informational",
			})
			require.NoError(t, err)
			count++
		}
		counter := testpkg.CaptureQueriesForContext(t, db)
		snapshot, err := service.AuthenticateStaffCalDAV(counter.Context(testpkg.Ctx(t)), account.Email, access.CalDAV.AppPassword)
		require.NoError(t, err)
		require.Len(t, snapshot.Items, n)
		require.Equal(t, testpkg.Tenant(t), snapshot.TenantID)
		for _, item := range snapshot.Items {
			require.Contains(t, string(item.Content), "SUMMARY:Growth event")
			require.Contains(t, string(item.Content), "UID:"+item.UID)
		}
		t.Logf("CalDAV appointments N%d statements=%d", n, len(counter.Queries()))
		querybudgets.AssertQueryBudget(t, "services.calendar.caldav_snapshot", counter.Queries())
		counter.Stop()

	}
}
