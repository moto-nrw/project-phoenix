package compose_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/appointments"
	calendarSvc "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
	calendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/compose"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// CalendarFeedByToken (ADR 0161 step 1, #5881) must answer like the order the
// public route used before: the standalone parent feed, then the standalone
// staff feed on a miss. Its statements are the same minus the COMMIT, BEGIN
// and scope statement between the two token reads, in every entry: no
// transaction, a school in the context, and an open administrative, same
// school or other school transaction (ADR 0161 point 7).

type feedCall func(context.Context, string) (string, string, error)

func legacyPublicFeed(service calendarCompose.Application) feedCall {
	return func(ctx context.Context, token string) (string, string, error) {
		filename, content, err := service.ParentCalendarFeedByToken(ctx, token)
		if errors.Is(err, calendarSvc.ErrNotFound) {
			return service.StaffCalendarFeedByToken(ctx, token)
		}
		return filename, content, err
	}
}

type feedOutcome struct {
	filename, content, err string
	queries                []string
}

func observeFeed(t *testing.T, db *bun.DB, call feedCall, ctx context.Context, token string) feedOutcome {
	t.Helper()
	counter := testpkg.CaptureQueries(t, db)
	filename, content, err := call(ctx, token)
	queries := counter.Queries()
	counter.Stop()
	outcome := feedOutcome{filename: filename, content: content, queries: queries}
	if err != nil {
		outcome.err = err.Error()
	}
	return outcome
}

func feedQueryIndex(queries []string, match func(string) bool) int {
	for i, q := range queries {
		if match(q) {
			return i
		}
	}
	return -1
}

func isParentFeedTokenRead(q string) bool {
	return strings.Contains(q, "FROM auth.accounts WHERE calendar_feed_token =")
}

func isStaffFeedTokenRead(q string) bool {
	return strings.Contains(q, "FROM auth.account_tenants WHERE staff_calendar_feed_token =")
}

// withoutDiscoveryBoundary removes the COMMIT, BEGIN and scope statement
// that separated the parent and the staff token read before the bundle.
func withoutDiscoveryBoundary(queries []string) []string {
	parent, staff := feedQueryIndex(queries, isParentFeedTokenRead), feedQueryIndex(queries, isStaffFeedTokenRead)
	if parent < 0 || staff < 0 {
		return queries
	}
	for i := parent + 1; i+2 < staff; i++ {
		if queries[i] == "COMMIT" && queries[i+1] == "BEGIN" {
			return append(append([]string{}, queries[:i]...), queries[i+3:]...)
		}
	}
	return queries
}

func TestCalendarFeedByTokenMatchesTheStandaloneFeeds(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)
	repos, reposPeople, reposCalendar, _, _, _, _ := calendarTestRepositories(t, db)
	cfg := calendarTestConfig(t, db)
	cfg = owners.BindCalendarFeedFacts(cfg, nil, repos.ParentCalendarFeed, repos.StaffCalendarFeedToken, reposPeople.Person)
	cfg.StaffFeedTombstoneRepo = reposCalendar.CalendarStaffFeedTombstone
	cfg.ParentsURL = "https://parents.test"
	cfg.FrontendURL = "https://moto.test"
	service := owners.NewCalendarPortal(cfg)

	var staffToken, parentToken string
	var schoolB int64
	schoolA := testpkg.Tenant(t)
	{
		_, organizer := testpkg.CreateTestCalendarStaff(t, db, "Feed", "Equal")
		chain := testpkg.CreateTestParentGuardianChain(t, db)
		_, err := testpkg.TenantQuery1(t, calendarContext(t, organizer.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
			Title: "Termin gleich", StartDate: portalDate(2026, 8, 24).AddDays(7), EndDate: portalDate(2026, 8, 24).AddDays(7),
			StartTime: wallClock(15, 0), EndTime: wallClock(16, 0), DeliveryMode: appointments.DeliveryModeInformational,
			Targets: []calendarSvc.AppointmentTarget{{Type: appointments.TargetTypeGuardianProfile, ID: &chain.GuardianProfileID}},
		})
		require.NoError(t, err)
		staffURL, _, err := testpkg.TenantQueryPair0(t, calendarContext(t, organizer.ID), db, service.StaffCalendarFeedURL)
		require.NoError(t, err)
		staffToken = strings.TrimPrefix(staffURL, "https://moto.test/api/calendar-feed/")
		parentURL, _, err := service.ParentCalendarFeedURL(testpkg.Ctx(t), chain.AccountID)
		require.NoError(t, err)
		parentToken = strings.TrimPrefix(parentURL, "https://parents.test/api/calendar-feed/")
	}
	t.Run("school B", func(t *testing.T) { schoolB = testpkg.OwnTenant(t) })
	require.NotEqual(t, schoolA, schoolB)

	noSchool := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	inSchool := func(id int64) context.Context { return testpkg.ContextForTenant(noSchool, id) }
	inTenantTx := func(id int64) func(func(context.Context)) error {
		return func(fn func(context.Context)) error {
			return testpkg.WithinTenantContext(t, noSchool, db, id, func(txCtx context.Context) error { fn(txCtx); return nil })
		}
	}
	entries := map[string]func(func(context.Context)) error{
		"no transaction": func(fn func(context.Context)) error { fn(noSchool); return nil },
		"school context": func(fn func(context.Context)) error { fn(inSchool(schoolA)); return nil },
		"admin transaction": func(fn func(context.Context)) error {
			return testpkg.WithinAdminContext(t, noSchool, db, func(txCtx context.Context) error { fn(txCtx); return nil })
		},
		"same school transaction":  inTenantTx(schoolA),
		"other school transaction": inTenantTx(schoolB),
	}
	for entry, run := range entries {
		for name, token := range map[string]string{"staff": staffToken, "parent": parentToken, "unknown": "kein-bekannter-token"} {
			t.Run(entry+"/"+name, func(t *testing.T) {
				var before, after feedOutcome
				beforeErr := run(func(ctx context.Context) { before = observeFeed(t, db, legacyPublicFeed(service), ctx, token) })
				afterErr := run(func(ctx context.Context) { after = observeFeed(t, db, service.CalendarFeedByToken, ctx, token) })
				t.Logf("feed err=%q; statements %d -> %d; vevents=%d", after.err, len(before.queries), len(after.queries), strings.Count(after.content, "BEGIN:VEVENT"))
				require.Equal(t, fmt.Sprint(beforeErr), fmt.Sprint(afterErr))
				require.Equal(t, before.err, after.err)
				require.Equal(t, before.filename, after.filename)
				require.Equal(t, before.content, after.content)
				require.Equal(t, withoutDiscoveryBoundary(before.queries), after.queries)
				if name != "unknown" || after.err != "" {
					return
				}
				t.Fatalf("an unknown token was served: %q", after.content)
			})
		}
	}

	// A parent hit is the standalone parent feed: same answer, same
	// statements, one token read and no staff token read.
	before := observeFeed(t, db, service.ParentCalendarFeedByToken, noSchool, parentToken)
	after := observeFeed(t, db, service.CalendarFeedByToken, noSchool, parentToken)
	require.Empty(t, after.err)
	require.Contains(t, after.content, "SUMMARY:Termin gleich")
	require.Equal(t, before, after)
	parentReads := 0
	for _, q := range after.queries {
		if isParentFeedTokenRead(q) {
			parentReads++
		}
	}
	require.Equal(t, 1, parentReads, "%q", after.queries)
	require.Equal(t, -1, feedQueryIndex(after.queries, isStaffFeedTokenRead), "%q", after.queries)
}
