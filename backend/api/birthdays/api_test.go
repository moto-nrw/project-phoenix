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
		resource: birthdaysAPI.NewResource(
			svc.Birthdays, svc.ListExport, svc.UserContext, svc.Settings, db, slog.Default(),
		),
	}
}

func claimsFor(tb testing.TB, accountID int64) jwt.AppClaims {
	return jwt.AppClaims{
		ID:        int(accountID),
		Sub:       "birthday@example.com",
		Username:  "birthday",
		FirstName: "Birthday",
		LastName:  "Tester",
		Roles:     []string{"user"},
		TenantID:  testpkg.Tenant(tb),
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
	ctx := testpkg.Ctx(t)
	require.NoError(t, tc.services.Settings.SetValue(ctx, key, value, nil, nil), "set %s", key)
	t.Cleanup(func() {
		_ = tc.services.Settings.ResetValue(ctx, key, nil, nil)
	})
}

// adminPermissions mints the admin wildcard: children are student data, so a
// caller sees them with admin rights or a staff record in the tenant (#2329).
func adminPermissions() []string {
	return []string{permissions.UsersRead, "admin:*"}
}

func getOverview(t *testing.T, tc *testContext, accountID int64, perms []string) *httptest.ResponseRecorder {
	t.Helper()
	req, err := http.NewRequest("GET", "/", nil)
	require.NoError(t, err)
	return authExec(t, tc, req, claimsFor(t, accountID), perms)
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
	t.Parallel()

	tc := setupTestContext(t)
	account := testpkg.CreateTestAccount(t, tc.db, "birthday-overview@example.com")

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
	t.Parallel()

	tc := setupTestContext(t)

	req, err := http.NewRequest("GET", "/", nil)
	require.NoError(t, err)
	rr := testutil.ExecuteRequest(tc.resource.Router(), req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, rr.Body.String())
}

// A child with today's birth date appears with the age it reaches; a child
// without a stored date simply is not in the list (AK 5).
func TestOverviewListsTodaysChildren(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)
	setSetting(t, tc, configModel.KeyBirthdayDisplayEnabled, true)

	account := testpkg.CreateTestAccount(t, tc.db, "birthday-children@example.com")

	today := timezone.TodayDate()
	celebrating := testpkg.CreateTestStudent(t, tc.db, "Lina", "Geburtstagskind", "1a")
	_ = testpkg.CreateTestStudent(t, tc.db, "Ohne", "Datum", "1a")

	setPersonBirthday(t, tc.db, celebrating.PersonID, timezone.NewDate(today.Year-7, today.Month, today.Day))

	rr := getOverview(t, tc, account.ID, adminPermissions())
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
	t.Parallel()

	tc := setupTestContext(t)
	setSetting(t, tc, configModel.KeyBirthdayDisplayEnabled, false)

	account := testpkg.CreateTestAccount(t, tc.db, "birthday-off@example.com")

	today := timezone.TodayDate()
	student := testpkg.CreateTestStudent(t, tc.db, "Nicht", "Sichtbar", "2b")
	setPersonBirthday(t, tc.db, student.PersonID, timezone.NewDate(today.Year-6, today.Month, today.Day))

	rr := getOverview(t, tc, account.ID, adminPermissions())
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	payload := decodeOverview(t, rr)
	assert.False(t, payload.Data.Enabled)
	assert.Empty(t, payload.Data.Celebrations)
}

