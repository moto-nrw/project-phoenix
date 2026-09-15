package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The tests below exercise the composed /staff router (#2667): membership
// routes from the School Membership adapter and workforce routes from the
// time-tracking admin resource share one protected group. Each assertion
// spans both halves, which is why they live with the composition root.

// staffCompositionContext keeps the composed router plus fixture closures;
// the closures capture the database so the test names no persistence type.
type staffCompositionContext struct {
	router chi.Router
	// createStaff creates a staff member and returns (staffID, personID).
	createStaff func(firstName, lastName string) (int64, int64)
	// absentToday records an approved absence of a school-defined type with
	// the given wording for the staff member on the current day.
	absentToday func(staffID int64, wording string)
}

func setupStaffCompositionRoute(t *testing.T) *staffCompositionContext {
	t.Helper()
	db, svc := testutil.SetupStaffModule(t)
	membership, err := repositories.NewSchoolMembership(db)
	require.NoError(t, err)
	resource := newStaffTestResource(membership, svc, db, slog.Default())

	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	router.Mount("/staff", resource.Router())
	return &staffCompositionContext{
		router: router,
		createStaff: func(firstName, lastName string) (int64, int64) {
			staff := testpkg.CreateTestStaff(t, db, firstName, lastName)
			return staff.ID, staff.PersonID
		},
		absentToday: func(staffID int64, wording string) {
			absenceType := testpkg.CreateTestStaffAbsenceType(t, db, wording)
			testpkg.CreateTestStaffAbsenceToday(t, db, staffID, absenceType.ID)
		},
	}
}

func staffCompositionToken(t *testing.T, perms ...string) string {
	t.Helper()
	claims := testutil.DefaultTestClaims()
	claims.Permissions = perms
	return testutil.MintTestJWT(t, claims)
}

// betreuerPermissions is the effective permission set of the unmodified
// `user` (Betreuer) system role for the staff surfaces: it may read the
// directory and write child data, and that is all (#2906).
var betreuerPermissions = []string{
	"users:read", "users:list", "users:create", "users:update", "users:absence",
	"groups:read", "visits:read", "time_tracking:own",
}

// TestBetreuerRoleCannotReachColleaguePersonnelData pins acceptance criteria
// 1-3 of #2906 across both halves of the composed router.
func TestBetreuerRoleCannotReachColleaguePersonnelData(t *testing.T) {
	t.Parallel()

	ctx := setupStaffCompositionRoute(t)
	colleagueID, colleaguePersonID := ctx.createStaff("Kollegin", "Personalakte")
	token := staffCompositionToken(t, betreuerPermissions...)

	forbiddenReads := []string{
		fmt.Sprintf("/staff/%d/stammdaten", colleagueID),
		fmt.Sprintf("/staff/%d/documents", colleagueID),
		fmt.Sprintf("/staff/documents-profile/%d", colleagueID),
		"/staff/documents-directory",
		fmt.Sprintf("/staff/%d/stammdaten/bank-steuer", colleagueID),
	}
	for _, path := range forbiddenReads {
		req := testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil, testutil.WithJWTBearer(token))
		rr := testutil.ExecuteRequest(ctx.router, req)
		assert.Equal(t, http.StatusForbidden, rr.Code, "GET %s must be refused: %s", path, rr.Body.String())
	}

	forbiddenWrites := []struct {
		method string
		path   string
		body   map[string]interface{}
	}{
		{http.MethodPut, fmt.Sprintf("/staff/%d", colleagueID),
			map[string]interface{}{"person_id": colleaguePersonID, "staff_notes": "Fremde Notiz"}},
		{http.MethodPut, fmt.Sprintf("/staff/%d/stammdaten/person", colleagueID),
			map[string]interface{}{"first_name": "Neu", "last_name": "Name"}},
		{http.MethodPut, fmt.Sprintf("/staff/%d/stammdaten/kontakt", colleagueID),
			map[string]interface{}{"phone": "+49 170 1"}},
		{http.MethodPut, fmt.Sprintf("/staff/%d/stammdaten/arbeitsvertrag", colleagueID),
			map[string]interface{}{"weekly_hours": 20}},
		{http.MethodPut, fmt.Sprintf("/staff/%d/stammdaten/qualifikationen", colleagueID),
			map[string]interface{}{"qualifikationen": []any{}}},
		{http.MethodPut, fmt.Sprintf("/staff/%d/vacation/quota", colleagueID),
			map[string]interface{}{"year": 2026, "days": 30}},
	}
	for _, tc := range forbiddenWrites {
		req := testutil.NewAuthenticatedRequest(t, tc.method, tc.path, tc.body, testutil.WithJWTBearer(token))
		rr := testutil.ExecuteRequest(ctx.router, req)
		assert.Equal(t, http.StatusForbidden, rr.Code, "%s %s must be refused: %s", tc.method, tc.path, rr.Body.String())
	}
}

