// The renewal's acceptance on its way into the contract (#6456): a rollover
// copies no evidence, the parents accept the terms of the new phase on the
// status page, and the Träger-Büro reads that acceptance, or its absence,
// when the school accepted the renewed child.
package api

import (
	"context"
	"log/slog"
	"maps"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestRenewalAcceptanceReachesTheContractIntake(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		mode          string
		confirm       bool
		changeTerms   bool
		wantIssue     string
		wantConfirmed int
	}{
		{name: "opt-in with ticks", mode: enrollment.PhaseRolloverModeOptIn, confirm: true, wantConfirmed: 1},
		{name: "opt-out without action", mode: enrollment.PhaseRolloverModeOptOut, wantIssue: carecontract.SkipAcceptanceMissing},
		{name: "terms changed after the acceptance", mode: enrollment.PhaseRolloverModeOptOut, confirm: true, changeTerms: true,
			wantIssue: carecontract.SkipAcceptanceChanged},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newCareContractFixture(t, "Träger Wiederanmeldung "+tc.name)
			require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, f.office.token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate).Code)
			intake, rollovers, schemaID := newRenewalContractFlow(t, f)

			request, child := rollRenewal(t, f, intake, rollovers, schemaID, tc.mode)
			assert.Empty(t, request.LegalBlocksSnapshot, "the rollover copies no evidence")
			traegerTemplateFor(t, f, child.offering, careContractTemplate)

			if tc.confirm {
				confirmed, err := confirmRenewalTerms(t, f, intake, schemaID, request.StatusToken)
				require.NoError(t, err)
				assert.Equal(t, tc.wantConfirmed, confirmed)
			}
			if tc.changeTerms {
				changed := maps.Clone(careContractTemplate)
				changed["notice_months"] = 3
				traegerTemplateFor(t, f, child.offering, changed)
			}
			// The school accepts the renewed child afterwards.
			_, err := f.db.ExecContext(context.Background(), `UPDATE enrollment.request_children
  SET status = 'approved', reviewed_at = clock_timestamp() + interval '1 minute' WHERE id = ?`, child.id)
			require.NoError(t, err)

			setTraegerConclusionMode(t, f, carecontract.ModeOnAcceptance)
			assert.Equal(t, tc.wantIssue, renewalCandidateIssue(t, f, child.id))
		})
	}
}

// renewalIntake is the part of Enrollment's intake the parents' status page
// uses.
type renewalIntake interface {
	StatusByToken(ctx context.Context, token string) (*enrollment.RequestStatus, error)
	StatusPage(ctx context.Context, status *enrollment.RequestStatus) (enrollment.StatusPage, error)
	ConfirmRenewal(ctx context.Context, token string, confirmation enrollment.RenewalConfirmation) (int, error)
}

type renewedChild struct {
	id       int64
	offering int64
}

