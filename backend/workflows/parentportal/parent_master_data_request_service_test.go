package parentportal_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/communication/communicationtest"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
)

func buildRequestService(t *testing.T) (*parentService.Portal, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	reposPeople := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	svc := parentportalcompose.New(parentportalcompose.Dependencies{
		GuardianChanges:     owners.ParentGuardianChanges(),
		DeparturePolicy:     owners.ParentDeparturePolicy(),
		GuardianPolicy:      owners.NewParentGuardianPolicy(),
		Fingerprints:        owners.SecurityFingerprints{},
		Declarations:        owners.ParentDeclarationPolicy(),
		Locales:             owners.ParentPortalLocales(),
		Runtime:             owners.ParentPortalRuntime{},
		CarePlan:            owners.NewParentCarePlanCommands(repos.CarePlan(), nil),
		People:              owners.MustNewPeopleDirectory(testpkg.MustDatabase(db)),
		CareProfiles:        careProfiles(t, db),
		ChildRepo:           repos.ParentChildReads(),
		StudentRepo:         owners.NewParentStudentReads(reposPeople.Student),
		GuardianProfileRepo: owners.NewParentGuardianProfileReads(reposPeople.GuardianProfile),
		PersonRepo:          owners.NewParentPersonReads(reposPeople.Person),
		GuardianPhoneRepo:   owners.NewParentGuardianPhoneReads(reposPeople.GuardianPhoneNumber),
		ChangeRequestRepo:   owners.NewParentDataRequestReads(repos.CarePlan()),
		Settings:            masterDataSettings(false, true, false),
		StudentUpdates:      owners.BindParentStudentUpdates(testpkg.NewRecordingBroadcaster()),
		Logger:              slog.Default(),

		DataRequests: owners.NewParentDataRequestCommands(repos.CarePlan()),

		CarePlanPolicy: owners.NewParentCarePlanPolicy(),
	})
	return svc, db
}

func TestSubmitMasterDataChangeRequest_CreatesPending(t *testing.T) {
	t.Parallel()

	svc, db := buildRequestService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	rows, err := svc.SubmitMasterDataChangeRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, []parentService.MasterDataFieldChange{
		{Target: parentService.DataChangeTargetPerson, FieldKey: "first_name", Value: json.RawMessage(`"Maximilian"`)},
	}, nil)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, parentService.DataChangeStatusPending, rows[0].Status)

	// The live person record is NOT changed by a Track B submission.
	person, err := testpkg.TenantQuery1(t, testpkg.ContextForTenant(testpkg.WithPackageTenantRuntime(context.Background()), chain.TenantID), db, api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Person.FindByID, any(chain.PersonID))
	require.NoError(t, err)
	assert.Equal(t, "Felix", person.FirstName)
}

func TestSubmitMasterDataChangeRequest_CareEndedChildIsRejected(t *testing.T) {
	t.Parallel()

	svc, db := buildRequestService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	endCareFor(t, db, chain.StudentID)

	_, err := svc.SubmitMasterDataChangeRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, []parentService.MasterDataFieldChange{
		{Target: parentService.DataChangeTargetPerson, FieldKey: "first_name", Value: json.RawMessage(`"Maximilian"`)},
	}, nil)
	require.ErrorIs(t, err, parentService.ErrChildCareEnded)
}

func TestSubmitMasterDataChangeRequest_SchoolClassRemainsPending(t *testing.T) {
	t.Parallel()

	svc, db := buildRequestService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	rows, err := svc.SubmitMasterDataChangeRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, []parentService.MasterDataFieldChange{
		{Target: parentService.DataChangeTargetStudent, FieldKey: "school_class", Value: json.RawMessage(`"2b"`)},
	}, nil)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, parentService.DataChangeTargetStudent, rows[0].Target)
	assert.Equal(t, "school_class", rows[0].FieldKey)
	assert.JSONEq(t, `"1a"`, string(rows[0].OldValue))
	assert.JSONEq(t, `"2b"`, string(rows[0].NewValue))

	student, err := testpkg.TenantQuery1(t, testpkg.ContextForTenant(testpkg.WithPackageTenantRuntime(context.Background()), chain.TenantID), db, api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Student.FindByID, any(chain.StudentID))
	require.NoError(t, err)
	assert.Equal(t, "1a", student.SchoolClass)
}

func TestSubmitMasterDataChangeRequest_DepartureAndListRequests(t *testing.T) {
	t.Parallel()

	svc, db := buildRequestService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	rows, err := svc.SubmitMasterDataChangeRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, []parentService.MasterDataFieldChange{
		{
			Target:   "departure",
			FieldKey: "allowed_departure_modes",
			Value:    json.RawMessage(`{"mon":["pickup"],"wed":["bus"]}`),
		},
	}, nil)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "departure", rows[0].Target)
	assert.Equal(t, "allowed_departure_modes", rows[0].FieldKey)
	assert.JSONEq(t, `{"mon":["pickup"],"wed":["bus"]}`, string(rows[0].NewValue))

	listed, err := svc.ListMyMasterDataRequests(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, rows[0].ID, listed[0].ID)
}