// Datenschutz: staff birthdays appear only when the school opted in, and never
// for someone who opted out personally.
func TestOverviewStaffVisibility(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)
	setSetting(t, tc, configModel.KeyBirthdayDisplayEnabled, true)

	account := testpkg.CreateTestAccount(t, tc.db, "birthday-staff@example.com")

	today := timezone.TodayDate()
	staff := testpkg.CreateTestStaff(t, tc.db, "Anna", "Kollegin")
	optedOut := testpkg.CreateTestStaff(t, tc.db, "Bea", "Abgemeldet")

	birthday := timezone.NewDate(today.Year-40, today.Month, today.Day)
	setPersonBirthday(t, tc.db, staff.PersonID, birthday)
	setPersonBirthday(t, tc.db, optedOut.PersonID, birthday)

	ctx := testpkg.Ctx(t)
	_, err := tc.db.NewUpdate().
		Table("users.staff").
		Set("birthday_display_opt_out = TRUE").
		Where("id = ?", optedOut.ID).
		Exec(ctx)
	require.NoError(t, err)

	t.Run("hidden while the school has not opted in", func(t *testing.T) {
		rr := getOverview(t, tc, account.ID, adminPermissions())
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

		payload := decodeOverview(t, rr)
		assert.False(t, payload.Data.IncludeStaff)
		assert.NotContains(t, payload.names(), "Anna Kollegin")
	})

	t.Run("shown once the school opted in, except for the opt-out", func(t *testing.T) {
		setSetting(t, tc, configModel.KeyBirthdayDisplayIncludeStaff, true)

		rr := getOverview(t, tc, account.ID, adminPermissions())
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
	t.Parallel()

	tc := setupTestContext(t)

	_, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Clara", "Selbst")

	read := func() *httptest.ResponseRecorder {
		req, err := http.NewRequest("GET", "/opt-out", nil)
		require.NoError(t, err)
		return authExec(t, tc, req, claimsFor(t, account.ID), []string{})
	}

	rr := read()
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"opt_out":false`)

	req, err := http.NewRequest("PUT", "/opt-out", strings.NewReader(`{"opt_out":true}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	rr = authExec(t, tc, req, claimsFor(t, account.ID), []string{})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	rr = read()
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"opt_out":true`)
}

// An account without a staff record has nothing to opt out of — a 404, not a
// 500 and not a silent success.
func TestOptOutWithoutStaffRecord(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)
	account := testpkg.CreateTestAccount(t, tc.db, "birthday-no-staff@example.com")

	req, err := http.NewRequest("GET", "/opt-out", nil)
	require.NoError(t, err)
	rr := authExec(t, tc, req, claimsFor(t, account.ID), []string{})

	assert.Equal(t, http.StatusNotFound, rr.Code, rr.Body.String())
}

// Berechtigung: the staff list reveals full birth dates, so users:read is not
// enough — it needs the permission that opens the Stammdaten behind it.
func TestStaffExportPermissionGate(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)
	account := testpkg.CreateTestAccount(t, tc.db, "birthday-export@example.com")

	request := func() *http.Request {
		req, err := http.NewRequest("POST", "/staff-export", strings.NewReader(`{"format":"xlsx"}`))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		return req
	}

	t.Run("rejected with users:read alone", func(t *testing.T) {
		rr := authExec(t, tc, request(), claimsFor(t, account.ID), []string{permissions.UsersRead})
		assert.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
	})

	t.Run("allowed with users:update", func(t *testing.T) {
		rr := authExec(t, tc, request(), claimsFor(t, account.ID), []string{permissions.UsersUpdate})
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
	t.Parallel()

	tc := setupTestContext(t)
	account := testpkg.CreateTestAccount(t, tc.db, "birthday-export-month@example.com")

	req, err := http.NewRequest("POST", "/staff-export", strings.NewReader(`{"format":"xlsx","months":["13"]}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rr := authExec(t, tc, req, claimsFor(t, account.ID), []string{permissions.UsersUpdate})
	assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
}

// Datenschutz-Blocker aus dem Review: a birthday row carries a child's name,
// group, class and age. users:read alone must therefore not hand out the whole
// school — the route applies the same student read gate as every other child
// list, so an account without a staff record sees nothing.
func TestOverviewAppliesStudentDataScope(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)
	setSetting(t, tc, configModel.KeyBirthdayDisplayEnabled, true)

	account := testpkg.CreateTestAccount(t, tc.db, "birthday-scope@example.com")

	today := timezone.TodayDate()
	student := testpkg.CreateTestStudent(t, tc.db, "Fremdes", "Scopekind", "4a")
	setPersonBirthday(t, tc.db, student.PersonID, timezone.NewDate(today.Year-9, today.Month, today.Day))

	t.Run("plain users:read without a staff record sees no child", func(t *testing.T) {
		rr := getOverview(t, tc, account.ID, []string{permissions.UsersRead})
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

		assert.NotContains(t, decodeOverview(t, rr).names(), "Fremdes Scopekind")
	})

	t.Run("admin sees the child", func(t *testing.T) {
		rr := getOverview(t, tc, account.ID, adminPermissions())
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

		assert.Contains(t, decodeOverview(t, rr).names(), "Fremdes Scopekind")
	})

	// The other way in is a staff record: every verified staff member reads the
	// directory (#2329), so the card follows without any tenant opt-in.
	t.Run("a staff record opens it for plain users:read", func(t *testing.T) {
		_, staffAccount := testpkg.CreateTestStaffWithAccount(t, tc.db, "Sara", "Scopekraft")

		rr := getOverview(t, tc, staffAccount.ID, []string{permissions.UsersRead})
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

		assert.Contains(t, decodeOverview(t, rr).names(), "Fremdes Scopekind")
	})
}
