// Router-level tests for the evaluation of overruns over the schools of a
// Träger (#4683, ADR 0156): the figures per school with "Berichte lesen",
// the children by name only with "Verträge verwalten", the Soll from the
// sent contract, the Träger boundary and the access log of every named read.
package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

type overrunFiguresBody struct {
	Children     int `json:"children"`
	OverrunDays  int `json:"overrun_days"`
	UnbookedDays int `json:"unbooked_days"`
	ExtraMinutes int `json:"extra_minutes"`
}

type overrunEvaluationBody struct {
	ToleranceMinutes int                `json:"tolerance_minutes"`
	Total            overrunFiguresBody `json:"total"`
	Schools          []struct {
		SchoolID       int64              `json:"school_id"`
		ComparedDays   int                `json:"compared_days"`
		UncomparedDays int                `json:"uncompared_days"`
		Figures        overrunFiguresBody `json:"figures"`
	} `json:"schools"`
	Children []struct {
		SchoolID     int64  `json:"school_id"`
		StudentID    int64  `json:"student_id"`
		FirstName    string `json:"first_name"`
		LastName     string `json:"last_name"`
		ExtraMinutes int    `json:"extra_minutes"`
	} `json:"children"`
}

type overrunFixture struct {
	signInRole func(*testing.T, string) string
	router     http.Handler
	db         *bun.DB
	traeger    *traegerPortalHandler
	school     int64
	other      int64
	child      int64
	admin      string
	billing    string
	adminID    int64
	foreignJWT string
}

// newOverrunFixture is a Träger with two schools. The per-test school holds
// the child of careTimeChild with a sent contract: Monday until 13:00, no
// Tuesday. The care plan says Monday 08:00 to 14:00.
func newOverrunFixture(t *testing.T) overrunFixture {
	t.Helper()
	return newOverrunFixtureOn(t, testpkg.SetupTestDB(t))
}

// newOverrunFixtureOn builds the fixture on db; a test that counts
// statements passes its own isolated database.
func newOverrunFixtureOn(t *testing.T, db *bun.DB) overrunFixture {
	t.Helper()
	unit := testpkg.TenantRuntime(t, db)
	traeger, _ := traegerPortalOn(t, db, unit)
	organization := traegerOrganization(t, db, "Träger Zeitvergleich", true)
	testpkg.OwnTraegerCareContracts(t, db, organization)
	school := testpkg.Tenant(t)
	// The per-test school joins the Träger for the test and leaves it before
	// the organization's cleanup, which would otherwise take the school's rows
	// with it while the tenant cleanup still owns them.
	var previous *int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, school).Scan(context.Background(), &previous))
	_, err := db.ExecContext(context.Background(), `UPDATE platform.schools SET organization_id = ? WHERE id = ?`, organization, school)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := db.ExecContext(context.Background(), `UPDATE platform.schools SET organization_id = ? WHERE id = ?`, previous, school)
		require.NoError(t, err)
	})
	other := testpkg.CreateTestSchoolInOrganization(t, db, organization, "Träger Zeitvergleich OGS Süd")

	staff, _ := testpkg.CreateTestStaffWithAccountForTenant(t, db, school, "Zeit", "Vergleich")
	child := careTimeChild(t, &statisticsHTTPTestContext{db: db}, staff.ID)
	testpkg.CreateTestCareContract(t, db, organization, school, child, "versendet", calendar.NewDate(2026, 6, 1),
		testpkg.CareContractService{Weekday: "mon", Module: "OGS", EndsAt: "13:00"})

	signIn := func(role string) (string, int64) {
		t.Helper()
		password := testpkg.NewTestPassword()
		account := traegerAccount(t, db, password)
		testpkg.CreateTestTraegerMember(t, db, organization, account.ID, role)
		code, answer := traegerSignIn(t, traeger, account.Email, password, 0)
		require.Equal(t, http.StatusOK, code)
		return answer.AccessToken, account.ID
	}
	admin, adminID := signIn(identityaccess.TraegerRoleAdministration)
	signInRole := func(t *testing.T, role string) string {
		t.Helper()
		token, _ := signIn(role)
		return token
	}
	billing, _ := signIn(identityaccess.TraegerRoleBilling)
	foreign := newTraegerOffice(t, db, traeger, "Träger Zeitvergleich Fremd", identityaccess.TraegerRoleAdministration)

	statistics, err := NewStatisticsTestModule(testpkg.NewDatabase(t, db), unit)
	require.NoError(t, err)
	organizations, err := owners.NewOrganizationTenancy(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	audit, err := auditCompose.NewTenantPersistence(db)
	require.NoError(t, err)
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(tenant.WithUnitOfWork(r.Context(), unit)))
		})
	})
	mountCareTimeOverrunRoutes(router, organizations, statistics.Statistics, audit.DataAccessLog)
	return overrunFixture{signInRole: signInRole, router: testpkg.SessionVerifier(router), db: db, traeger: traeger, school: school, other: other, child: child,
		admin: admin, billing: billing, adminID: adminID, foreignJWT: foreign.token}
}

