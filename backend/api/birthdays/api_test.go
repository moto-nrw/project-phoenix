// Package birthdays_test drives the production Resource.Router(), so the full
// middleware chain (Verifier → Authenticator → TenantMiddleware →
// RequiresPermission → TenantTxMiddleware) runs exactly as on the real server.
//
// What it pins: the permission gates (a colleague's birth date must not fall
// out of a users:read route), the two settings that govern the display, and the
// personal opt-out (#1542).
package birthdays_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	birthdaysAPI "github.com/moto-nrw/project-phoenix/api/birthdays"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type testContext struct {
	db       *bun.DB
	services *services.Factory
	resource *birthdaysAPI.Resource
}

func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)
	return &testContext{
		db:       db,
		services: svc,
		resource: birthdaysAPI.NewResource(svc.Birthdays, svc.ListExport, db, slog.Default()),
	}
}

func claimsFor(accountID int64) jwt.AppClaims {
	return jwt.AppClaims{
		ID:        int(accountID),
		Sub:       "birthday@example.com",
		Username:  "birthday",
		FirstName: "Birthday",
		LastName:  "Tester",
		Roles:     []string{"user"},
		TenantID:  1,
	}
}

func authExec(t *testing.T, tc *testContext, req *http.Request, claims jwt.AppClaims, perms []string) *httptest.ResponseRecorder {
	t.Helper()
	claims.Permissions = perms
	req.Header.Set("Authorization", "Bearer "+testutil.MintTestJWT(t, claims))
	return testutil.ExecuteRequest(tc.resource.Router(), req)
}

// setPersonBirthday stamps a birth date on an existing fixture person. The
// fixtures create people without one, which is itself the realistic default:
// a school that has not maintained every date must still get a working list.
func setPersonBirthday(t *testing.T, db *bun.DB, personID int64, date timezone.Date) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.NewUpdate().
		Table("users.persons").
		Set("birthday = ?", date).
		Where("id = ?", personID).
		Exec(ctx)
	require.NoError(t, err, "stamp birthday on test person")
}

func setSetting(t *testing.T, tc *testContext, key string, value any) {
	t.Helper()
	ctx := testpkg.TenantContext(1)
	require.NoError(t, tc.services.Settings.SetValue(ctx, key, value, nil, nil), "set %s", key)
	t.Cleanup(func() {
		_ = tc.services.Settings.ResetValue(ctx, key, nil, nil)
	})
}

func getOverview(t *testing.T, tc *testContext, accountID int64, perms []string) *httptest.ResponseRecorder {
	t.Helper()
	req, err := http.NewRequest("GET", "/", nil)
	require.NoError(t, err)
	return authExec(t, tc, req, claimsFor(accountID), perms)
}

type overviewPayload struct {
	Data struct {
		Enabled      bool   `json:"enabled"`
		IncludeStaff bool   `json:"include_staff"`
		Today        string `json:"today"`
		Celebrations []struct {
			Kind      string `json:"kind"`
			Name      string `json:"name"`
			Age       int    `json:"age"`
			IsToday   bool   `json:"is_today"`
			Date      string `json:"date"`
			GroupName string `json:"group_name"`
		} `json:"celebrations"`
	} `json:"data"`
}

func decodeOverview(t *testing.T, rr *httptest.ResponseRecorder) overviewPayload {
	t.Helper()
	var payload overviewPayload
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &payload), rr.Body.String())
	return payload
}

func (p overviewPayload) names() []string {
	names := make([]string, 0, len(p.Data.Celebrations))
	for _, celebration := range p.Data.Celebrations {
		names = append(names, celebration.Name)
	}
	return names
}

// Berechtigung: the overview lists names of children and colleagues, so it
// stays behind users:read like the rest of the directory.
func TestOverviewRequiresUsersRead(t *testing.T) {
	tc := setupTestContext(t)
	account := testpkg.CreateTestAccount(t, tc.db, "birthday-overview@example.com")
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	t.Run("rejected without permissions", func(t *testing.T) {
		rr := getOverview(t, tc, account.ID, []string{})
		assert.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
	})

	t.Run("rejected with an unrelated permission", func(t *testing.T) {
		rr := getOverview(t, tc, account.ID, []string{permissions.RoomsRead})
		assert.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
	})

	t.Run("allowed with users:read", func(t *testing.T) {
		rr := getOverview(t, tc, account.ID, []string{permissions.UsersRead})
		assert.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	})
}

