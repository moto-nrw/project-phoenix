package calendar_e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	rootAPI "github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	calendarHTTP "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/http"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The public feed contract of ADR 0161 step 1 (#5881), observed over HTTP on
// a real database. Behaviour assertions (status, headers, body, school, order
// of the token reads, statement after a failure) hold on the code before the
// bundle as well; frame-shape assertions name the bundled unit explicitly.

type publicFeedRig struct {
	db     *bun.DB
	router chi.Router
	module rootAPI.CalendarTestModule
}

func newPublicFeedRig(t *testing.T) publicFeedRig {
	t.Helper()
	testpkg.SetupIsolatedTestDB(t)
	db, module := setupCalendarModule(t)
	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	router.Mount("/calendar", calendarHTTP.NewResource(module.Calendar, slog.Default()).Router(rootAPI.TenantSessionChain))
	router.With(rootAPI.RequestSettingsCacheMiddleware, rootAPI.RequestIdentityCacheMiddleware).Mount("/parent", module.ParentCalendarRouter())
	router.Get("/public/calendar/{token}", calendarHTTP.PublicFeed(module.Calendar))
	return publicFeedRig{db: db, router: router, module: module}
}

// staffFeed creates a staff member of t's school with one appointment titled
// title and returns the raw token of the staff feed and the account ID.
func (r publicFeedRig) staffFeed(t *testing.T, last, title string) (string, int64) {
	t.Helper()
	_, account := testpkg.CreateTestCalendarStaff(t, r.db, "Feed", last)
	jwt := calendarToken(t, account.ID, permissions.CalendarOwn, permissions.CalendarManage)
	rec := doJSON(t, r.router, http.MethodPost, "/calendar/appointments", jwt, map[string]any{
		"title": title, "start_date": "2026-01-08", "end_date": "2026-01-08", "start_time": "14:00", "end_time": "15:00", "delivery_mode": "informational",
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	return feedTokenOf(t, doJSON(t, r.router, http.MethodGet, "/calendar/feed", jwt, nil)), account.ID
}

func feedTokenOf(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var feed feedE2EResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &feed))
	token := feed.Data.URL[strings.LastIndexByte(feed.Data.URL, '/')+1:]
	require.NotEmpty(t, token)
	return token
}

// fetch serves the public feed and returns the response and its statements.
func (r publicFeedRig) fetch(t *testing.T, token string) (*httptest.ResponseRecorder, []string) {
	t.Helper()
	counter := testpkg.CaptureQueries(t, r.db)
	rec := httptest.NewRecorder()
	r.router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/public/calendar/"+token, nil))
	queries := counter.Queries()
	counter.Stop()
	return rec, queries
}

func isParentTokenRead(q string) bool {
	return strings.Contains(q, "FROM auth.accounts WHERE calendar_feed_token =")
}

func isStaffTokenRead(q string) bool {
	return strings.Contains(q, "FROM auth.account_tenants WHERE staff_calendar_feed_token =")
}

func isTenantSwitch(q string) bool {
	return strings.Contains(q, "set_config('app.current_tenant_id'")
}

func indexOf(queries []string, match func(string) bool) int {
	for i, q := range queries {
		if match(q) {
			return i
		}
	}
	return -1
}

func countOf(queries []string, match func(string) bool) int {
	n := 0
	for _, q := range queries {
		if match(q) {
			n++
		}
	}
	return n
}

func isAdminRole(q string) bool { return q == "SET LOCAL ROLE phoenix_admin" }

// discoveryUnit returns the statements before the first tenant switch.
func discoveryUnit(queries []string) []string {
	if end := indexOf(queries, isTenantSwitch); end >= 0 {
		return queries[:end]
	}
	return queries
}

func notFoundWire(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	return fmt.Sprintf("%d %v %s", rec.Code, rec.Header(), rec.Body.String())
}

