package parent_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
	parent "github.com/moto-nrw/project-phoenix/workflows/parentportal/http"
)

// masterDataRequestSettings routes master-data changes through a request:
// direct editing is off, every other parent feature is on.
type masterDataRequestSettings struct{ parentService.SettingsService }

func (masterDataRequestSettings) ResolveBoolForTenant(_ context.Context, _ int64, key string) (bool, error) {
	return key != parentService.KeyParentMasterDataEditEnabled, nil
}

func (masterDataRequestSettings) ResolveStringForTenant(_ context.Context, _ int64, _ string) (string, error) {
	return "", nil
}

// newRequestLookupRouter mounts the parent routes with every request lookup
// the history, sharing and edit routes read, bound as the production root
// binds them.
func newRequestLookupRouter(t *testing.T, db *bun.DB) (http.Handler, careplan.Capability) {
	t.Helper()
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	people := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	carePlan := repos.CarePlan()
	svc := parentportalcompose.New(parentportalcompose.Dependencies{
		GuardianChanges: owners.ParentGuardianChanges(),
		DeparturePolicy: owners.ParentDeparturePolicy(),
		GuardianPolicy:  owners.NewParentGuardianPolicy(),
		Fingerprints:    owners.SecurityFingerprints{},
		Declarations:    owners.ParentDeclarationPolicy(),
		Locales:         owners.ParentPortalLocales(),
		Runtime:         owners.ParentPortalRuntime{},
		CarePlan:        owners.NewParentCarePlanCommands(carePlan, nil),
		People:          owners.MustNewPeopleDirectory(testpkg.MustDatabase(db)),
		CareProfiles:    careProfiles(t, db),
		ChildRepo:       repos.ParentChildReads(),
		StudentRepo:     owners.NewParentStudentReads(people.Student),
		PersonRepo:      owners.NewParentPersonReads(people.Person),
		Settings:        masterDataRequestSettings{},
		StudentUpdates:  owners.BindParentStudentUpdates(testpkg.NewRecordingBroadcaster()),
		Logger:          slog.Default(),

		GuardianProfileRepo: owners.NewParentGuardianProfileReads(people.GuardianProfile),
		GuardianPhoneRepo:   owners.NewParentGuardianPhoneReads(people.GuardianPhoneNumber),
		StudentGuardianRepo: owners.NewParentStudentGuardianReads(people.StudentGuardian),

		ChangeRequestRepo:         owners.NewParentDataRequestReads(carePlan),
		DataRequests:              owners.NewParentDataRequestCommands(carePlan),
		CareRequestRepo:           owners.NewParentCareRequestReads(carePlan),
		ExcusedRequestRepo:        repos.ExcusedAbsenceRequest,
		OfferingChangeRequestRepo: api.NewParentOfferingChangeTestReads(carePlan),
		FamilyProtectionEvents:    owners.BindParentFamilyProtectionReads(people.FamilyProtection),
		ParentRequestShares:       owners.BindParentRequestShareRecords(people.ParentRequestShare),
		ParentRequestEvents:       owners.NewParentRequestEvents(people.ParentRequestEvent),

		CarePlanPolicy: owners.NewParentCarePlanPolicy(),
	})
	rs := parent.NewResource(parent.ResourceConfig{Parent: svc})
	return testpkg.TenantRuntimeMiddleware(t, db)(rs.Router(api.ParentSessionChain)), carePlan
}