func TestOverviewRejectsUnauthenticated(t *testing.T) {
	tc := setupTestContext(t)

	req, err := http.NewRequest("GET", "/", nil)
	require.NoError(t, err)
	rr := testutil.ExecuteRequest(tc.resource.Router(), req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, rr.Body.String())
}

// A child with today's birth date appears with the age it reaches; a child
// without a stored date simply is not in the list (AK 5).
func TestOverviewListsTodaysChildren(t *testing.T) {
	tc := setupTestContext(t)
	setSetting(t, tc, configModel.KeyBirthdayDisplayEnabled, true)

	account := testpkg.CreateTestAccount(t, tc.db, "birthday-children@example.com")
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	today := timezone.TodayDate()
	celebrating := testpkg.CreateTestStudent(t, tc.db, "Lina", "Geburtstagskind", "1a")
	withoutDate := testpkg.CreateTestStudent(t, tc.db, "Ohne", "Datum", "1a")
	defer testpkg.CleanupActivityFixtures(t, tc.db, celebrating.ID, withoutDate.ID)

	setPersonBirthday(t, tc.db, celebrating.PersonID, timezone.NewDate(today.Year-7, today.Month, today.Day))

	rr := getOverview(t, tc, account.ID, []string{permissions.UsersRead})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	payload := decodeOverview(t, rr)
	assert.True(t, payload.Data.Enabled)
	assert.Equal(t, today.String(), payload.Data.Today)
	assert.Contains(t, payload.names(), "Lina Geburtstagskind")
	assert.NotContains(t, payload.names(), "Ohne Datum",
		"a child without a stored birth date must not be invented into the list")

	for _, celebration := range payload.Data.Celebrations {
		if celebration.Name == "Lina Geburtstagskind" {
			assert.Equal(t, "student", celebration.Kind)
			assert.Equal(t, 7, celebration.Age)
			assert.True(t, celebration.IsToday)
		}
	}
}

// The school switch is a real switch: off means the dashboard shows nothing,
// not "everything anyway".
func TestOverviewDisabledReturnsNothing(t *testing.T) {
	tc := setupTestContext(t)
	setSetting(t, tc, configModel.KeyBirthdayDisplayEnabled, false)

	account := testpkg.CreateTestAccount(t, tc.db, "birthday-off@example.com")
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	today := timezone.TodayDate()
	student := testpkg.CreateTestStudent(t, tc.db, "Nicht", "Sichtbar", "2b")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)
	setPersonBirthday(t, tc.db, student.PersonID, timezone.NewDate(today.Year-6, today.Month, today.Day))

	rr := getOverview(t, tc, account.ID, []string{permissions.UsersRead})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	payload := decodeOverview(t, rr)
	assert.False(t, payload.Data.Enabled)
	assert.Empty(t, payload.Data.Celebrations)
}