func TestPublicCalendarFeedDiscoveryKeepsSchoolAndRefusals(t *testing.T) {
	t.Parallel()
	rig := newPublicFeedRig(t)

	var tokenA, tokenB string
	var schoolA, schoolB int64
	t.Run("school A", func(t *testing.T) {
		schoolA = testpkg.OwnTenant(t)
		tokenA, _ = rig.staffFeed(t, "SchoolA", "Termin Schule A")
	})
	t.Run("school B", func(t *testing.T) {
		schoolB = testpkg.OwnTenant(t)
		tokenB, _ = rig.staffFeed(t, "SchoolB", "Termin Schule B")
	})
	require.NotEqual(t, schoolA, schoolB)

	for _, tc := range []struct {
		token        string
		school       int64
		own, foreign string
	}{{tokenA, schoolA, "Termin Schule A", "Termin Schule B"}, {tokenB, schoolB, "Termin Schule B", "Termin Schule A"}} {
		rec, queries := rig.fetch(t, tc.token)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.Equal(t, "text/calendar; charset=utf-8", rec.Header().Get("Content-Type"))
		require.Equal(t, `inline; filename="moto-kalender.ics"`, rec.Header().Get("Content-Disposition"))
		require.Equal(t, "private, max-age=3600", rec.Header().Get("Cache-Control"))
		require.Contains(t, rec.Body.String(), "SUMMARY:"+tc.own)
		require.NotContains(t, rec.Body.String(), tc.foreign)
		// The parent token is read first, the staff token only after its miss,
		// both before the school's projection; that projection runs for the
		// token's school only.
		parent, staff, school := indexOf(queries, isParentTokenRead), indexOf(queries, isStaffTokenRead), indexOf(queries, isTenantSwitch)
		require.True(t, parent >= 0 && parent < staff && staff < school, "order: %q", queries)
		require.Contains(t, queries[school], fmt.Sprintf("'app.current_tenant_id', '%d'", tc.school))
		require.Equal(t, 1, countOf(queries, isTenantSwitch), "one school projection: %q", queries)
	}

	// Unknown, rotated and inactive tokens cannot be told apart from outside.
	var rotated, inactive, leftSchool string
	t.Run("refused tokens", func(t *testing.T) {
		testpkg.OwnTenant(t)
		var accountID int64
		rotated, accountID = rig.staffFeed(t, "Rotated", "Termin rotiert")
		jwt := calendarToken(t, accountID, permissions.CalendarOwn, permissions.CalendarManage)
		require.NotEqual(t, rotated, feedTokenOf(t, doJSON(t, rig.router, http.MethodPost, "/calendar/feed/rotate", jwt, nil)))
		inactive, accountID = rig.staffFeed(t, "Inactive", "Termin inaktiv")
		_, err := rig.db.NewRaw("UPDATE auth.accounts SET active = false WHERE id = ?", accountID).Exec(context.Background())
		require.NoError(t, err)
		leftSchool, accountID = rig.staffFeed(t, "Left", "Termin ausgetreten")
		_, err = rig.db.NewRaw("UPDATE auth.account_tenants SET status = 'inactive' WHERE account_id = ? AND tenant_id = ?", accountID, testpkg.Tenant(t)).Exec(context.Background())
		require.NoError(t, err)
	})
	unknownRec, _ := rig.fetch(t, "kein-bekannter-token")
	unknown := notFoundWire(t, unknownRec)
	var body map[string]any
	require.NoError(t, json.Unmarshal(unknownRec.Body.Bytes(), &body))
	require.Equal(t, "general.input", body["code"])
	require.EqualValues(t, http.StatusNotFound, body["status"])
	for name, token := range map[string]string{"rotated": rotated, "inactive account": inactive, "left school": leftSchool} {
		rec, queries := rig.fetch(t, token)
		require.Equal(t, unknown, notFoundWire(t, rec), name)
		require.Equal(t, -1, indexOf(queries, isTenantSwitch), "%s reaches no school: %q", name, queries)
	}
}

