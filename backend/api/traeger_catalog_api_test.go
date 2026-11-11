package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// catalogPhaseOfferings is Care Plan as the phase administration counts
// and lists the offerings of a phase.
type catalogPhaseOfferings struct{ carePlan careplan.Capability }

func (o catalogPhaseOfferings) OfferingIDsForPhase(ctx context.Context, phaseID int64) ([]int64, error) {
	offerings, err := o.carePlan.ListCareOfferings(ctx, careplan.CareOfferingFilter{PhaseIDs: []int64{phaseID}, Order: careplan.OfferingOrderCatalog})
	ids := make([]int64, 0, len(offerings))
	for _, offering := range offerings {
		ids = append(ids, offering.ID)
	}
	return ids, err
}

func (o catalogPhaseOfferings) CountOfferingsForPhase(ctx context.Context, phaseID int64) (int, error) {
	return o.carePlan.CountCareOfferingsByPhase(ctx, phaseID)
}

func (o catalogPhaseOfferings) CareOfferingBookingsForPhase(context.Context, int64) ([]enrollment.OfferingBooking, error) {
	return nil, nil
}

type catalogNoResync struct{}

func (catalogNoResync) ResyncTemplatesSourcedFromOffering(context.Context, int64, calendar.Date) error {
	return nil
}

func (catalogNoResync) DetachTemplatesSourcedFromOffering(context.Context, int64, calendar.Date) error {
	return nil
}

// traegerCatalogRouter mounts /traeger/catalog as the serving root does,
// over the phase administration, the care-offering catalog, the
// Elternabrechnung and the Betreuungsvertrag of the test database.
func traegerCatalogRouter(t *testing.T, db *bun.DB) http.Handler {
	t.Helper()
	unit := testpkg.TenantRuntime(t, db)
	database := testpkg.NewDatabase(t, db)
	organizations, err := owners.NewOrganizationTenancy(database)
	require.NoError(t, err)
	audit, err := auditCompose.NewTenantPersistence(db)
	require.NoError(t, err)
	settings, err := owners.BuildTraegerSettings(database, unit, slog.Default(), organizations, nil)
	require.NoError(t, err)
	schoolSettings, err := owners.NewSettingsTestModule(database, unit)
	require.NoError(t, err)
	presence, err := presenceCompose.New(presenceCompose.Dependencies{DB: db, Observe: func(presenceCompose.Observation) {}})
	require.NoError(t, err)
	delivery, err := deliveryCompose.NewModule(deliveryCompose.ModuleDependencies{
		DB: db, People: owners.EnrollmentFlowGuardianDirectory{}, Observe: func(deliveryContract.Observation) {},
	})
	require.NoError(t, err)
	billing, intake := newTraegerCatalogBilling(organizations, settings, schoolSettings.Settings, presence, delivery.Module,
		providerBlockOutcomes{delivery.EmailOutcomes}, audit.DataAccessLog, owners.CareContractTestHolidays{}, "http://parents.localhost:3000")
	catalog, err := NewCareOfferingCatalog(database, unit)
	require.NoError(t, err)
	noLock := func(context.Context) error { return nil }
	phases, err := enrollmentCompose.NewResyncingPhases(enrollmentCompose.PhaseDependencies{
		Records:                NewEnrollmentSuiteRepositories(database).Enrollment(),
		Offerings:              catalogPhaseOfferings{carePlan: NewRepositoryTestFactory(database).CarePlan()},
		LockTemplateRecurrence: noLock, Resyncer: catalogNoResync{},
	})
	require.NoError(t, err)
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(tenant.WithUnitOfWork(r.Context(), unit)))
		})
	})
	require.NoError(t, mountTraegerCatalogRoutes(router, phases, catalog.Catalog, noLock, organizations, billing, intake))
	return testpkg.SessionVerifier(router)
}

type catalogField struct {
	Field string `json:"field"`
	State string `json:"state"`
}

type catalogPreviewBody struct {
	Fingerprint string `json:"fingerprint"`
	Schools     []struct {
		SchoolID  int64  `json:"school_id"`
		Action    string `json:"action"`
		Outcome   string `json:"outcome"`
		ErrorCode string `json:"error_code"`
		Offerings []struct {
			Fields     []catalogField `json:"fields"`
			PricesFrom *string        `json:"prices_from"`
		} `json:"offerings"`
	} `json:"schools"`
}

