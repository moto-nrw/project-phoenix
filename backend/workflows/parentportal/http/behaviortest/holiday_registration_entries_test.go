package parent_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	parentService "github.com/moto-ogs/project-phoenix/workflows/parentportal"
	portalCompose "github.com/moto-ogs/project-phoenix/workflows/parentportal/compose"
	parentHTTP "github.com/moto-ogs/project-phoenix/workflows/parentportal/http"
)

// holidayEntriesPortal is the parents portal over the real Enrollment,
// Organisation & Tenancy and Identity & Access, with the route of the
// account's holiday registrations.
func holidayEntriesPortal(t *testing.T, db *bun.DB) (*parentService.Portal, http.Handler) {
	t.Helper()
	database := testpkg.NewDatabase(t, db)
	organizations, err := owners.NewOrganizationTenancy(database)
	require.NoError(t, err)
	identity, err := api.NewIdentityAccessForTests(database)
	require.NoError(t, err)
	portal := portalCompose.New(portalCompose.Dependencies{
		GuardianPolicy: owners.NewParentGuardianPolicy(), DeparturePolicy: owners.ParentDeparturePolicy(), Fingerprints: owners.SecurityFingerprints{},
		Declarations: owners.ParentDeclarationPolicy(), Locales: owners.ParentPortalLocales(), Runtime: owners.ParentPortalRuntime{}, Logger: slog.Default(),
		ChildRepo: api.NewRepositoryTestFactory(database).ParentChildReads(), ParentAccounts: owners.NewParentAccounts(identity),
		HolidayRegistrations: owners.NewParentHolidayRegistrations(organizations),
	})
	router := testpkg.TenantRuntimeMiddleware(t, db)(parentHTTP.NewResource(parentHTTP.ResourceConfig{Parent: portal}).Router(api.ParentSessionChain))
	return portal, router
}

// holidayTraeger is a Träger with one school that hosts its holiday weeks.
func holidayTraeger(t *testing.T, db *bun.DB, name string) (int64, int64) {
	t.Helper()
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: name, Slug: fmt.Sprintf("ferien-eintraege-%d", testpkg.UniqueTestTenantID(t)), Active: true,
	}).ID
	return organization, testpkg.CreateTestSchoolInOrganization(t, db, organization, name+" OGS")
}

func linkedAccount(t *testing.T, db *bun.DB, registrationID int64) *int64 {
	t.Helper()
	var account *int64
	require.NoError(t, db.NewRaw(`SELECT parent_account_id FROM enrollment.holiday_registrations WHERE id = ?`, registrationID).Scan(context.Background(), &account))
	return account
}

