package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// officeChangeReview stands in for Care Plan's offering-change review of the
// contract's school: the child's one open request and every decision the
// route hands it.
type officeChangeReview struct {
	careplan.OfferingChangeCapability
	pending careplan.OfferingChangeRequest
	decided []careplan.OfferingChangeDecisionInput
}

func (r *officeChangeReview) GetForStudent(_ context.Context, studentID int64) (*careplan.OfferingChangeView, error) {
	if studentID != r.pending.StudentID {
		return nil, nil
	}
	return &careplan.OfferingChangeView{Request: &r.pending, Diff: []careplan.OfferingChangeDiffEntry{{
		OfferingID: 11, Label: "Offene Ganztagsschule", OldState: "booked", OldDays: []string{"mon"},
		NewState: "booked", NewDays: []string{"mon", "tue"},
	}}}, nil
}

func (r *officeChangeReview) Decide(_ context.Context, input careplan.OfferingChangeDecisionInput) (careplan.RosterImpact, error) {
	r.decided = append(r.decided, input)
	impact := careplan.RosterImpact{}
	if input.EffectiveFrom != nil {
		impact.EffectiveFrom = *input.EffectiveFrom
	}
	return impact, nil
}

// careContractChangeRouter mounts /traeger/contract-changes as the serving
// root does, behind the tenant runtime and the session verifier.
func careContractChangeRouter(t *testing.T, db *bun.DB, review careplan.OfferingChangeCapability) http.Handler {
	t.Helper()
	unit := testpkg.TenantRuntime(t, db)
	database := testpkg.NewDatabase(t, db)
	organizations, err := owners.NewOrganizationTenancy(database)
	require.NoError(t, err)
	audit, err := auditCompose.NewTenantPersistence(db)
	require.NoError(t, err)
	schoolSettings, err := owners.NewSettingsTestModule(database, unit)
	require.NoError(t, err)
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(tenant.WithUnitOfWork(r.Context(), unit)))
		})
	})
	mountCareContractChangeRoutes(router, organizations, review, schoolSettings.Settings, audit.DataAccessLog)
	return testpkg.SessionVerifier(router)
}

type contractChangesBody struct {
	Decidable bool `json:"decidable"`
	Pending   []struct {
		ID      int64  `json:"id"`
		Version string `json:"version"`
		Lines   []struct {
			ModuleName string   `json:"module_name"`
			State      string   `json:"state"`
			AfterDays  []string `json:"after_days"`
		} `json:"lines"`
	} `json:"pending"`
}

// The Träger-Büro reads and decides the parents' change of a contract's
// child with the right "Verträge verwalten" and only for its own Träger
// (#5979). Without the right or for another Träger the route answers
// without handing anything to the school's review, and the read and the
// decision land in the school's access log.
func TestTraegerOfficeDecidesTheParentsChangeOnlyWithTheRightAndForItsOwnTraeger(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Vertrag Änderungswunsch")
	ctx := context.Background()
	student := testpkg.UniqueTestTenantID(t)
	var contract int64
	require.NoError(t, f.db.NewRaw(`INSERT INTO care_contract.contracts (organization_id, school_id, request_child_id, student_id,
			status, sent_at, concluded_at, conclusion_method) VALUES (?, ?, ?, ?, 'aktiv', NOW(), NOW(), 'mit_anmeldung') RETURNING id`,
		f.office.organization, f.school, f.child, student).Scan(ctx, &contract))
	review := &officeChangeReview{pending: careplan.OfferingChangeRequest{
		ID: testpkg.UniqueTestTenantID(t), StudentID: student, RequestChildID: f.child, EffectiveFrom: "2100-03-01", Status: careplan.OfferingChangePending,
		CreatedAt: time.Date(2099, 12, 1, 9, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2099, 12, 1, 9, 0, 0, 0, time.UTC),
	}}
	router := careContractChangeRouter(t, testpkg.SetupTestDB(t), review)
	listPath := fmt.Sprintf("/traeger/contract-changes/%d", contract)
	decidePath := fmt.Sprintf("%s/requests/%d/decide", listPath, review.pending.ID)
	approve := map[string]any{"approve": true, "effective_from": "2100-03-01",
		"expected_version": careplan.ParentRequestVersion(review.pending.UpdatedAt)}

	rec := traegerJSON(t, router, f.office.token, http.MethodGet, listPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	read := decodeSchoolPortalData[contractChangesBody](t, rec.Body.Bytes())
	assert.True(t, read.Decidable)
	require.Len(t, read.Pending, 1)
	assert.Equal(t, review.pending.ID, read.Pending[0].ID)
	require.Len(t, read.Pending[0].Lines, 1)
	assert.Equal(t, "changed", read.Pending[0].Lines[0].State)
	assert.Equal(t, []string{"mon", "tue"}, read.Pending[0].Lines[0].AfterDays)

	rec = traegerJSON(t, router, f.office.token, http.MethodPost, decidePath, approve)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Len(t, review.decided, 1)
	decided := review.decided[0]
	assert.Equal(t, student, decided.StudentID, "Care Plan checks the request against the contract's child")
	assert.Equal(t, f.child, decided.RequestChildID, "Care Plan checks the contract enrollment under its request lock")
	assert.Equal(t, review.pending.ID, decided.RequestID)
	assert.True(t, decided.Approve)
	require.NotNil(t, decided.EffectiveFrom)
	assert.Equal(t, calendar.NewDate(2100, 3, 1), *decided.EffectiveFrom)
	assert.Equal(t, approve["expected_version"], decided.ExpectedVersion)
	assert.Equal(t, "traeger", decided.ActorRole)
	assert.Positive(t, decided.ReviewedBy)

	billing := traegerColleague(t, f.reviewFixture, identityaccess.TraegerRoleBilling)
	rec = traegerJSON(t, router, billing, http.MethodPost, decidePath, approve)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	rec = traegerJSON(t, router, billing, http.MethodGet, listPath, nil)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	other := newTraegerOffice(t, f.db, f.traeger, "Träger Vertrag Änderungswunsch Fremd", identityaccess.TraegerRoleAdministration)
	testpkg.OwnTraegerCareContracts(t, f.db, other.organization)
	rec = traegerJSON(t, router, other.token, http.MethodPost, decidePath, approve)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	rec = traegerJSON(t, router, other.token, http.MethodGet, listPath, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Len(t, review.decided, 1, "no refused call reaches the school's review")

	var views []string
	require.NoError(t, f.db.NewRaw(`SELECT metadata->>'view' FROM audit.data_access_log
		WHERE tenant_id = ? AND resource_type = 'care_contract_changes' ORDER BY id`, f.school).Scan(ctx, &views))
	assert.Equal(t, []string{"contract_changes", "contract_change_approve"}, views, "the read and the decision are in the school's access log")
}