// newRenewalContractFlow composes Enrollment's intake and rollover over the
// fixture school, as the serving root binds them, with a form for the new
// phase that asks for the contract conditions.
func newRenewalContractFlow(t *testing.T, f careContractFixture) (renewalIntake, enrollment.Rollovers, int64) {
	t.Helper()
	database := testpkg.NewDatabase(t, f.db)
	unit := testpkg.TenantRuntime(t, f.db)
	repos, err := NewEnrollmentTestRepositories(database, owners.NewTestAuditStore(database))
	require.NoError(t, err)
	settings, err := owners.NewSettingsTestModule(database, unit)
	require.NoError(t, err)
	catalog, err := NewCareOfferingCatalog(database, unit)
	require.NoError(t, err)
	ctx := context.Background()
	// The school booked the Online-Anmeldung; otherwise its status links stay
	// closed.
	_, err = f.db.ExecContext(ctx, `INSERT INTO config.setting_values (tenant_id, setting_key, value)
		VALUES (?, 'module.online_anmeldung', 'true'::jsonb) ON CONFLICT (tenant_id, setting_key) DO UPDATE SET value = EXCLUDED.value`, f.school)
	require.NoError(t, err)
	// The rollover clones the offering into the new phase and checks it: its
	// end times name only the days it offers. A child without a grade would
	// wait for the office's review instead of its renewal.
	_, err = f.db.ExecContext(ctx, `UPDATE enrollment.care_offerings
		SET pickup_times = '{"mon":"14:00","tue":"16:30","wed":"16:30"}'::jsonb WHERE id = ?`, f.offering)
	require.NoError(t, err)
	_, err = f.db.ExecContext(ctx, `UPDATE enrollment.request_children SET target_grade_level = 1 WHERE id = ?`, f.child)
	require.NoError(t, err)

	// The new phase's form asks for the conditions of the contract, in the
	// wording the Träger saved.
	text := careContractAcceptanceText(t, f)
	_, author := testpkg.CreateTestPersonWithAccount(t, f.db, "Renate", "Vertrag")
	schemas := enrollmentCompose.NewFormSchemas(repos.Enrollment(), nil, slog.Default())
	var schema *enrollment.FormSchema
	require.NoError(t, tenant.NewTransactionRunner().RunInTx(testpkg.TenantContext(f.school), func(txCtx context.Context) error {
		var createErr error
		schema, createErr = schemas.CreateSchemaWithLegal(txCtx, "Wiederanmeldung "+t.Name(), nil, author.ID, enrollment.CoreRequirements{},
			[]enrollment.FormLegalBlock{{Key: "terms", Kind: enrollment.LegalBlockKindTerms, Title: "Betreuungsvertrag",
				Label: "Ich stimme den Vertragsbedingungen zu.", Text: text, Required: true, Enabled: true,
				Source: enrollment.LegalBlockSourceCustom}})
		return createErr
	}))

	intake := owners.NewEnrollmentIntake(owners.EnrollmentIntakeSources{
		Requests: repos.Enrollment(), ParentChanges: repos.Enrollment(), Children: repos.Enrollment(), Guardians: repos.Enrollment(),
		Catalog: repos.Enrollment(), Settings: settings.Settings, ParentsURL: "http://parents.localhost:3000", Logger: slog.Default(),
	})
	rollovers := owners.NewEnrollmentRollovers(owners.EnrollmentRolloverSources{
		Bookings: NewEnrollmentCareBookingCommands(carePlanCompose.NewOfferingBookings()),
		Phases:   repos.Enrollment(), Requests: repos.Enrollment(), Children: repos.Enrollment(),
		Catalog: catalog.Catalog, Settings: settings.Settings, ParentsURL: "http://parents.localhost:3000", Logger: slog.Default(),
	})
	return intake, rollovers, schema.ID
}

// rollRenewal renews the fixture's accepted child into the next school year
// with the contract form and returns the new request with its child.
func rollRenewal(t *testing.T, f careContractFixture, intake renewalIntake, rollovers enrollment.Rollovers, schemaID int64, mode string) (*enrollment.Request, renewedChild) {
	t.Helper()
	ctx := testpkg.TenantContext(f.school)
	var result *enrollment.RolloverResult
	require.NoError(t, tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		result, err = rollovers.CreatePhaseFromSource(txCtx, enrollment.CreatePhaseFromSourceRequest{
			SourcePhaseID: f.phase, Name: "Schuljahr 2028/29", Kind: enrollment.PhaseKindSchoolYear,
			ServiceStartDate: calendar.NewDate(2028, 8, 1), ServiceEndDate: calendar.NewDate(2029, 7, 31),
			FormSchemaID: &schemaID, RolloverMode: mode, RolloverDeadline: time.Date(2028, 7, 15, 0, 0, 0, 0, time.UTC),
			RolloverBumpsGrade: true,
		})
		return err
	}))
	require.Equal(t, 1, result.RolledCount, "review queue: %v", result.ReviewByReason)

	var row struct {
		ChildID   int64 `bun:"child_id"`
		RequestID int64 `bun:"request_id"`
		Offering  int64 `bun:"offering_id"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT c.id AS child_id, c.request_id, b.care_offering_id AS offering_id
		FROM enrollment.request_children c
		JOIN enrollment.care_offering_bookings b ON b.request_child_id = c.id
		JOIN enrollment.care_offerings o ON o.id = b.care_offering_id AND o.counts_as_care
		WHERE c.tenant_id = ? AND c.rollover_source_child_id = ?`, f.school, f.child).Scan(context.Background(), &row))
	var request *enrollment.Request
	require.NoError(t, tenant.WithinAdmin(ctx, func(adminCtx context.Context) error {
		var token string
		if err := f.db.NewRaw(`SELECT status_token FROM enrollment.requests WHERE id = ?`, row.RequestID).Scan(adminCtx, &token); err != nil {
			return err
		}
		status, err := intake.StatusByToken(adminCtx, token)
		if err != nil {
			return err
		}
		request = status.Request
		return nil
	}))
	return request, renewedChild{id: row.ChildID, offering: row.Offering}
}