// TestBetreuerRoleSeesOnlyTheMinimalColleagueView pins acceptance criterion
// 4 of #2906: the directory a Betreuer may read carries names, work e-mail,
// account role and today's presence, never notes, employment type, absence
// reason or NFC tag.
func TestBetreuerRoleSeesOnlyTheMinimalColleagueView(t *testing.T) {
	t.Parallel()

	ctx := setupStaffCompositionRoute(t)
	colleagueID, colleaguePersonID := ctx.createStaff("Sichtbare", "Kollegin")

	body := map[string]interface{}{
		"person_id":   colleaguePersonID,
		"staff_notes": "Geheime Personalnotiz",
	}
	req := testutil.NewAuthenticatedRequest(t, http.MethodPut, fmt.Sprintf("/staff/%d", colleagueID), body,
		testutil.WithJWTBearer(staffCompositionToken(t, "staff:manage")))
	testutil.AssertSuccessResponse(t, testutil.ExecuteRequest(ctx.router, req), http.StatusOK)

	betreuer := testutil.WithJWTBearer(staffCompositionToken(t, betreuerPermissions...))
	for _, path := range []string{"/staff", fmt.Sprintf("/staff/%d", colleagueID)} {
		rr := testutil.ExecuteRequest(ctx.router,
			testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil, betreuer))
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		assert.NotContains(t, rr.Body.String(), "Geheime Personalnotiz",
			"%s must not carry staff notes for the Betreuer tier", path)
		assert.NotContains(t, rr.Body.String(), `"staff_notes"`, path)
		assert.NotContains(t, rr.Body.String(), `"employment_type"`, path)
		assert.NotContains(t, rr.Body.String(), `"absence_type"`, path)
		assert.NotContains(t, rr.Body.String(), `"tag_id"`, path)
	}

	rr := testutil.ExecuteRequest(ctx.router, testutil.NewAuthenticatedRequest(t, http.MethodGet,
		fmt.Sprintf("/staff/%d", colleagueID), nil, testutil.WithJWTBearer(staffCompositionToken(t, "staff:manage", "users:read"))))
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "Geheime Personalnotiz")

	rr = testutil.ExecuteRequest(ctx.router, testutil.NewAuthenticatedRequest(t, http.MethodGet,
		fmt.Sprintf("/staff/%d", colleagueID), nil, testutil.WithJWTBearer(staffCompositionToken(t, "staff:stammdaten"))))
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.NotContains(t, rr.Body.String(), "Geheime Personalnotiz",
		"staff:stammdaten maintains the personnel file, not the private staff notes")
}

// TestPersonnelPermissionsReachTheStaffDirectory pins that the two personnel
// permissions of #2906 open the staff list and the profile on their own.
func TestPersonnelPermissionsReachTheStaffDirectory(t *testing.T) {
	t.Parallel()

	ctx := setupStaffCompositionRoute(t)
	colleagueID, _ := ctx.createStaff("Erreichbare", "Kollegin")

	for _, permission := range []string{"staff:stammdaten", "staff:manage"} {
		auth := testutil.WithJWTBearer(staffCompositionToken(t, permission))
		for _, path := range []string{"/staff", fmt.Sprintf("/staff/%d", colleagueID)} {
			rr := testutil.ExecuteRequest(ctx.router,
				testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil, auth))
			assert.Equal(t, http.StatusOK, rr.Code,
				"GET %s must be reachable with %s: %s", path, permission, rr.Body.String())
		}
	}
}

func TestStaffPersonConflictIsHTTPConflict(t *testing.T) {
	t.Parallel()

	ctx := setupStaffCompositionRoute(t)
	staffID, _ := ctx.createStaff("Bestehende", "Zuordnung")
	_, conflictingPersonID := ctx.createStaff("Doppelte", "Zuordnung")

	req := testutil.NewAuthenticatedRequest(t, http.MethodPut, fmt.Sprintf("/staff/%d", staffID),
		map[string]interface{}{"person_id": conflictingPersonID},
		testutil.WithJWTBearer(staffCompositionToken(t, "staff:manage", "users:manage")))
	rr := testutil.ExecuteRequest(ctx.router, req)

	assert.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())
}

// TestBetreuerRoleDoesNotSeeTheSchoolsAbsenceWording pins that the school's
// own Abwesenheitsart wording (#2403) follows the personnel gate on both
// staff read paths (#2906).
func TestBetreuerRoleDoesNotSeeTheSchoolsAbsenceWording(t *testing.T) {
	t.Parallel()

	ctx := setupStaffCompositionRoute(t)
	colleagueID, _ := ctx.createStaff("Abwesende", "Kollegin")
	ctx.absentToday(colleagueID, "Regenerationstag")

	betreuer := testutil.WithJWTBearer(staffCompositionToken(t, betreuerPermissions...))
	for _, path := range []string{"/staff", fmt.Sprintf("/staff/%d", colleagueID)} {
		rr := testutil.ExecuteRequest(ctx.router,
			testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil, betreuer))
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		assert.NotContains(t, rr.Body.String(), "Regenerationstag",
			"%s must not carry the school's absence wording for the Betreuer tier", path)
		assert.NotContains(t, rr.Body.String(), `"absence_type_label"`, path)
	}

	personnel := testutil.WithJWTBearer(staffCompositionToken(t, "users:read", "staff:stammdaten"))
	for _, path := range []string{"/staff", fmt.Sprintf("/staff/%d", colleagueID)} {
		rr := testutil.ExecuteRequest(ctx.router,
			testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil, personnel))
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Regenerationstag", path)
	}
}