type catalogResult struct {
	SchoolID int64  `json:"school_id"`
	Outcome  string `json:"outcome"`
}

type catalogRolloutBody struct {
	Schools []catalogResult `json:"schools"`
}

type catalogPhaseBody struct {
	ID      int64 `json:"id"`
	Schools []struct {
		SchoolID       int64  `json:"school_id"`
		Outcome        string `json:"outcome"`
		AppliedVersion *int   `json:"applied_version"`
	} `json:"schools"`
	Offerings []struct {
		ID int64 `json:"id"`
	} `json:"offerings"`
}

func catalogOffering(capacity int, mondayCents int64) map[string]any {
	return map[string]any{
		"fields": map[string]any{
			"name": "Frühbetreuung", "days_of_week_mode": "fixed", "available_days": []string{"mon", "tue"},
			"capacity": capacity, "counts_as_care": true, "selection_rule": "optional",
			"pickup_times": map[string]string{"mon": "08:30", "tue": "08:30"},
		},
		"prices":            []map[string]any{{"weekday": "mon", "monthly_cents": mondayCents}, {"weekday": "tue", "monthly_cents": 1600}},
		"contract_template": map[string]any{"term_kind": "school_year", "notice_months": 1, "notice_to": "month_end"},
	}
}

// The Träger-Büro keeps one phase with one offering for all its schools
// and rolls it out (#4704, ADR 0195): every school gets a phase and an
// offering of its own with tariff and contract template; a school's own
// change stays through the next rollout; a stale preview is refused; and
// another Träger or a role without the right reaches nothing.
func TestTraegerCatalogRollsOutIntoEverySchoolAndKeepsTheSchoolsChanges(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	portal := traegerPortalWithDays(t, db, testpkg.TenantRuntime(t, db), owners.HolidayTestSchoolCalendar{})
	office := newTraegerOffice(t, db, portal, "Träger Katalog", identityaccess.TraegerRoleAdministration)
	testpkg.OwnTraegerCatalog(t, db, office.organization)
	testpkg.OwnTraegerCareContracts(t, db, office.organization)
	testpkg.OwnTenantRows(t, db, office.schools...)
	router := traegerCatalogRouter(t, db)

	rec := traegerJSON(t, router, office.token, http.MethodPost, "/traeger/catalog/phases", map[string]any{
		"name": "Schuljahr 2027/28", "kind": "school_year", "service_start_date": "2027-08-01", "service_end_date": "2028-07-31",
		"all_schools": true,
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	phase := decodeSchoolPortalData[catalogPhaseBody](t, rec.Body.Bytes())
	phasePath := fmt.Sprintf("/traeger/catalog/phases/%d", phase.ID)
	rec = traegerJSON(t, router, office.token, http.MethodPost, phasePath+"/offerings", catalogOffering(20, 1600))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	offering := decodeSchoolPortalData[careplan.TraegerOffering](t, rec.Body.Bytes())

	preview := catalogPreview(t, router, office.token, phasePath)
	require.Len(t, preview.Schools, 2)
	for _, school := range preview.Schools {
		assert.Equal(t, "ready", school.Outcome, school.ErrorCode)
		assert.Equal(t, "create", school.Action)
	}
	rollout := catalogRollout(t, router, office.token, phasePath, preview.Fingerprint)
	assert.Equal(t, []catalogResult{{SchoolID: office.schools[0], Outcome: "applied"}, {SchoolID: office.schools[1], Outcome: "applied"}}, rollout.Schools)

	ctx := context.Background()
	for _, school := range office.schools {
		var copyID int64
		require.NoError(t, db.NewRaw(`SELECT id FROM enrollment.care_offerings WHERE tenant_id = ? AND traeger_source_id = ?`,
			school, offering.ID).Scan(ctx, &copyID), "school %d has its copy", school)
		var tariffs, templates int
		require.NoError(t, db.NewRaw(`SELECT count(*) FROM parent_billing.care_tariffs WHERE school_id = ? AND care_offering_id = ?`, school, copyID).Scan(ctx, &tariffs))
		require.NoError(t, db.NewRaw(`SELECT count(*) FROM care_contract.contract_templates WHERE school_id = ? AND care_offering_id = ?`, school, copyID).Scan(ctx, &templates))
		assert.Equal(t, 2, tariffs, "school %d has a price per weekday", school)
		assert.Equal(t, 1, templates, "school %d has its contract template", school)
	}

	// The first school deactivates its copy; the office raises the capacity
	// and the Monday price.
	_, err := db.ExecContext(ctx, `UPDATE enrollment.care_offerings SET is_active = false WHERE tenant_id = ? AND traeger_source_id = ?`,
		office.schools[0], offering.ID)
	require.NoError(t, err)
	rec = traegerJSON(t, router, office.token, http.MethodPut, fmt.Sprintf("/traeger/catalog/offerings/%d", offering.ID), catalogOffering(30, 1800))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	stale := catalogPreview(t, router, office.token, phasePath)
	rec = traegerJSON(t, router, office.token, http.MethodPut, fmt.Sprintf("/traeger/catalog/offerings/%d", offering.ID), catalogOffering(30, 1900))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = traegerJSON(t, router, office.token, http.MethodPost, phasePath+"/rollout", map[string]any{"fingerprint": stale.Fingerprint})
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "enrollment.traeger_preview_stale")

	preview = catalogPreview(t, router, office.token, phasePath)
	states := map[string]string{}
	for _, field := range preview.Schools[0].Offerings[0].Fields {
		states[field.Field] = field.State
	}
	assert.Equal(t, "keeps", states["is_active"], "the preview shows the school's deactivation next to the Träger")
	assert.Equal(t, "changes", states["capacity"])
	require.NotNil(t, preview.Schools[0].Offerings[0].PricesFrom, "a changed price is a new tariff")
	catalogRollout(t, router, office.token, phasePath, preview.Fingerprint)
	var active bool
	var capacity int
	require.NoError(t, db.NewRaw(`SELECT is_active, capacity FROM enrollment.care_offerings WHERE tenant_id = ? AND traeger_source_id = ?`,
		office.schools[0], offering.ID).Scan(ctx, &active, &capacity))
	assert.False(t, active, "the school's deactivation stays")
	assert.Equal(t, 30, capacity, "the Träger's capacity arrives")

	// Another Träger and a role without „Anmeldung verwalten“ reach nothing.
	foreign := newTraegerOffice(t, db, portal, "Träger Katalog Fremd", identityaccess.TraegerRoleAdministration)
	rec = traegerJSON(t, router, foreign.token, http.MethodGet, phasePath, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	rec = traegerJSON(t, router, foreign.token, http.MethodPost, phasePath+"/preview", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	billingOffice := newTraegerOffice(t, db, portal, "Träger Katalog Abrechnung", identityaccess.TraegerRoleBilling)
	rec = traegerJSON(t, router, billingOffice.token, http.MethodGet, "/traeger/catalog/phases", nil)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	// The office withdraws the offering: copies without bookings go.
	rec = traegerJSON(t, router, office.token, http.MethodPost, fmt.Sprintf("/traeger/catalog/offerings/%d/withdraw", offering.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	preview = catalogPreview(t, router, office.token, phasePath)
	catalogRollout(t, router, office.token, phasePath, preview.Fingerprint)
	var copies int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM enrollment.care_offerings WHERE tenant_id IN (?) AND traeger_source_id = ?`,
		bun.List(office.schools), offering.ID).Scan(ctx, &copies))
	assert.Zero(t, copies, "a withdrawn offering leaves the schools")
}

func catalogPreview(t *testing.T, router http.Handler, token, phasePath string) catalogPreviewBody {
	t.Helper()
	rec := traegerJSON(t, router, token, http.MethodPost, phasePath+"/preview", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[catalogPreviewBody](t, rec.Body.Bytes())
}

func catalogRollout(t *testing.T, router http.Handler, token, phasePath, fingerprint string) catalogRolloutBody {
	t.Helper()
	rec := traegerJSON(t, router, token, http.MethodPost, phasePath+"/rollout", map[string]any{"fingerprint": fingerprint})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[catalogRolloutBody](t, rec.Body.Bytes())
}
