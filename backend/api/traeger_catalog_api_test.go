package api

import (
	"context"
	"errors"
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
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
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
func traegerCatalogRouter(t *testing.T, db *bun.DB, tariffOverride ...parentbilling.CareModulePrices) http.Handler {
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
	if len(tariffOverride) > 0 {
		billing = tariffOverride[0]
	}
	bookings, err := NewBookingMaterialization(database, unit)
	require.NoError(t, err)
	catalog, err := NewCareOfferingCatalog(database, unit, WithCareOfferingRosterResync(bookings.Bookings), WithCareOfferingPickupResync(bookings.Bookings))
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
	ErrorCode string `json:"error_code"`
	SchoolID  int64  `json:"school_id"`
	Outcome   string `json:"outcome"`
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

type catalogFailedTariffs struct{ err error }

func (f catalogFailedTariffs) SaveCareModulePrices(context.Context, parentbilling.HolidayReviewer, []parentbilling.CareModulePriceList) error {
	return f.err
}

func TestTraegerCatalogRecoversCopiesAfterBillingFailure(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name     string
		failure  error
		status   int
		deselect bool
	}{
		{name: "refusal then withdrawal", failure: parentbilling.ErrCareTariffMonthBilled, status: http.StatusOK},
		{name: "interruption then deselection", failure: errors.New("billing unavailable"), status: http.StatusInternalServerError, deselect: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			db := testpkg.SetupTestDB(t)
			portal := traegerPortalWithDays(t, db, testpkg.TenantRuntime(t, db), owners.HolidayTestSchoolCalendar{})
			office := newTraegerOffice(t, db, portal, "Träger Katalog Wiederholung", identityaccess.TraegerRoleAdministration)
			testpkg.OwnTraegerCatalog(t, db, office.organization)
			testpkg.OwnTenantRows(t, db, office.schools...)
			router := traegerCatalogRouter(t, db, catalogFailedTariffs{err: scenario.failure})
			fields := map[string]any{
				"name": "Schuljahr 2027/28", "kind": "school_year", "service_start_date": "2027-08-01", "service_end_date": "2028-07-31", "all_schools": true,
			}
			rec := traegerJSON(t, router, office.token, http.MethodPost, "/traeger/catalog/phases", fields)
			require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
			phase := decodeSchoolPortalData[catalogPhaseBody](t, rec.Body.Bytes())
			path := fmt.Sprintf("/traeger/catalog/phases/%d", phase.ID)
			rec = traegerJSON(t, router, office.token, http.MethodPost, path+"/offerings", catalogOffering(20, 1600))
			require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
			preview := catalogPreview(t, traegerCatalogRouter(t, db), office.token, path)
			rec = traegerJSON(t, router, office.token, http.MethodPost, path+"/rollout", map[string]any{"fingerprint": preview.Fingerprint})
			require.Equal(t, scenario.status, rec.Code, rec.Body.String())
			if scenario.status == http.StatusOK {
				result := decodeSchoolPortalData[catalogRolloutBody](t, rec.Body.Bytes())
				for _, school := range result.Schools {
					assert.Equal(t, "rejected", school.Outcome)
				}
				assert.Contains(t, rec.Body.String(), "parent_billing.care_tariff_month_billed")
			}
			var copies, applied int
			require.NoError(t, db.NewRaw(`SELECT count(*) FROM enrollment.phases WHERE tenant_id IN (?) AND traeger_source_id = ?`, bun.List(office.schools), phase.ID).Scan(t.Context(), &copies))
			assert.Positive(t, copies, "copies committed before billing failed")
			require.NoError(t, db.NewRaw(`SELECT count(*) FROM enrollment.traeger_phase_schools WHERE organization_id = ? AND traeger_phase_id = ? AND applied_version IS NOT NULL`, office.organization, phase.ID).Scan(t.Context(), &applied))
			assert.Zero(t, applied, "billing failure does not advance the applied version")
			if scenario.deselect {
				fields["all_schools"] = false
				fields["school_ids"] = []int64{}
				rec = traegerJSON(t, router, office.token, http.MethodPut, path, fields)
			} else {
				rec = traegerJSON(t, router, office.token, http.MethodPost, path+"/withdraw", nil)
			}
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			preview = catalogPreview(t, router, office.token, path)
			require.Len(t, preview.Schools, len(office.schools), "every interrupted target remains recoverable")
			result := catalogRollout(t, router, office.token, path, preview.Fingerprint)
			for _, school := range result.Schools {
				assert.Equal(t, "withdrawn", school.Outcome)
			}
			require.NoError(t, db.NewRaw(`SELECT count(*) FROM enrollment.phases WHERE tenant_id IN (?) AND traeger_source_id = ?`, bun.List(office.schools), phase.ID).Scan(t.Context(), &copies))
			assert.Zero(t, copies, "withdrawal removes unfinished copies")
		})
	}
}