func TestListMyMasterDataRequests_HidesGuardianContactAuditRows(t *testing.T) {
	t.Parallel()

	svc, db := buildRequestService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))

	otherAccount := testpkg.CreateTestAccount(t, db, "other-parent")

	err := testpkg.WithTenantTx(t, testpkg.WithPackageTenantRuntime(context.Background()), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		privateAudit := &parentService.StudentDataChangeRequest{
			StudentID:   chain.StudentID,
			SubmittedBy: otherAccount.ID,
			Target:      "guardian_profile",
			FieldKey:    "email",
			OldValue:    json.RawMessage(`"other-old@example.test"`),
			NewValue:    json.RawMessage(`"other-new@example.test"`),
			Status:      "auto_applied",
		}
		privateAudit.TenantID = chain.TenantID
		if createErr := api.CreateParentDataRequestFixture(repos.CarePlan(), txCtx, privateAudit); createErr != nil {
			return createErr
		}

		visibleRequest := &parentService.StudentDataChangeRequest{
			StudentID:   chain.StudentID,
			SubmittedBy: otherAccount.ID,
			Target:      parentService.DataChangeTargetPerson,
			FieldKey:    "first_name",
			OldValue:    json.RawMessage(`"Felix"`),
			NewValue:    json.RawMessage(`"Max"`),
			Status:      parentService.DataChangeStatusPending,
		}
		visibleRequest.TenantID = chain.TenantID
		return api.CreateParentDataRequestFixture(repos.CarePlan(), txCtx, visibleRequest)
	})
	require.NoError(t, err)

	rows, err := svc.ListMyMasterDataRequests(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	assert.Empty(t, rows, "another guardian's requested values stay private")
}

func TestSubmitMasterDataChangeRequest_DuplicatePending(t *testing.T) {
	t.Parallel()

	svc, db := buildRequestService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	change := []parentService.MasterDataFieldChange{
		{Target: parentService.DataChangeTargetPerson, FieldKey: "first_name", Value: json.RawMessage(`"Maximilian"`)},
	}
	_, err := svc.SubmitMasterDataChangeRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, change, nil)
	require.NoError(t, err)

	_, err = svc.SubmitMasterDataChangeRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, change, nil)
	assert.ErrorIs(t, err, parentService.ErrMasterDataDuplicatePending)
}

func TestSubmitMasterDataChangeRequest_ConcurrentDuplicatePending(t *testing.T) {
	t.Parallel()

	svc, db := buildRequestService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	change := []parentService.MasterDataFieldChange{
		{Target: parentService.DataChangeTargetPerson, FieldKey: "first_name", Value: json.RawMessage(`"Maximilian"`)},
	}

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = svc.SubmitMasterDataChangeRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, change, nil)
		}(i)
	}
	wg.Wait()

	var successCount, duplicateCount int
	for _, err := range errs {
		switch {
		case err == nil:
			successCount++
		case errors.Is(err, parentService.ErrMasterDataDuplicatePending):
			duplicateCount++
		default:
			t.Fatalf("unexpected concurrent submit error: %v", err)
		}
	}
	assert.Equal(t, 1, successCount)
	assert.Equal(t, 1, duplicateCount)
}

func TestSubmitMasterDataChangeRequest_NoChange(t *testing.T) {
	t.Parallel()

	svc, db := buildRequestService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	// Submitting the current value is a no-op -> ErrMasterDataNoChanges.
	_, err := svc.SubmitMasterDataChangeRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, []parentService.MasterDataFieldChange{
		{Target: parentService.DataChangeTargetPerson, FieldKey: "first_name", Value: json.RawMessage(`"Felix"`)},
	}, nil)
	assert.ErrorIs(t, err, parentService.ErrMasterDataNoChanges)
}

func TestSubmitMasterDataChangeRequest_InvalidInputs(t *testing.T) {
	t.Parallel()

	svc, db := buildRequestService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	_, err := svc.SubmitMasterDataChangeRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, nil, nil)
	assert.ErrorIs(t, err, parentService.ErrMasterDataNoChanges)

	_, err = svc.SubmitMasterDataChangeRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, []parentService.MasterDataFieldChange{
		{Target: parentService.DataChangeTargetStudent, FieldKey: "health_info", Value: json.RawMessage(`"x"`)},
	}, nil)
	assert.ErrorIs(t, err, parentService.ErrMasterDataFieldNotEditable)

	_, err = svc.SubmitMasterDataChangeRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, []parentService.MasterDataFieldChange{
		{Target: parentService.DataChangeTargetPerson, FieldKey: "birthday", Value: json.RawMessage(`"not-a-date"`)},
	}, nil)
	assert.ErrorIs(t, err, parentService.ErrMasterDataInvalidValue)

	_, err = svc.SubmitMasterDataChangeRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, []parentService.MasterDataFieldChange{
		{Target: "departure", FieldKey: "allowed_departure_modes", Value: json.RawMessage(`{`)},
	}, nil)
	assert.ErrorIs(t, err, parentService.ErrMasterDataInvalidValue)

	_, err = svc.SubmitMasterDataChangeRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, []parentService.MasterDataFieldChange{
		{Target: "departure", FieldKey: "allowed_departure_modes", Value: json.RawMessage(`{"sat":["pickup"]}`)},
	}, nil)
	assert.ErrorIs(t, err, parentService.ErrMasterDataInvalidValue)

	_, err = svc.SubmitMasterDataChangeRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, []parentService.MasterDataFieldChange{
		{Target: "departure", FieldKey: "allowed_departure_modes", Value: json.RawMessage(`{"mon":["spaceship"]}`)},
	}, nil)
	assert.ErrorIs(t, err, parentService.ErrMasterDataInvalidValue)

	_, err = svc.SubmitMasterDataChangeRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, []parentService.MasterDataFieldChange{
		{Target: "departure", FieldKey: "allowed_departure_modes", Value: json.RawMessage(`{"mon":["accompanied"]}`)},
	}, nil)
	assert.ErrorIs(t, err, parentService.ErrMasterDataInvalidValue)
}