// TestHolidayRegistrationEntriesShowTheAccountsOwnRegistrations pins ADR
// 0216 points 3 and 9: the sign-in claims the registrations without an
// account that name its address, without regard to case, and only those;
// the route lists the account's registrations of every Träger, newest
// first, with the first name and the weeks of each child and nothing of a
// child's health, address, contacts, last name or date of birth. Another
// account sees none of them, and the statements do not grow with the
// registrations.
func TestHolidayRegistrationEntriesShowTheAccountsOwnRegistrations(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	portal, router := holidayEntriesPortal(t, db)
	// The isolated clone carries its own runtime for the sign-in's claim.
	ctx := tenant.WithUnitOfWork(context.Background(), testpkg.TenantRuntime(t, db))
	north, northSchool := holidayTraeger(t, db, "Träger Nord Ferien")
	south, southSchool := holidayTraeger(t, db, "Träger Süd Ferien")
	eva := testpkg.CreateTestAccount(t, db, "ferien-eva")
	tom := testpkg.CreateTestAccount(t, db, "ferien-tom")

	autumn := testpkg.CreateTestHolidayRegistration(t, db, north, northSchool, "Herbstferien 2029", strings.ToUpper(eva.Email), "Lia", "Ben")
	winter := testpkg.CreateTestHolidayRegistration(t, db, south, southSchool, "Winterferien 2029", eva.Email, "Lia")
	taken := testpkg.CreateTestHolidayRegistration(t, db, north, northSchool, "Osterferien 2030", eva.Email, "Mia")
	foreign := testpkg.CreateTestHolidayRegistration(t, db, north, northSchool, "Sommerferien 2030", tom.Email, "Tim")
	_, err := db.ExecContext(ctx, `UPDATE enrollment.holiday_registrations SET parent_account_id = ? WHERE id = ?`, tom.ID, taken.RegistrationID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE enrollment.holiday_registrations SET submitted_at = submitted_at - INTERVAL '1 day' WHERE id = ?`, autumn.RegistrationID)
	require.NoError(t, err)

	portal.ClaimHolidayRegistrationsAfterSignIn(ctx, eva.Email)
	require.NotNil(t, linkedAccount(t, db, autumn.RegistrationID), "the address claims the registration, whatever its case")
	assert.Equal(t, eva.ID, *linkedAccount(t, db, autumn.RegistrationID))
	assert.Equal(t, eva.ID, *linkedAccount(t, db, winter.RegistrationID), "at every Träger")
	assert.Equal(t, tom.ID, *linkedAccount(t, db, taken.RegistrationID), "a registration with an account keeps it")
	assert.Nil(t, linkedAccount(t, db, foreign.RegistrationID), "another address claims nothing")
	portal.ClaimHolidayRegistrationsAfterSignIn(ctx, "niemand@example.org")

	counter := testpkg.CaptureQueries(t, db)
	list := func(account *testpkg.AccountFixture) (string, testpkg.QueryMeasurement) {
		token := testutil.MintTestJWT(t, sessiontoken.AppClaims{ID: int(account.ID), Sub: account.Email, Roles: []string{"parent"}, Scope: "parent"})
		req := httptest.NewRequest(http.MethodGet, "/me/holiday-registration-entries", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		counter.Reset()
		testpkg.MeasuredHTTPRequest(counter, testpkg.MountedQueryBudgetRouter("/parent", router), rec, req)
		queries := counter.Queries()
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return rec.Body.String(), counter.Measurement(queries)
	}
	decode := func(body string) []parentHTTP.HolidayRegistrationEntryResponse {
		var envelope struct {
			Data parentHTTP.HolidayRegistrationEntriesResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal([]byte(body), &envelope))
		return envelope.Data.Registrations
	}

	body, small := list(eva)
	entries := decode(body)
	require.Len(t, entries, 2)
	first, last := calendar.NewDate(2029, time.October, 15), calendar.NewDate(2029, time.October, 19)
	week := parentHTTP.HolidayRegistrationEntryWeekResponse{FirstDay: first, LastDay: last, TimeModelName: "bis 16 Uhr", Placement: "placed"}
	assert.Equal(t, winter.RegistrationID, entries[0].ID, "newest first")
	assert.Equal(t, "Träger Süd Ferien", entries[0].TraegerName)
	assert.Equal(t, "Winterferien 2029", entries[0].PeriodName)
	assert.Equal(t, &first, entries[0].FirstDay)
	assert.Equal(t, &last, entries[0].LastDay)
	assert.Equal(t, "received", entries[0].Status)
	assert.Equal(t, []parentHTTP.HolidayRegistrationEntryChildResponse{{FirstName: "Lia", Weeks: []parentHTTP.HolidayRegistrationEntryWeekResponse{week}}}, entries[0].Children)
	assert.Equal(t, autumn.RegistrationID, entries[1].ID)
	assert.Equal(t, "Träger Nord Ferien", entries[1].TraegerName)
	assert.Equal(t, []string{"Lia", "Ben"}, []string{entries[1].Children[0].FirstName, entries[1].Children[1].FirstName})
	for _, withheld := range []string{"Muster", "2019-03-01", "Grundschule Melle", "0151", "Mia", "Tim", "Osterferien", "Sommerferien"} {
		assert.NotContains(t, body, withheld, "the list carries no last name, date of birth, school, contact or foreign registration")
	}

	tomBody, _ := list(tom)
	tomEntries := decode(tomBody)
	require.Len(t, tomEntries, 1, "another account sees only its own registration")
	assert.Equal(t, taken.RegistrationID, tomEntries[0].ID)

	for i := range 4 {
		more := testpkg.CreateTestHolidayRegistration(t, db, south, southSchool, fmt.Sprintf("Ferien %d", i), eva.Email, "Lia", "Ben", "Ida")
		_, err := db.ExecContext(ctx, `UPDATE enrollment.holiday_registrations SET parent_account_id = ? WHERE id = ?`, eva.ID, more.RegistrationID)
		require.NoError(t, err)
	}
	body, large := list(eva)
	require.Len(t, decode(body), 6)
	require.Equal(t, len(small.Statements), len(large.Statements), "the statements do not grow with the registrations")
	querybudgets.AssertQueryBudget(t, "parent.holiday_registration_entries.growth.mine", large)
}

// An account without a linked registration gets an empty list, not an
// error: the Eltern-App shows nothing then.
func TestHolidayRegistrationEntriesAreEmptyWithoutARegistration(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	_, router := holidayEntriesPortal(t, db)
	account := testpkg.CreateTestAccount(t, db, "ferien-leer")
	token := testutil.MintTestJWT(t, sessiontoken.AppClaims{ID: int(account.ID), Sub: account.Email, Roles: []string{"parent"}, Scope: "parent"})
	req := httptest.NewRequest(http.MethodGet, "/me/holiday-registration-entries", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	assert.JSONEq(t, `{"registrations": []}`, string(envelope.Data))
}