func TestTraegerCatalogChangesSelectionGroupsTogether(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	portal := traegerPortalWithDays(t, db, testpkg.TenantRuntime(t, db), owners.HolidayTestSchoolCalendar{})
	office := newTraegerOffice(t, db, portal, "Träger Auswahlgruppe", identityaccess.TraegerRoleAdministration)
	testpkg.OwnTraegerCatalog(t, db, office.organization)
	testpkg.OwnTenantRows(t, db, office.schools...)
	router := traegerCatalogRouter(t, db)
	rec := traegerJSON(t, router, office.token, http.MethodPost, "/traeger/catalog/phases", map[string]any{
		"name": "Wahlphase", "kind": "school_year", "service_start_date": "2027-08-01", "service_end_date": "2028-07-31", "all_schools": true,
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	phase := decodeSchoolPortalData[catalogPhaseBody](t, rec.Body.Bytes())
	path := fmt.Sprintf("/traeger/catalog/phases/%d", phase.ID)
	ids := []int64{}
	for index := range 2 {
		draft := catalogOffering(20, 1600)
		fields := draft["fields"].(map[string]any)
		fields["name"] = fmt.Sprintf("Wahlmodul %d", index)
		fields["selection_group"] = "Betreuung"
		delete(draft, "prices")
		delete(draft, "contract_template")
		rec = traegerJSON(t, router, office.token, http.MethodPost, path+"/offerings", draft)
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		ids = append(ids, decodeSchoolPortalData[careplan.TraegerOffering](t, rec.Body.Bytes()).ID)
	}
	preview := catalogPreview(t, router, office.token, path)
	catalogRollout(t, router, office.token, path, preview.Fingerprint)
	for index, id := range ids {
		draft := catalogOffering(20, 1600)
		fields := draft["fields"].(map[string]any)
		fields["name"] = fmt.Sprintf("Wahlmodul %d", index)
		fields["selection_group"], fields["selection_rule"] = "Betreuung", "exactly_one"
		delete(draft, "prices")
		delete(draft, "contract_template")
		rec = traegerJSON(t, router, office.token, http.MethodPut, fmt.Sprintf("/traeger/catalog/offerings/%d", id), draft)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	}
	preview = catalogPreview(t, router, office.token, path)
	for _, school := range preview.Schools {
		assert.Equal(t, "ready", school.Outcome, school.ErrorCode)
	}
	result := catalogRollout(t, router, office.token, path, preview.Fingerprint)
	for _, school := range result.Schools {
		assert.Equal(t, "applied", school.Outcome)
	}
}

func TestTraegerCatalogStatementCountDoesNotGrowWithOfferings(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	portal := traegerPortalWithDays(t, db, testpkg.TenantRuntime(t, db), owners.HolidayTestSchoolCalendar{})
	office := newTraegerOffice(t, db, portal, "Träger Stapelprüfung", identityaccess.TraegerRoleAdministration)
	testpkg.OwnTraegerCatalog(t, db, office.organization)
	testpkg.OwnTraegerCareContracts(t, db, office.organization)
	testpkg.OwnTenantRows(t, db, office.schools...)
	router := traegerCatalogRouter(t, db)
	counter := testpkg.CaptureQueriesForContext(t, db)
	counted := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		router.ServeHTTP(w, r.WithContext(counter.Context(r.Context())))
	})
	var previews, rollouts, updates, withdrawals []int
	for _, size := range []int{1, 5} {
		rec := traegerJSON(t, router, office.token, http.MethodPost, "/traeger/catalog/phases", map[string]any{
			"name": fmt.Sprintf("Stapel %d", size), "kind": "school_year", "service_start_date": "2027-08-01", "service_end_date": "2028-07-31", "all_schools": true,
		})
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		phase := decodeSchoolPortalData[catalogPhaseBody](t, rec.Body.Bytes())
		path := fmt.Sprintf("/traeger/catalog/phases/%d", phase.ID)
		ids := []int64{}
		for index := range size {
			draft := catalogOffering(20, 1600)
			draft["fields"].(map[string]any)["name"] = fmt.Sprintf("Modul %d", index)
			rec = traegerJSON(t, router, office.token, http.MethodPost, path+"/offerings", draft)
			require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
			ids = append(ids, decodeSchoolPortalData[careplan.TraegerOffering](t, rec.Body.Bytes()).ID)
		}
		counter.Reset()
		preview := catalogPreview(t, counted, office.token, path)
		previews = append(previews, counter.Total())
		counter.Reset()
		result := catalogRollout(t, counted, office.token, path, preview.Fingerprint)
		rollouts = append(rollouts, counter.Total())
		for _, school := range result.Schools {
			require.Equal(t, "applied", school.Outcome, school.ErrorCode)
		}
		// Every offering feeds a distinct live template in each school.
		// Counting only empty catalogs misses per-template reads and source writes.
		for _, schoolID := range office.schools {
			var copies []struct{ ID int64 }
			err := db.NewSelect().Table("enrollment.care_offerings").Column("id").Where("tenant_id = ?", schoolID).Where("traeger_source_id IN (?)", bun.List(ids)).Order("id").Scan(t.Context(), &copies)
			require.NoError(t, err)
			require.Len(t, copies, size)
			for _, copy := range copies {
				group := testpkg.CreateTestActivityGroupForTenant(t, db, schoolID, fmt.Sprintf("Quelle %d", copy.ID))
				_, err := db.NewUpdate().Table("activities.groups").Set("is_template = TRUE").Set("target_group_type = ?", "angebot").Set("source_care_offering_ids = ?::jsonb", fmt.Sprintf("[%d]", copy.ID)).Where("tenant_id = ?", schoolID).Where("id = ?", group.ID).Exec(t.Context())
				require.NoError(t, err)
			}
		}
		for index, id := range ids {
			draft := catalogOffering(30, 1800)
			draft["fields"].(map[string]any)["name"] = fmt.Sprintf("Modul %d", index)
			draft["contract_template"].(map[string]any)["contract_partner"] = "Aktualisierter Vertragspartner"
			rec = traegerJSON(t, router, office.token, http.MethodPut, fmt.Sprintf("/traeger/catalog/offerings/%d", id), draft)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		}
		preview = catalogPreview(t, router, office.token, path)
		counter.Reset()
		result = catalogRollout(t, counted, office.token, path, preview.Fingerprint)
		updates = append(updates, counter.Total())
		for _, school := range result.Schools {
			require.Equal(t, "applied", school.Outcome, school.ErrorCode)
		}
		rec = traegerJSON(t, router, office.token, http.MethodPost, path+"/withdraw", nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		preview = catalogPreview(t, router, office.token, path)
		counter.Reset()
		result = catalogRollout(t, counted, office.token, path, preview.Fingerprint)
		withdrawals = append(withdrawals, counter.Total())
		for _, school := range result.Schools {
			require.Equal(t, "withdrawn", school.Outcome)
		}
	}
	assert.Equal(t, previews[0], previews[1], "preview statement count")
	assert.Equal(t, rollouts[0], rollouts[1], "creation includes tariffs and templates")
	assert.Equal(t, updates[0], updates[1], "updates include tariffs and templates")
	assert.Equal(t, withdrawals[0], withdrawals[1], "withdrawal statement count")
	t.Logf("1/5 offerings: preview %v, create %v, update %v, withdraw %v", previews, rollouts, updates, withdrawals)
}

func TestTraegerCatalogFinancialWritesAreAtomic(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	portal := traegerPortalWithDays(t, db, testpkg.TenantRuntime(t, db), owners.HolidayTestSchoolCalendar{})
	office := newTraegerOffice(t, db, portal, "Träger Finanzprüfung", identityaccess.TraegerRoleAdministration)
	testpkg.OwnTraegerCatalog(t, db, office.organization)
	testpkg.OwnTraegerCareContracts(t, db, office.organization)
	testpkg.OwnTenantRows(t, db, office.schools...)
	router := traegerCatalogRouter(t, db)
	rec := traegerJSON(t, router, office.token, http.MethodPost, "/traeger/catalog/phases", map[string]any{
		"name": "Finanzprüfung", "kind": "school_year", "service_start_date": "2027-08-01", "service_end_date": "2028-07-31", "all_schools": true,
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	phase := decodeSchoolPortalData[catalogPhaseBody](t, rec.Body.Bytes())
	path := fmt.Sprintf("/traeger/catalog/phases/%d", phase.ID)
	draft := catalogOffering(20, 1600)
	draft["contract_template"].(map[string]any)["contract_partner"] = "Ursprüngliche Bedingungen"
	rec = traegerJSON(t, router, office.token, http.MethodPost, path+"/offerings", draft)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	offering := decodeSchoolPortalData[careplan.TraegerOffering](t, rec.Body.Bytes())
	preview := catalogPreview(t, router, office.token, path)
	catalogRollout(t, router, office.token, path, preview.Fingerprint)

	dearer := catalogOffering(30, 1800)
	dearer["contract_template"].(map[string]any)["contract_partner"] = "Abgelehnte Bedingungen"
	rec = traegerJSON(t, router, office.token, http.MethodPut, fmt.Sprintf("/traeger/catalog/offerings/%d", offering.ID), dearer)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	preview = catalogPreview(t, router, office.token, path)
	_, err := db.ExecContext(t.Context(), `CREATE FUNCTION care_contract.refuse_catalog_template_test() RETURNS trigger LANGUAGE plpgsql AS $$
  BEGIN IF NEW.contract_partner = 'Abgelehnte Bedingungen' THEN RAISE EXCEPTION 'template write refused'; END IF; RETURN NEW; END $$;
  CREATE TRIGGER refuse_catalog_template_test BEFORE INSERT OR UPDATE ON care_contract.contract_templates
  FOR EACH ROW EXECUTE FUNCTION care_contract.refuse_catalog_template_test()`)
	require.NoError(t, err)
	rec = traegerJSON(t, router, office.token, http.MethodPost, path+"/rollout", map[string]any{"fingerprint": preview.Fingerprint})
	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	checkTerms := func() {
		t.Helper()
		for _, school := range office.schools {
			var cents int64
			var partner string
			require.NoError(t, db.NewRaw(`SELECT tariff.monthly_cents, template.contract_partner FROM enrollment.care_offerings AS offering
    JOIN parent_billing.care_tariffs AS tariff ON tariff.school_id=offering.tenant_id AND tariff.care_offering_id=offering.id AND tariff.organization_id=? AND tariff.weekday='mon'
    JOIN care_contract.contract_templates AS template ON template.school_id=offering.tenant_id AND template.care_offering_id=offering.id AND template.organization_id=?
    WHERE offering.tenant_id=? AND offering.traeger_source_id=? ORDER BY tariff.valid_from DESC LIMIT 1`, office.organization, office.organization, school, offering.ID).Scan(t.Context(), &cents, &partner))
			assert.Equal(t, int64(1600), cents, "a failed template cannot leave the higher tariff committed")
			assert.Equal(t, "Ursprüngliche Bedingungen", partner)
		}
	}
	checkTerms()
	rec = traegerJSON(t, router, office.token, http.MethodPut, fmt.Sprintf("/traeger/catalog/offerings/%d", offering.ID), draft)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	preview = catalogPreview(t, router, office.token, path)
	result := catalogRollout(t, router, office.token, path, preview.Fingerprint)
	for _, school := range result.Schools {
		require.Equal(t, "applied", school.Outcome, school.ErrorCode)
	}
	checkTerms()
	// Outcome persistence shares the financial commit, so a failed outcome
	// cannot leave the next release comparing against an obsolete baseline.
	_, err = db.ExecContext(t.Context(), `CREATE FUNCTION enrollment.refuse_catalog_outcome_test() RETURNS trigger LANGUAGE plpgsql AS $$
  BEGIN IF NEW.applied_version IS DISTINCT FROM OLD.applied_version THEN RAISE EXCEPTION 'outcome write refused'; END IF; RETURN NEW; END $$;
  CREATE TRIGGER refuse_catalog_outcome_test BEFORE UPDATE ON enrollment.traeger_care_offering_schools
  FOR EACH ROW EXECUTE FUNCTION enrollment.refuse_catalog_outcome_test()`)
	require.NoError(t, err)
	dearer["contract_template"].(map[string]any)["contract_partner"] = "Neue Bedingungen"
	rec = traegerJSON(t, router, office.token, http.MethodPut, fmt.Sprintf("/traeger/catalog/offerings/%d", offering.ID), dearer)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	preview = catalogPreview(t, router, office.token, path)
	rec = traegerJSON(t, router, office.token, http.MethodPost, path+"/rollout", map[string]any{"fingerprint": preview.Fingerprint})
	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	checkTerms()
}

func TestTraegerCatalogRejectsInvalidOfferingFields(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	portal := traegerPortalWithDays(t, db, testpkg.TenantRuntime(t, db), owners.HolidayTestSchoolCalendar{})
	office := newTraegerOffice(t, db, portal, "Träger Validierung", identityaccess.TraegerRoleAdministration)
	testpkg.OwnTraegerCatalog(t, db, office.organization)
	router := traegerCatalogRouter(t, db)
	rec := traegerJSON(t, router, office.token, http.MethodPost, "/traeger/catalog/phases", map[string]any{
		"name": "Validierung", "kind": "school_year", "service_start_date": "2027-08-01", "service_end_date": "2028-07-31", "all_schools": true,
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	phase := decodeSchoolPortalData[catalogPhaseBody](t, rec.Body.Bytes())
	path := fmt.Sprintf("/traeger/catalog/phases/%d/offerings", phase.ID)
	rec = traegerJSON(t, router, office.token, http.MethodPost, path, catalogOffering(20, 1600))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	offering := decodeSchoolPortalData[careplan.TraegerOffering](t, rec.Body.Bytes())
	for _, invalid := range []struct {
		field string
		value any
		code  string
	}{
		{"days_of_week_mode", "unknown", "enrollment.traeger_catalog_invalid"},
		{"available_days", []string{"unknown"}, "enrollment.traeger_catalog_invalid"},
		{"pickup_times", map[string]string{"unknown": "08:30"}, "enrollment.traeger_catalog_invalid"},
		{"available_days", []string{}, "enrollment.care_offering_days_required"},
		{"name", "", careplan.CodeCareOfferingNameRequired},
		{"capacity", -1, careplan.CodeCareOfferingCapacityInvalid},
	} {
		for _, method := range []string{http.MethodPost, http.MethodPut} {
			t.Run(invalid.field+"/"+method, func(t *testing.T) {
				draft := catalogOffering(20, 1600)
				draft["fields"].(map[string]any)[invalid.field] = invalid.value
				target := path
				if method == http.MethodPut {
					target = fmt.Sprintf("/traeger/catalog/offerings/%d", offering.ID)
				}
				rec := traegerJSON(t, router, office.token, method, target, draft)
				assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
				assert.Contains(t, rec.Body.String(), fmt.Sprintf(`"code":%q`, invalid.code))
			})
		}
	}
}

func (o catalogPhaseOfferings) HasTraegerCopiesForPhase(ctx context.Context, id int64) (bool, error) {
	rows, err := o.carePlan.ListCareOfferings(ctx, careplan.CareOfferingFilter{PhaseIDs: []int64{id}})
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		if row.Traeger != nil {
			return true, nil
		}
	}
	return false, nil
}