func (f overrunFixture) signIn(t *testing.T, role string) string {
	t.Helper()
	return f.signInRole(t, role)
}

const overrunWindow = "/traeger/care-time-overruns?from=2026-06-08&to=2026-06-16"

func (f overrunFixture) get(t *testing.T, token, path string) (int, overrunEvaluationBody, string) {
	t.Helper()
	rec := traegerJSON(t, f.router, token, http.MethodGet, path, nil)
	if rec.Code != http.StatusOK {
		return rec.Code, overrunEvaluationBody{}, rec.Body.String()
	}
	return rec.Code, decodeSchoolPortalData[overrunEvaluationBody](t, rec.Body.Bytes()), rec.Body.String()
}

func TestCareTimeOverrunsSummariseEverySchoolFromTheContract(t *testing.T) {
	t.Parallel()
	f := newOverrunFixture(t)
	// A child without any care plan or contract checks in once: the day
	// cannot be compared and must not count as compared without overrun.
	unplanned := testpkg.CreateTestStudent(t, f.db, "Nora", "Ohneplan", "2a")
	device := testpkg.CreateTestDevice(t, f.db, "overrun-unplanned-device")
	insertAttendanceSpan(t, f.db, f.school, unplanned.ID, device.ID, calendar.NewDate(2026, 6, 10), 8*time.Hour, 15*time.Hour)
	code, body, raw := f.get(t, f.billing, overrunWindow)
	require.Equal(t, http.StatusOK, code, raw)
	// 8 June: 60 minutes after the contract's 13:00. 9 June: a Tuesday the
	// contract does not book, 360 minutes. 15 June: an hour early against
	// the care plan's arrival and an hour after 13:00.
	want := overrunFiguresBody{Children: 1, OverrunDays: 2, UnbookedDays: 1, ExtraMinutes: 60 + 360 + 120}
	assert.Equal(t, want, body.Total)
	assert.Equal(t, 15, body.ToleranceMinutes)
	figures := map[int64]overrunFiguresBody{}
	attended := map[int64]int{}
	uncompared := map[int64]int{}
	for _, school := range body.Schools {
		figures[school.SchoolID] = school.Figures
		attended[school.SchoolID] = school.ComparedDays
		uncompared[school.SchoolID] = school.UncomparedDays
	}
	assert.Equal(t, want, figures[f.school])
	assert.Equal(t, 3, attended[f.school], "the booked child's three check-in days were compared")
	assert.Equal(t, 1, uncompared[f.school], "the day without a plan is attended but not compared")
	assert.Equal(t, overrunFiguresBody{}, figures[f.other], "the other school is listed with nothing")
	assert.Zero(t, attended[f.other], "no check-in at the other school: its zeros are missing data, not an all-clear")
	assert.Empty(t, body.Children, "the summary names no child")
	assert.NotContains(t, raw, "Sina")
}

