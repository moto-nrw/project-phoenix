// Router-level tests for GET /api/time-tracking/absence-allowances (#4971):
// the staff member's own Kontingente, read-only.
package timetracking

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/http/absencetypes"
	"github.com/moto-nrw/project-phoenix/modules/workforce/workforcetest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

const ownAllowancesPath = "/time-tracking/absence-allowances"

var ownAllowancesNow = testpkg.Date(2026, time.August, 24).BerlinMidnight().Add(12 * time.Hour)

type ownAllowancesRoute struct {
	db         *bun.DB
	capability workforce.Capability
	router     chi.Router
}

// setupOwnAllowancesRoute serves the own time-tracking resource through the
// production middleware chain, mounted at /time-tracking.
func setupOwnAllowancesRoute(t *testing.T) *ownAllowancesRoute {
	t.Helper()
	clock := func() time.Time { return ownAllowancesNow }
	db, capabilities := workforcetest.ComposeAdmin(t, clock)
	capability := newWorkforceCapability(t, db, clock)
	resource := NewResource(Dependencies{
		WorkSessions:  capabilities.WorkSessions,
		StaffAbsences: capabilities.StaffAbsences,
		AbsenceWrites: capabilities.AbsenceWrites,
		Staff:         capabilities.Staff,
		Allowances:    capability,
		AllowanceResponses: func(accounts []workforce.AbsenceTypeAllowanceAccount) any {
			return absencetypes.ToOwnAllowanceResponses(accounts)
		},
		Identity:     testIdentity,
		WithinTenant: testWithinTenant(db),
	})
	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	router.Mount("/time-tracking", resource.Router(scopeTestTenantRoutes(testpkg.TenantSessionGatesWithCaches(scopeTestSettingsCache))))
	return &ownAllowancesRoute{db: db, capability: capability, router: router}
}

func ownToken(t *testing.T, accountID, tenantID int64, perms ...string) string {
	t.Helper()
	claims := testutil.DefaultTestClaims()
	claims.ID = int(accountID)
	claims.Roles = []string{"user"}
	claims.IsAdmin = false
	claims.Permissions = perms
	if tenantID != 0 {
		claims.TenantID = tenantID
	}
	return testutil.MintTestJWT(t, claims)
}

func (r *ownAllowancesRoute) get(ctx context.Context, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.router.ServeHTTP(rec, req)
	return rec
}