// TestRequestHistoryAndSharingAnswerAMissingRequestWithNotFound pins #4599
// case 1: a request id that no longer exists answers the same 404 a foreign
// request does, for every request type the history and sharing routes take.
func TestRequestHistoryAndSharingAnswerAMissingRequestWithNotFound(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	router, _ := newRequestLookupRouter(t, db)
	token := parentToken(t, chain.AccountID)

	for requestType, table := range map[string]string{
		parentService.RequestShareMasterData:   "users.student_data_change_requests",
		parentService.RequestShareCareSchedule: "schedule.care_schedule_change_requests",
		"pickup_change":                        "schedule.care_schedule_change_requests",
		"offering":                             "enrollment.offering_change_requests",
		parentService.RequestShareExcused:      "active.excused_absence_requests",
	} {
		missingID := reserveMissingRequestID(t, db, table)
		t.Run(requestType, func(t *testing.T) {
			history := doRequest(t, router, http.MethodGet,
				fmt.Sprintf("/me/children/%d/requests/%s/%d/events", chain.StudentID, requestType, missingID), token, nil)
			assert.Equal(t, http.StatusNotFound, history.Code, history.Body.String())
			assert.Contains(t, history.Body.String(), `"code":"care.request_not_found"`)

			sharingPath := fmt.Sprintf("/me/children/%d/request-sharing/%s/%d", chain.StudentID, requestType, missingID)
			read := doRequest(t, router, http.MethodGet, sharingPath, token, nil)
			assert.Equal(t, http.StatusNotFound, read.Code, read.Body.String())
			assert.Contains(t, read.Body.String(), `"code":"care.request_not_found"`)

			write := doRequest(t, router, http.MethodPut, sharingPath, token,
				map[string]any{"recipient_guardian_profile_ids": []string{}})
			assert.Equal(t, http.StatusNotFound, write.Code, write.Body.String())
			assert.Contains(t, write.Body.String(), `"code":"care.request_not_found"`)
		})
	}
}

// TestEditMasterDataRequestEndpointRefusesForeignMissingAndDecided pins #4599
// case 2: a foreign or missing request is not found, a decided one is no
// longer open.
func TestEditMasterDataRequestEndpointRefusesForeignMissingAndDecided(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	mine := testpkg.CreateTestParentGuardianChain(t, db)
	theirs := testpkg.CreateTestParentGuardianChain(t, db)
	router, carePlan := newRequestLookupRouter(t, db)
	token := parentToken(t, mine.AccountID)

	created := doRequest(t, router, http.MethodPost,
		fmt.Sprintf("/me/children/%d/master-data/requests", theirs.StudentID), parentToken(t, theirs.AccountID),
		map[string]any{"changes": []map[string]any{{"target": "person", "field_key": "first_name", "value": "Maximilian"}}})
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	var createdEnv envelope
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &createdEnv))
	var rows []struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(createdEnv.Data, &rows))
	require.Len(t, rows, 1)
	foreignID := rows[0].ID

	foreign := doRequest(t, router, http.MethodPut,
		fmt.Sprintf("/me/children/%d/master-data/requests/%s", mine.StudentID, foreignID), token,
		map[string]any{"new_value": "Maxi"})
	assert.Equal(t, http.StatusNotFound, foreign.Code, foreign.Body.String())
	assert.Contains(t, foreign.Body.String(), `"code":"care.request_not_found"`)

	missing := doRequest(t, router, http.MethodPut,
		fmt.Sprintf("/me/children/%d/master-data/requests/%d", mine.StudentID,
			reserveMissingRequestID(t, db, "users.student_data_change_requests")), token,
		map[string]any{"new_value": "Maxi"})
	assert.Equal(t, http.StatusNotFound, missing.Code, missing.Body.String())
	assert.Contains(t, missing.Body.String(), `"code":"care.request_not_found"`)

	var requestID int64
	_, err := fmt.Sscan(foreignID, &requestID)
	require.NoError(t, err)
	require.NoError(t, testpkg.WithTenantTx(t, context.Background(), db, theirs.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		return carePlan.DecideStudentDataRequest(txCtx, careplan.StudentDataRequestDecision{
			ID: requestID, Status: "rejected", ReviewedBy: theirs.AccountID,
		})
	}))
	decided := doRequest(t, router, http.MethodPut,
		fmt.Sprintf("/me/children/%d/master-data/requests/%s", theirs.StudentID, foreignID), parentToken(t, theirs.AccountID),
		map[string]any{"new_value": "Maxi"})
	assert.Equal(t, http.StatusConflict, decided.Code, decided.Body.String())
	assert.Contains(t, decided.Body.String(), `"code":"care.request_not_open"`)
}

// reserveMissingRequestID takes an id from the table's sequence that no row
// holds, so the missing request is real rather than a guessed literal.
func reserveMissingRequestID(t *testing.T, db *bun.DB, table string) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.NewSelect().
		ColumnExpr("nextval(pg_get_serial_sequence(?, 'id'))", table).
		Scan(testpkg.Ctx(t), &id))
	return id
}