func TestCareTimeOverrunChildrenNeedTheContractRightAndAreLogged(t *testing.T) {
	t.Parallel()
	f := newOverrunFixture(t)
	// The Leitung of the Träger holds no right "Berichte lesen": neither the
	// figures nor the names.
	leadership := f.signIn(t, identityaccess.TraegerRoleLeadership)
	for _, path := range []string{overrunWindow, "/traeger/care-time-overruns/children?from=2026-06-08&to=2026-06-16"} {
		code, _, raw := f.get(t, leadership, path)
		assert.Equal(t, http.StatusForbidden, code, "%s: %s", path, raw)
	}
	code, _, raw := f.get(t, f.billing, "/traeger/care-time-overruns/children?from=2026-06-08&to=2026-06-16")
	assert.Equal(t, http.StatusForbidden, code, "Abrechnung holds no right \"Verträge verwalten\": %s", raw)

	code, body, raw := f.get(t, f.admin, "/traeger/care-time-overruns/children?from=2026-06-08&to=2026-06-16")
	require.Equal(t, http.StatusOK, code, raw)
	require.Len(t, body.Children, 1)
	assert.Equal(t, f.child, body.Children[0].StudentID)
	assert.Equal(t, "Sina", body.Children[0].FirstName)
	assert.Equal(t, 540, body.Children[0].ExtraMinutes)

	var logged []struct {
		Metadata map[string]any `bun:"metadata,type:jsonb"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT metadata FROM audit.data_access_log
		WHERE actor_account_id = ? AND resource_type = 'care_time_overruns'`, f.adminID).Scan(context.Background(), &logged))
	require.Len(t, logged, 1, "one entry for the one school that showed a child")
	assert.Equal(t, "traeger_portal", logged[0].Metadata["channel"])
	assert.Equal(t, []any{float64(f.child)}, logged[0].Metadata["student_ids"])
}

func TestCareTimeOverrunsStayWithTheirTraeger(t *testing.T) {
	t.Parallel()
	f := newOverrunFixture(t)
	code, _, raw := f.get(t, f.foreignJWT, overrunWindow+"&school_id="+strconv.FormatInt(f.school, 10))
	assert.Equal(t, http.StatusForbidden, code, raw)
	assert.Contains(t, raw, "care_contract.school_outside_traeger")
	code, body, raw := f.get(t, f.foreignJWT, overrunWindow)
	require.Equal(t, http.StatusOK, code, raw)
	for _, school := range body.Schools {
		assert.NotEqual(t, f.school, school.SchoolID, "another Träger never sees the school")
	}

	for _, window := range []string{"from=2026-06-16&to=2026-06-08", "from=2026-01-01&to=2026-06-16", "from=2026-06-08", "from=x&to=y"} {
		code, _, raw := f.get(t, f.admin, "/traeger/care-time-overruns?"+window)
		assert.Equal(t, http.StatusBadRequest, code, "%s: %s", window, raw)
	}
}

// The evaluation reads every school in its own unit: its statements grow
// with the schools of the Träger, never with the children (#4683).
func TestCareTimeOverrunsQueryBudget(t *testing.T) {
	t.Parallel()
	f := newOverrunFixtureOn(t, testpkg.SetupIsolatedTestDB(t))
	counter := testpkg.CaptureQueries(t, f.db)
	run := func(token, path string) []string {
		t.Helper()
		counter.Reset()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		f.router.ServeHTTP(rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Queries()
	}
	const childrenPath = "/traeger/care-time-overruns/children?from=2026-06-08&to=2026-06-16"
	// The first reads record the report access; the budget measures the
	// steady state after it, like the school's overrun list.
	run(f.billing, overrunWindow)
	run(f.admin, childrenPath)
	summaryOne := run(f.billing, overrunWindow)
	childrenOne := run(f.admin, childrenPath)

	staff, _ := testpkg.CreateTestStaffWithAccountForTenant(t, f.db, f.school, "Budget", "Kind")
	for i := 0; i < 3; i++ {
		careTimeChild(t, &statisticsHTTPTestContext{db: f.db}, staff.ID)
	}
	summaryMany := run(f.billing, overrunWindow)
	childrenMany := run(f.admin, childrenPath)

	assert.Len(t, summaryMany, len(summaryOne), "the summary must batch its children")
	assert.Len(t, childrenMany, len(childrenOne), "the named list must batch its children")
	querybudgets.AssertQueryBudget(t, "api.traeger.care_time_overruns", summaryMany)
	querybudgets.AssertQueryBudget(t, "api.traeger.care_time_overrun_children", childrenMany)
}