// TestPublicCalendarFeedDiscoveryIsOneAdministrativeUnit pins the bundle:
// both token reads share one administrative unit; a miss ends it with
// ROLLBACK, a staff hit commits it before the school's own unit.
func TestPublicCalendarFeedDiscoveryIsOneAdministrativeUnit(t *testing.T) {
	t.Parallel()
	rig := newPublicFeedRig(t)
	token, _ := rig.staffFeed(t, "Unit", "Termin Einheit")

	rec, queries := rig.fetch(t, token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	discovery := discoveryUnit(queries)
	require.Equal(t, 1, countOf(discovery, isAdminRole), "one administrative unit: %q", discovery)
	require.Equal(t, "BEGIN", discovery[0])
	require.Equal(t, "COMMIT", discovery[len(discovery)-2], "discovery commits before the school unit: %q", discovery)
	require.Equal(t, "BEGIN", discovery[len(discovery)-1])

	_, queries = rig.fetch(t, "kein-bekannter-token")
	require.Len(t, queries, 5, "%q", queries)
	require.Equal(t, "BEGIN", queries[0])
	require.True(t, isAdminRole(queries[1]), "%q", queries)
	require.True(t, isParentTokenRead(queries[2]), "%q", queries)
	require.True(t, isStaffTokenRead(queries[3]), "%q", queries)
	require.Equal(t, "ROLLBACK", queries[4])

	// The bundled unit leaves no role or school on the pooled connection,
	// after a commit as after a rollback.
	sqlDB := rig.db
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetMaxOpenConns(1)
	var loginRole string
	require.NoError(t, rig.db.NewRaw("SELECT current_user").Scan(context.Background(), &loginRole))
	for _, feedToken := range []string{token, "kein-bekannter-token"} {
		rig.fetch(t, feedToken)
		var role, school string
		require.NoError(t, rig.db.NewRaw("SELECT current_user, coalesce(current_setting('app.current_tenant_id', true), '')").Scan(context.Background(), &role, &school))
		require.Equal(t, loginRole, role)
		require.Empty(t, school)
	}
}

// TestPublicCalendarFeedDatabaseFaults injects a real database failure into
// each part of the request. Every part answers 500 like before, and no
// statement but ROLLBACK follows the failing one.
func TestPublicCalendarFeedDatabaseFaults(t *testing.T) {
	t.Parallel()
	rig := newPublicFeedRig(t)
	token, _ := rig.staffFeed(t, "Fault", "Termin Fehler")

	for _, fault := range []struct {
		name, table, role string
		failing           func(string) bool
		staffRead         bool
	}{
		{"parent token read", "auth.accounts", "phoenix_admin", isParentTokenRead, false},
		{"staff token read", "auth.account_tenants", "phoenix_admin", isStaffTokenRead, true},
		{"school projection", "users.persons", "phoenix_tenant", func(q string) bool {
			return strings.Contains(q, `"users"."persons"`) || strings.Contains(q, "users.persons")
		}, true},
	} {
		t.Run(fault.name, func(t *testing.T) {
			ctx := context.Background()
			_, err := rig.db.NewRaw("REVOKE SELECT ON " + fault.table + " FROM " + fault.role).Exec(ctx)
			require.NoError(t, err)
			rec, queries := rig.fetch(t, token)
			_, err = rig.db.NewRaw("GRANT SELECT ON " + fault.table + " TO " + fault.role).Exec(ctx)
			require.NoError(t, err)

			require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
			var body map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			require.Equal(t, "general.server", body["code"])
			failed := indexOf(queries, fault.failing)
			require.GreaterOrEqual(t, failed, 0, "%q", queries)
			require.Equal(t, []string{"ROLLBACK"}, queries[failed+1:], "%q", queries)
			require.Equal(t, fault.staffRead, indexOf(queries, isStaffTokenRead) >= 0, "staff fallback after %s: %q", fault.name, queries)

			ok, _ := rig.fetch(t, token)
			require.Equal(t, http.StatusOK, ok.Code, "grant restored")
		})
	}
}

// TestPublicCalendarFeedParentHitReadsTheTokenOnce: a parent token is served
// as the parent feed, its token read once and no staff token read.
func TestPublicCalendarFeedParentHitReadsTheTokenOnce(t *testing.T) {
	t.Parallel()
	rig := newPublicFeedRig(t)
	_, organizer := testpkg.CreateTestCalendarStaff(t, rig.db, "Feed", "Organizer")
	chain := testpkg.CreateTestParentGuardianChain(t, rig.db)
	jwt := calendarToken(t, organizer.ID, permissions.CalendarManage, permissions.CalendarOwn)
	created := doJSON(t, rig.router, http.MethodPost, "/calendar/appointments", jwt, map[string]any{
		"title": "Elternabend Feed", "start_date": "2026-01-08", "end_date": "2026-01-08", "start_time": "18:00", "end_time": "19:00", "delivery_mode": "informational",
		"targets": []map[string]any{{"type": "guardian_profile", "id": chain.GuardianProfileID}},
	})
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	token := feedTokenOf(t, doJSON(t, rig.router, http.MethodGet, "/parent/me/calendar/feed", parentCalendarToken(t, chain.AccountID), nil))

	rec, queries := rig.fetch(t, token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, `inline; filename="moto-kalender.ics"`, rec.Header().Get("Content-Disposition"))
	require.Contains(t, rec.Body.String(), "SUMMARY:Elternabend Feed")
	require.Equal(t, 1, countOf(queries, isParentTokenRead), "%q", queries)
	require.Equal(t, -1, indexOf(queries, isStaffTokenRead), "%q", queries)
}
