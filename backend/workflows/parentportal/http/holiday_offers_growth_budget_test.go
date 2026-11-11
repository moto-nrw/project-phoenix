package parent_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	portalCompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
	parentHTTP "github.com/moto-nrw/project-phoenix/workflows/parentportal/http"
)

// holidayOfferPeriod is a holiday period of a Träger as the test writes it.
type holidayOfferPeriod struct {
	name, opensOn, closesOn string
	published               bool
}

// insertHolidayOfferPeriod writes the period with one care week from
// 15.10.2029 to 19.10.2029 hosted by school and returns its link token.
func insertHolidayOfferPeriod(t *testing.T, db *bun.DB, organizationID, school int64, period holidayOfferPeriod) string {
	t.Helper()
	ctx := context.Background()
	var id int64
	var token string
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_periods
			(organization_id, name, registration_opens_on, registration_closes_on, status, published_at)
		VALUES (?, ?, ?::date, ?::date, CASE WHEN ? THEN 'published' ELSE 'draft' END, CASE WHEN ? THEN statement_timestamp() END)
		RETURNING id, public_token`,
		organizationID, period.name, period.opensOn, period.closesOn, period.published, period.published).Scan(ctx, &id, &token))
	_, err := db.ExecContext(ctx, `INSERT INTO enrollment.holiday_period_weeks (organization_id, holiday_period_id, starts_on, ends_on, host_school_id)
		VALUES (?, ?, DATE '2029-10-15', DATE '2029-10-19', ?)`, organizationID, id, school)
	require.NoError(t, err)
	return token
}

// TestParentHolidayRegistrationOffers pins #6671 on the route the start page
// of the parents app reads: a parent sees the published holiday periods of
// the Träger of a child the account may register, until the registration
// closes. A draft, a closed period, a period of another Träger and an
// account whose relationship does not grant the registration stay out, and
// the statement count does not grow with the periods.
func TestParentHolidayRegistrationOffers(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	database := testpkg.NewDatabase(t, db)
	repos := api.NewRepositoryTestFactory(database)
	organizations, err := owners.NewOrganizationTenancy(database)
	require.NoError(t, err)
	today := calendar.NewDate(2029, time.September, 10)
	portal := portalCompose.New(portalCompose.Dependencies{
		GuardianPolicy: owners.NewParentGuardianPolicy(), DeparturePolicy: owners.ParentDeparturePolicy(), Fingerprints: owners.SecurityFingerprints{},
		Declarations: owners.ParentDeclarationPolicy(), Locales: owners.ParentPortalLocales(), Runtime: owners.ParentPortalRuntime{}, Logger: slog.Default(),
		ChildRepo: repos.ParentChildReads(), SchoolTraeger: owners.NewParentSchoolTraeger(organizations),
		HolidayAnnouncements: owners.NewParentHolidayAnnouncements(),
		Now:                  func() time.Time { return today.BerlinMidnight().Add(12 * time.Hour) },
	})
	router := testpkg.TenantRuntimeMiddleware(t, db)(parentHTTP.NewResource(parentHTTP.ResourceConfig{Parent: portal}).Router(api.ParentSessionChain))

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	pickupOnly := testpkg.CreateTestStudent(t, db, "Mia", "Schneider", "2b")
	testpkg.CreateTestStudentGuardianLink(t, db, pickupOnly.ID, chain.GuardianProfileID, "pickup_only")
	// A second account sees its child, but may not register it.
	viewer := testpkg.CreateTestParentGuardianChain(t, db)
	_, err = db.ExecContext(context.Background(), `UPDATE auth.guardian_student_access
		SET permissions = '{"parent_portal.access": true}'::jsonb WHERE account_id = ?`, viewer.AccountID)
	require.NoError(t, err)

	var own int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, chain.TenantID).Scan(context.Background(), &own))
	testpkg.OwnTraegerHolidayPeriods(t, db, own)
	unique := testpkg.UniqueTestTenantID(t)
	foreign := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Anderer Träger", Slug: fmt.Sprintf("holiday-offers-%d", unique)}).ID
	foreignSchool := testpkg.CreateTestSchoolInOrganization(t, db, foreign, "Schule eines anderen Trägers")
	testpkg.OwnTraegerHolidayPeriods(t, db, foreign)

	insertHolidayOfferPeriod(t, db, own, chain.TenantID, holidayOfferPeriod{name: "Entwurf", opensOn: "2029-09-01", closesOn: "2029-09-30"})
	insertHolidayOfferPeriod(t, db, own, chain.TenantID, holidayOfferPeriod{name: "Sommerferien", opensOn: "2029-06-01", closesOn: "2029-09-09", published: true})
	insertHolidayOfferPeriod(t, db, foreign, foreignSchool, holidayOfferPeriod{name: "Fremde Ferien", opensOn: "2029-09-01", closesOn: "2029-09-30", published: true})
	lastDay := insertHolidayOfferPeriod(t, db, own, chain.TenantID, holidayOfferPeriod{name: "Herbstferien", opensOn: "2029-09-01", closesOn: "2029-09-10", published: true})
	upcoming := insertHolidayOfferPeriod(t, db, own, chain.TenantID, holidayOfferPeriod{name: "Weihnachtsferien", opensOn: "2029-09-20", closesOn: "2029-10-05", published: true})

	counter := testpkg.CaptureQueries(t, db)
	list := func(account testpkg.ParentChain) ([]parentHTTP.HolidayRegistrationOfferResponse, testpkg.QueryMeasurement) {
		token := testutil.MintTestJWT(t, sessiontoken.AppClaims{ID: int(account.AccountID), Sub: account.Email, Roles: []string{"parent"}, Scope: "parent"})
		req := httptest.NewRequest(http.MethodGet, "/me/holiday-registrations", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		counter.Reset()
		testpkg.MeasuredHTTPRequest(counter, testpkg.MountedQueryBudgetRouter("/parent", router), rec, req)
		queries := counter.Queries()
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var body struct {
			Data []parentHTTP.HolidayRegistrationOfferResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		return body.Data, counter.Measurement(queries)
	}

	offers, small := list(chain)
	require.Len(t, offers, 2, "the open and the coming period of the child's Träger")
	firstDay, lastCareDay := calendar.NewDate(2029, time.October, 15), calendar.NewDate(2029, time.October, 19)
	child := []parentHTTP.HolidayRegistrationOfferChildResponse{{StudentID: fmt.Sprint(chain.StudentID), FirstName: "Felix"}}
	assert.Equal(t, parentHTTP.HolidayRegistrationOfferResponse{
		Token: lastDay, Name: "Herbstferien", TraegerName: offers[0].TraegerName,
		RegistrationOpensOn: calendar.NewDate(2029, time.September, 1), RegistrationClosesOn: today,
		FirstDay: &firstDay, LastDay: &lastCareDay, Open: true, Children: child,
	}, offers[0], "open until the end of the last day; only the child the account may register")
	assert.NotEmpty(t, offers[0].TraegerName)
	assert.Equal(t, upcoming, offers[1].Token)
	assert.False(t, offers[1].Open, "before the start of the registration")
	assert.Equal(t, calendar.NewDate(2029, time.September, 20), offers[1].RegistrationOpensOn)
	assert.Equal(t, child, offers[1].Children)

	none, _ := list(viewer)
	assert.Empty(t, none, "a relationship without the registration right opens no period")

	for i := range 5 {
		insertHolidayOfferPeriod(t, db, own, chain.TenantID, holidayOfferPeriod{name: fmt.Sprintf("Ferien %d", i), opensOn: "2029-09-01", closesOn: "2029-09-30", published: true})
	}
	offers, large := list(chain)
	require.Len(t, offers, 7)
	require.Equal(t, len(small.Statements), len(large.Statements), "the statements do not grow with the periods")
	querybudgets.AssertQueryBudget(t, "parent.holiday_registrations.growth.mine", large)
}