// Datenschutz: staff birthdays appear only when the school opted in, and never
// for someone who opted out personally.
func TestOverviewStaffVisibility(t *testing.T) {
	tc := setupTestContext(t)
	setSetting(t, tc, configModel.KeyBirthdayDisplayEnabled, true)

	account := testpkg.CreateTestAccount(t, tc.db, "birthday-staff@example.com")
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	today := timezone.TodayDate()
	staff := testpkg.CreateTestStaff(t, tc.db, "Anna", "Kollegin")
	optedOut := testpkg.CreateTestStaff(t, tc.db, "Bea", "Abgemeldet")
	defer testpkg.CleanupStaffFixtures(t, tc.db, staff.ID, optedOut.ID)

	birthday := timezone.NewDate(today.Year-40, today.Month, today.Day)
	setPersonBirthday(t, tc.db, staff.PersonID, birthday)
	setPersonBirthday(t, tc.db, optedOut.PersonID, birthday)

	ctx := testpkg.TenantContext(1)
	_, err := tc.db.NewUpdate().
		Table("users.staff").
		Set("birthday_display_opt_out = TRUE").
		Where("id = ?", optedOut.ID).
		Exec(ctx)
	require.NoError(t, err)

	t.Run("hidden while the school has not opted in", func(t *testing.T) {
		rr := getOverview(t, tc, account.ID, []string{permissions.UsersRead})
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

		payload := decodeOverview(t, rr)
		assert.False(t, payload.Data.IncludeStaff)
		assert.NotContains(t, payload.names(), "Anna Kollegin")
	})

	t.Run("shown once the school opted in, except for the opt-out", func(t *testing.T) {
		setSetting(t, tc, configModel.KeyBirthdayDisplayIncludeStaff, true)

		rr := getOverview(t, tc, account.ID, []string{permissions.UsersRead})
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

		payload := decodeOverview(t, rr)
		assert.True(t, payload.Data.IncludeStaff)
		assert.Contains(t, payload.names(), "Anna Kollegin")
		assert.NotContains(t, payload.names(), "Bea Abgemeldet",
			"a personal opt-out outranks the school setting")

		for _, celebration := range payload.Data.Celebrations {
			if celebration.Name == "Anna Kollegin" {
				assert.Equal(t, "staff", celebration.Kind)
				assert.Zero(t, celebration.Age, "a colleague's age is never published")
			}
		}
	})
}

// The opt-out is self-service and acts on the caller's own row.
func TestOptOutRoundTrip(t *testing.T) {
	tc := setupTestContext(t)

	staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Clara", "Selbst")
	defer testpkg.CleanupStaffFixtures(t, tc.db, staff.ID)
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	read := func() *httptest.ResponseRecorder {
		req, err := http.NewRequest("GET", "/opt-out", nil)
		require.NoError(t, err)
		return authExec(t, tc, req, claimsFor(account.ID), []string{})
	}

	rr := read()
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"opt_out":false`)

	req, err := http.NewRequest("PUT", "/opt-out", strings.NewReader(`{"opt_out":true}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	rr = authExec(t, tc, req, claimsFor(account.ID), []string{})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	rr = read()
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"opt_out":true`)
}

// An account without a staff record has nothing to opt out of — a 404, not a
// 500 and not a silent success.
func TestOptOutWithoutStaffRecord(t *testing.T) {
	tc := setupTestContext(t)
	account := testpkg.CreateTestAccount(t, tc.db, "birthday-no-staff@example.com")
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	req, err := http.NewRequest("GET", "/opt-out", nil)
	require.NoError(t, err)
	rr := authExec(t, tc, req, claimsFor(account.ID), []string{})

	assert.Equal(t, http.StatusNotFound, rr.Code, rr.Body.String())
}

// Berechtigung: the staff list reveals full birth dates, so users:read is not
// enough — it needs the permission that opens the Stammdaten behind it.
func TestStaffExportPermissionGate(t *testing.T) {
	tc := setupTestContext(t)
	account := testpkg.CreateTestAccount(t, tc.db, "birthday-export@example.com")
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	request := func() *http.Request {
		req, err := http.NewRequest("POST", "/staff-export", strings.NewReader(`{"format":"xlsx"}`))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		return req
	}

	t.Run("rejected with users:read alone", func(t *testing.T) {
		rr := authExec(t, tc, request(), claimsFor(account.ID), []string{permissions.UsersRead})
		assert.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
	})

	t.Run("allowed with users:update", func(t *testing.T) {
		rr := authExec(t, tc, request(), claimsFor(account.ID), []string{permissions.UsersUpdate})
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		assert.Equal(t,
			"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			rr.Header().Get("Content-Type"))
		assert.Contains(t, rr.Header().Get("Content-Disposition"), ".xlsx")
	})
}

// A month filter outside 01..12 is a client mistake, not a silent full-year
// export.
func TestStaffExportRejectsInvalidMonth(t *testing.T) {
	tc := setupTestContext(t)
	account := testpkg.CreateTestAccount(t, tc.db, "birthday-export-month@example.com")
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	req, err := http.NewRequest("POST", "/staff-export", strings.NewReader(`{"format":"xlsx","months":["13"]}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rr := authExec(t, tc, req, claimsFor(account.ID), []string{permissions.UsersUpdate})
	assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
}