// confirmRenewalTerms ticks every block the status page shows, as the
// parents do.
func confirmRenewalTerms(t *testing.T, f careContractFixture, intake renewalIntake, schemaID int64, token string) (int, error) {
	t.Helper()
	ctx := testpkg.TenantContext(f.school)
	var terms *enrollment.RenewalTerms
	require.NoError(t, tenant.WithinAdmin(ctx, func(adminCtx context.Context) error {
		status, err := intake.StatusByToken(adminCtx, token)
		if err != nil {
			return err
		}
		page, err := intake.StatusPage(adminCtx, status)
		terms = page.RenewalTerms
		return err
	}))
	require.NotNil(t, terms)
	require.Equal(t, schemaID, terms.FormSchemaID, "the page shows the form of the new phase")
	flags := map[string]any{}
	for _, block := range terms.Blocks {
		flags[block.Key] = true
	}
	var confirmed int
	err := tenant.WithinAdmin(ctx, func(adminCtx context.Context) error {
		var confirmErr error
		confirmed, confirmErr = intake.ConfirmRenewal(adminCtx, token, enrollment.RenewalConfirmation{
			ConsentFlags: enrollment.CollectFormValues(maps.All(flags)), DisplayedFormSchemaID: terms.FormSchemaID,
		})
		return confirmErr
	})
	return confirmed, err
}

// careContractAcceptanceText is the conditions text the Träger saved for the
// fixture's offering.
func careContractAcceptanceText(t *testing.T, f careContractFixture) string {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, "/traeger/contract-templates", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	for _, offering := range decodeSchoolPortalData[[]carecontract.OfferingTemplate](t, rec.Body.Bytes()) {
		if offering.SchoolID == f.school && offering.OfferingID == f.offering && offering.AcceptanceText != "" {
			return offering.AcceptanceText
		}
	}
	t.Fatal("the offering's conditions are missing")
	return ""
}

// traegerTemplateFor saves the conditions of one offering.
func traegerTemplateFor(t *testing.T, f careContractFixture, offeringID int64, template map[string]any) {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, templatePath(f.school, offeringID), template)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// renewalCandidateIssue reads the proof state the candidates list gives for
// one child.
func renewalCandidateIssue(t *testing.T, f careContractFixture, childID int64) string {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, "/traeger/contracts/candidates", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	for _, candidate := range decodeSchoolPortalData[[]carecontract.Candidate](t, rec.Body.Bytes()) {
		if candidate.RequestChildID == childID {
			require.Empty(t, candidate.TemplateIssue, "the renewed module has its conditions")
			assert.True(t, candidate.Renewal, "the office sees that the child was renewed")
			return candidate.AcceptanceIssue
		}
	}
	t.Fatalf("child %d is no candidate", childID)
	return ""
}