func decodeOwnAllowances(t *testing.T, rec *httptest.ResponseRecorder) []absencetypes.OwnAllowanceResponse {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body struct {
		Data []absencetypes.OwnAllowanceResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.NotNil(t, body.Data, "no type answers with an empty list, not null")
	return body.Data
}

func (r *ownAllowancesRoute) allowanceType(t *testing.T, ctx context.Context, name string, allowance bool) workforce.StaffAbsenceType {
	t.Helper()
	typ, err := r.capability.CreateAbsenceType(ctx, workforce.CreateAbsenceType{Name: name, AllowanceEnabled: allowance})
	require.NoError(t, err)
	return typ
}

func (r *ownAllowancesRoute) claim(t *testing.T, ctx context.Context, staffID, typeID int64, days float64, changedBy int64) {
	t.Helper()
	_, err := r.capability.SetAllowance(ctx, workforce.SetAbsenceTypeAllowance{
		StaffID: staffID, AbsenceTypeID: typeID, Year: 2026, EntitledDays: days, Reason: "Anspruch", ChangedBy: changedBy,
	})
	require.NoError(t, err)
}

func (r *ownAllowancesRoute) book(t *testing.T, ctx context.Context, staffID, typeID int64, day, status string) {
	t.Helper()
	_, err := r.capability.CreateStaffAbsence(ctx, &workforce.StaffAbsence{
		StaffID: staffID, CreatedBy: staffID, AbsenceType: workforce.AbsenceTypeOther, AbsenceTypeID: &typeID,
		Status: status, DateStart: day, DateEnd: day, CreatedAt: ownAllowancesNow,
	})
	require.NoError(t, err)
}

func TestOwnAbsenceAllowances_ShowTheLeitungNumbersOfTheCaller(t *testing.T) {
	t.Parallel()
	route := setupOwnAllowancesRoute(t)
	ctx := testpkg.Ctx(t)
	staff, account := testpkg.CreateTestStaffWithAccount(t, route.db, "Rena", "Eigen")
	colleague := testpkg.CreateTestStaff(t, route.db, "Olaf", "Fremd")
	admin := testpkg.CreateTestStaff(t, route.db, "Swantje", "Leitung")

	regeneration := route.allowanceType(t, ctx, "Regenerationstag", true)
	route.allowanceType(t, ctx, "Ohne Kontingent", false)
	retired := route.allowanceType(t, ctx, "Alt", true)
	route.claim(t, ctx, staff.ID, regeneration.ID, 3, admin.ID)
	route.claim(t, ctx, staff.ID, retired.ID, 2, admin.ID)
	route.claim(t, ctx, colleague.ID, regeneration.ID, 9, admin.ID)
	inactive := false
	_, err := route.capability.UpdateAbsenceType(ctx, workforce.UpdateAbsenceType{ID: retired.ID, IsActive: &inactive})
	require.NoError(t, err)
	route.book(t, ctx, staff.ID, regeneration.ID, "2026-03-02", workforce.AbsenceStatusApproved)
	route.book(t, ctx, staff.ID, regeneration.ID, "2026-09-07", workforce.AbsenceStatusRequested)
	route.book(t, ctx, colleague.ID, regeneration.ID, "2026-03-03", workforce.AbsenceStatusApproved)

	token := ownToken(t, account.ID, 0, "time_tracking:own")
	// A staff ID in the request is not part of the contract and changes nothing.
	path := fmt.Sprintf("%s?year=2026&staff_id=%d&staffId=%d", ownAllowancesPath, colleague.ID, colleague.ID)
	got := decodeOwnAllowances(t, route.get(context.Background(), path, token))

	require.Len(t, got, 1, "inactive types and types without an allowance stay out")
	leitung, err := route.capability.AllowanceSummary(ctx, staff.ID, regeneration.ID, 2026)
	require.NoError(t, err)
	assert.Equal(t, absencetypes.ToOwnAllowanceResponses([]workforce.AbsenceTypeAllowanceAccount{{AbsenceType: regeneration, Summary: leitung}})[0], got[0])
	assert.Equal(t, fmt.Sprint(staff.ID), got[0].StaffID)
	assert.Equal(t, 3.0, got[0].EntitledDays)
	assert.Equal(t, 1.0, got[0].TakenDays)
	assert.Equal(t, 1.0, got[0].ReservedDays)
	assert.Equal(t, 1.0, got[0].RemainingDays)

	rec := route.get(context.Background(), ownAllowancesPath+"?year=1999", token)
	testutil.AssertErrorResponse(t, rec, http.StatusBadRequest)
}

func TestOwnAbsenceAllowances_RequireTimeTrackingOwn(t *testing.T) {
	t.Parallel()
	route := setupOwnAllowancesRoute(t)
	_, account := testpkg.CreateTestStaffWithAccount(t, route.db, "Ohne", "Recht")

	rec := route.get(context.Background(), ownAllowancesPath, ownToken(t, account.ID, 0, "users:read"))
	testutil.AssertErrorResponse(t, rec, http.StatusForbidden)
}

func TestOwnAbsenceAllowances_EmptyWithoutAllowanceTypes(t *testing.T) {
	t.Parallel()
	route := setupOwnAllowancesRoute(t)
	ctx := testpkg.Ctx(t)
	_, account := testpkg.CreateTestStaffWithAccount(t, route.db, "Leer", "Liste")
	route.allowanceType(t, ctx, "Ohne Kontingent", false)

	got := decodeOwnAllowances(t, route.get(context.Background(), ownAllowancesPath+"?year=2026", ownToken(t, account.ID, 0, "time_tracking:own")))
	assert.Empty(t, got)
}

// Each school sees its own types only, through the tenant transaction.
func TestOwnAbsenceAllowances_StayInsideTheSchool(t *testing.T) {
	t.Parallel()
	route := setupOwnAllowancesRoute(t)
	ctx := testpkg.Ctx(t)
	staff, account := testpkg.CreateTestStaffWithAccount(t, route.db, "Schule", "Eins")
	admin := testpkg.CreateTestStaff(t, route.db, "Leitung", "Eins")
	own := route.allowanceType(t, ctx, "Regenerationstag", true)
	route.claim(t, ctx, staff.ID, own.ID, 4, admin.ID)

	foreignID, _ := testpkg.CreateTestTenant(t, route.db)
	foreignCtx := testpkg.TenantContext(foreignID)
	foreignStaff, foreignAccount := testpkg.CreateTestStaffWithAccountForTenant(t, route.db, foreignID, "Schule", "Zwei")
	foreignAdmin := testpkg.CreateTestStaffForTenant(t, route.db, foreignID, "Leitung", "Zwei")
	foreign := route.allowanceType(t, foreignCtx, "Krank-Urlaubstag", true)
	route.claim(t, foreignCtx, foreignStaff.ID, foreign.ID, 7, foreignAdmin.ID)

	got := decodeOwnAllowances(t, route.get(context.Background(), ownAllowancesPath+"?year=2026", ownToken(t, account.ID, 0, "time_tracking:own")))
	require.Len(t, got, 1)
	assert.Equal(t, "Regenerationstag", got[0].Name)
	assert.Equal(t, 4.0, got[0].EntitledDays)

	got = decodeOwnAllowances(t, route.get(context.Background(), ownAllowancesPath+"?year=2026", ownToken(t, foreignAccount.ID, foreignID, "time_tracking:own")))
	require.Len(t, got, 1)
	assert.Equal(t, "Krank-Urlaubstag", got[0].Name)
	assert.Equal(t, 7.0, got[0].EntitledDays)
}

// Rule 15 (#2940): the types, the claims and the absences are read once
// each, however many types with an allowance the school has.
func TestOwnAbsenceAllowances_QueryBudget(t *testing.T) {
	t.Parallel()
	route := setupOwnAllowancesRoute(t)
	ctx := testpkg.Ctx(t)
	staff, account := testpkg.CreateTestStaffWithAccount(t, route.db, "Budget", "Konten")
	admin := testpkg.CreateTestStaff(t, route.db, "Budget", "Leitung")
	token := ownToken(t, account.ID, 0, "time_tracking:own")
	addType := func(name, day string) {
		typ := route.allowanceType(t, ctx, name, true)
		route.claim(t, ctx, staff.ID, typ.ID, 5, admin.ID)
		route.book(t, ctx, staff.ID, typ.ID, day, workforce.AbsenceStatusApproved)
	}
	addType("Art A", "2026-02-02")
	addType("Art B", "2026-02-03")
	addType("Art C", "2026-02-04")

	measure := func(want int) []string {
		counter := testpkg.CaptureQueriesForContext(t, route.db)
		rec := route.get(counter.Context(context.Background()), ownAllowancesPath+"?year=2026", token)
		require.Len(t, decodeOwnAllowances(t, rec), want)
		queries := counter.Queries()
		counter.Stop()
		return queries
	}
	small := measure(3)
	addType("Art D", "2026-02-05")
	addType("Art E", "2026-02-06")
	large := measure(5)

	assert.Equal(t, len(small), len(large), "the list must not issue a statement per type")
	testpkg.AssertQueryBudget(t, "workforce.own_absence_allowances.list", large)
}