// TestSubmitMasterDataChangeRequest_PerRowCreatedPills pins the P3 fix (#1803
// review follow-up): a multi-field submit emits ONE "Anfrage erstellt" pill PER
// created row, each referencing its own row — not a single pill tied to the
// first row. Per-row pills keep each field's created↔decision refs paired, so
// the thread timeline and the deep-link into the Änderungsanfragen queue can
// resolve the exact row for any field, not just the first.
func TestSubmitMasterDataChangeRequest_PerRowCreatedPills(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	reposMessages := api.NewParentMessagingSuiteRepositories(testpkg.NewDatabase(t, db))
	reposPeople := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	settings := parentSettingsStub{boolDefault: true}
	emitter := api.NewParentEventEmitter(
		testpkg.MustDatabase(db), testpkg.TenantRuntime(t, db), reposMessages.ParentMessageThread, communicationtest.NewParentMessageRepository(db),
		settings, nil, slog.Default(),
	)
	svc := parentportalcompose.New(parentportalcompose.Dependencies{
		GuardianChanges:     owners.ParentGuardianChanges(),
		DeparturePolicy:     owners.ParentDeparturePolicy(),
		GuardianPolicy:      owners.NewParentGuardianPolicy(),
		Fingerprints:        owners.SecurityFingerprints{},
		Declarations:        owners.ParentDeclarationPolicy(),
		Locales:             owners.ParentPortalLocales(),
		Runtime:             owners.ParentPortalRuntime{},
		CarePlan:            owners.NewParentCarePlanCommands(repos.CarePlan(), nil),
		People:              owners.MustNewPeopleDirectory(testpkg.MustDatabase(db)),
		CareProfiles:        careProfiles(t, db),
		ChildRepo:           repos.ParentChildReads(),
		StudentRepo:         owners.NewParentStudentReads(reposPeople.Student),
		GuardianProfileRepo: owners.NewParentGuardianProfileReads(reposPeople.GuardianProfile),
		PersonRepo:          owners.NewParentPersonReads(reposPeople.Person),
		GuardianPhoneRepo:   owners.NewParentGuardianPhoneReads(reposPeople.GuardianPhoneNumber),
		ChangeRequestRepo:   owners.NewParentDataRequestReads(repos.CarePlan()),
		Settings:            settings,
		StudentUpdates:      owners.BindParentStudentUpdates(testpkg.NewRecordingBroadcaster()),
		Emitter:             owners.BindParentChildEvents(emitter),
		Logger:              slog.Default(),

		DataRequests: owners.NewParentDataRequestCommands(repos.CarePlan()),

		CarePlanPolicy: owners.NewParentCarePlanPolicy(),
	})

	chain := testpkg.CreateTestParentGuardianChain(t, db)

	// Two changed fields (current person is Felix Schneider) -> two rows.
	rows, err := svc.SubmitMasterDataChangeRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, []parentService.MasterDataFieldChange{
		{Target: parentService.DataChangeTargetPerson, FieldKey: "first_name", Value: json.RawMessage(`"Maximilian"`)},
		{Target: parentService.DataChangeTargetPerson, FieldKey: "last_name", Value: json.RawMessage(`"Neumann"`)},
	}, nil)
	require.NoError(t, err)
	require.Len(t, rows, 2, "two distinct field changes create two request rows")

	// Every created row must have its OWN request_created pill (found by its id),
	// so the decision path (keyed on the decided row's id) can always resolve it.
	err = testpkg.WithTenantTx(t, testpkg.WithPackageTenantRuntime(context.Background()), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		thread, gErr := testpkg.TenantQuery3(t, txCtx, db, reposMessages.ParentMessageThread.GetOrCreate, chain.TenantID, chain.StudentID, chain.AccountID)
		if gErr != nil {
			return gErr
		}
		for _, row := range rows {
			pill, fErr := testpkg.TenantQuery4(t, txCtx, db, communicationtest.NewParentMessageRepository(db).FindEventByRef, thread.ID, "request_created", "users.student_data_change_requests", row.ID)
			if fErr != nil {
				return fErr
			}
			require.NotNilf(t, pill, "row %d must have its own request_created pill", row.ID)
		}
		return nil
	})
	require.NoError(t, err)
}
