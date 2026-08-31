// Router-level tests for the tenant-wide time-tracking views (#1417):
// GET /api/staff/dashboard-summary and GET /api/staff/time-tracking/overview,
// plus the Monatsabschluss routes. They pin the permission split, the query
// validation, and the response field names the frontend will consume.
package staff_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// overviewAPIContext wires a router plus an editor account whose permissions the
// individual test picks.
type overviewAPIContext struct {
	tc      *testContext
	staffID int64
	get     func(path string, perms ...string) *httptest.ResponseRecorder
	post    func(path, body string, perms ...string) *httptest.ResponseRecorder
	put     func(path, body string, perms ...string) *httptest.ResponseRecorder
}

func setupOverviewAPI(t *testing.T) *overviewAPIContext {
	t.Helper()
	tc := setupStaffRoute(t)
	suffix := time.Now().UnixNano()

	person, account := testpkg.CreateTestPersonWithAccount(t, tc.db, "Overview", fmt.Sprintf("Editor-%d", suffix))
	editorStaff := testpkg.CreateTestStaffForPerson(t, tc.db, person.ID)

	token := func(perms ...string) string {
		claims := testutil.DefaultTestClaims()
		claims.ID = int(account.ID)
		claims.Permissions = perms
		return testutil.MintTestJWT(t, claims)
	}

	return &overviewAPIContext{
		tc:      tc,
		staffID: editorStaff.ID,
		get: func(path string, perms ...string) *httptest.ResponseRecorder {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Authorization", "Bearer "+token(perms...))
			rec := httptest.NewRecorder()
			tc.router.ServeHTTP(rec, req)
			return rec
		},
		post: func(path, body string, perms ...string) *httptest.ResponseRecorder {
			req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
			req.Header.Set("Authorization", "Bearer "+token(perms...))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			tc.router.ServeHTTP(rec, req)
			return rec
		},
		put: func(path, body string, perms ...string) *httptest.ResponseRecorder {
			req := httptest.NewRequest(http.MethodPut, path, bytes.NewBufferString(body))
			req.Header.Set("Authorization", "Bearer "+token(perms...))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			tc.router.ServeHTTP(rec, req)
			return rec
		},
	}
}

func TestDashboardSummaryAPI(t *testing.T) {
	t.Parallel()

	ctx := setupOverviewAPI(t)

	rec := ctx.get("/staff/dashboard-summary", "users:read")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	// Field names are fixed by #1417; a rename must break CI.
	for _, field := range []string{
		`"active_staff_count"`, `"sick_today"`, `"vacation_today"`,
		`"currently_clocked_in"`, `"expected_clocked_in"`, `"period"`,
		`"soll_minutes"`, `"ist_minutes"`, `"delta_minutes"`,
		`"saldo_school_total_minutes"`, `"pending_requests_count"`,
	} {
		assert.Contains(t, rec.Body.String(), field)
	}

	rec = ctx.get("/staff/dashboard-summary?period=week", "users:read")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = ctx.get("/staff/dashboard-summary?period=quarter", "users:read")
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	rec = ctx.get("/staff/dashboard-summary", "feedback:read")
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
}

// TestTimeTrackingOverviewAPI_PermissionSplit is the DSGVO-relevant assertion:
// per-person Soll/Ist/Saldo needs time_tracking:manage, NOT the users:read that
// merely lets someone see the staff list.
func TestTimeTrackingOverviewAPI_PermissionSplit(t *testing.T) {
	t.Parallel()

	ctx := setupOverviewAPI(t)

	rec := ctx.get("/staff/time-tracking/overview", "users:read")
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	rec = ctx.get("/staff/time-tracking/overview", "time_tracking:manage")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	for _, field := range []string{`"rows"`, `"year"`, `"month"`} {
		assert.Contains(t, rec.Body.String(), field)
	}
}

func TestTimeTrackingOverviewAPI_FilterValidation(t *testing.T) {
	t.Parallel()

	ctx := setupOverviewAPI(t)

	for _, query := range []string{
		"?saldo_min=abc",
		"?employment_type=bogus",
		"?sort=bogus",
		"?order=sideways",
		"?saldo_min=100&saldo_max=10",
		"?month=3",   // month without year
		"?year=2026", // year without month
		"?year=abc&month=3",
		"?year=0&month=0",
		"?year=0&month=1",
		"?year=2026&month=0",
		"?year=2026&month=13",
		"?year=2999&month=1", // future
	} {
		rec := ctx.get("/staff/time-tracking/overview"+query, "time_tracking:manage")
		assert.Equal(t, http.StatusBadRequest, rec.Code, "query %s: %s", query, rec.Body.String())
	}

	// Valid combinations pass.
	for _, query := range []string{
		"?employment_type=full_time",
		"?saldo_min=-600&saldo_max=600",
		"?sort=balance&order=desc",
		"?year=2025&month=1",
	} {
		rec := ctx.get("/staff/time-tracking/overview"+query, "time_tracking:manage")
		assert.Equal(t, http.StatusOK, rec.Code, "query %s: %s", query, rec.Body.String())
	}
}

// TestMonthCloseAPI covers the Monatsabschluss routes end to end through the
// router, including the guard against closing a month that is not over.
func TestMonthCloseAPI(t *testing.T) {
	t.Parallel()

	ctx := setupOverviewAPI(t)
	today := timezone.TodayDate()

	rec := ctx.get(fmt.Sprintf("/staff/time-tracking/month-close?year=%d&month=%d", today.Year(), int(today.Month())), "time_tracking:manage")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = ctx.get("/staff/time-tracking/month-close?year=2026&month=13", "time_tracking:manage")
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	rec = ctx.get("/staff/time-tracking/month-close?year=2026&month=6", "users:read")
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	// The current month is not over: 400, not a silently wrong snapshot.
	body := fmt.Sprintf(`{"year":%d,"month":%d,"reason":"Abschluss"}`, today.Year(), int(today.Month()))
	rec = ctx.post("/staff/time-tracking/month-close", body, "time_tracking:manage")
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	// A missing reason is a 400 too — freezing has to stay attributable.
	rec = ctx.post("/staff/time-tracking/month-close", `{"year":2025,"month":8,"reason":"  "}`, "time_tracking:manage")
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	// Reopening a month that was never closed is a 404.
	rec = ctx.post(fmt.Sprintf("/staff/%d/time-tracking/month-close/reopen", ctx.staffID),
		`{"year":2025,"month":8,"reason":"Korrektur"}`, "time_tracking:manage")
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}

// TestMonthCloseAPI_StableErrorCodes pins the machine-readable codes the
// frontend branches on for its German copy (#1417 UI). The wire strings are
// English; the codes are the contract.
func TestMonthCloseAPI_StableErrorCodes(t *testing.T) {
	t.Parallel()

	ctx := setupOverviewAPI(t)
	today := timezone.TodayDate()

	// Closing the current month: it is not over yet.
	body := fmt.Sprintf(`{"year":%d,"month":%d,"reason":"Abschluss"}`, today.Year(), int(today.Month()))
	rec := ctx.post("/staff/time-tracking/month-close", body, "time_tracking:manage")
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"code":"month_not_closable"`)

	// Reopening a month that was never closed.
	rec = ctx.post(fmt.Sprintf("/staff/%d/time-tracking/month-close/reopen", ctx.staffID),
		`{"year":2025,"month":8,"reason":"Korrektur"}`, "time_tracking:manage")
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"code":"month_not_closed"`)
}
