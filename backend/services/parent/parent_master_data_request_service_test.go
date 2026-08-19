package parent_test

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

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func buildRequestService(t *testing.T) (parentService.Service, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:           repos.ParentChild,
		StudentRepo:         repos.Student,
		GuardianProfileRepo: repos.GuardianProfile,
		PersonRepo:          repos.Person,
		GuardianPhoneRepo:   repos.GuardianPhoneNumber,
		ChangeRequestRepo:   repos.StudentDataChangeRequest,
		Settings:            masterDataSettings(false, true, false),
		Broadcaster:         testpkg.NewRecordingBroadcaster(),
		DB:                  db,
		Logger:              slog.Default(),
	})
	return svc, db
}

func TestSubmitMasterDataChangeRequest_CreatesPending(t *testing.T) {
	svc, db := buildRequestService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	rows, err := svc.SubmitMasterDataChangeRequest(context.Background(), chain.AccountID, chain.StudentID, []parentService.MasterDataFieldChange{
		{Target: usersModels.DataChangeTargetPerson, FieldKey: "first_name", Value: json.RawMessage(`"Maximilian"`)},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, usersModels.DataChangeStatusPending, rows[0].Status)

	// The live person record is NOT changed by a Track B submission.
	person, err := repositories.NewFactory(db).Person.FindByID(context.Background(), chain.PersonID)
	require.NoError(t, err)
	assert.Equal(t, "Felix", person.FirstName)
}

func TestSubmitMasterDataChangeRequest_SchoolClassRemainsPending(t *testing.T) {
	svc, db := buildRequestService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	rows, err := svc.SubmitMasterDataChangeRequest(context.Background(), chain.AccountID, chain.StudentID, []parentService.MasterDataFieldChange{
		{Target: usersModels.DataChangeTargetStudent, FieldKey: "school_class", Value: json.RawMessage(`"2b"`)},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, usersModels.DataChangeTargetStudent, rows[0].Target)
	assert.Equal(t, "school_class", rows[0].FieldKey)
	assert.JSONEq(t, `"1a"`, string(rows[0].OldValue))
	assert.JSONEq(t, `"2b"`, string(rows[0].NewValue))

	student, err := repositories.NewFactory(db).Student.FindByID(context.Background(), chain.StudentID)
	require.NoError(t, err)
	assert.Equal(t, "1a", student.SchoolClass)
}

func TestSubmitMasterDataChangeRequest_DepartureAndListRequests(t *testing.T) {
	svc, db := buildRequestService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	rows, err := svc.SubmitMasterDataChangeRequest(context.Background(), chain.AccountID, chain.StudentID, []parentService.MasterDataFieldChange{
		{
			Target:   usersModels.DataChangeTargetDeparture,
			FieldKey: "allowed_departure_modes",
			Value:    json.RawMessage(`{"mon":["pickup"],"wed":["bus"]}`),
		},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, usersModels.DataChangeTargetDeparture, rows[0].Target)
	assert.Equal(t, "allowed_departure_modes", rows[0].FieldKey)
	assert.JSONEq(t, `{"mon":["pickup"],"wed":["bus"]}`, string(rows[0].NewValue))

	listed, err := svc.ListMyMasterDataRequests(context.Background(), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, rows[0].ID, listed[0].ID)
}

func TestListMyMasterDataRequests_HidesGuardianContactAuditRows(t *testing.T) {
	svc, db := buildRequestService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	repos := repositories.NewFactory(db)

	otherAccount := testpkg.CreateTestAccount(t, db, "other-parent")
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, db, "auth.accounts", otherAccount.ID) })

	err := tenant.WithTenantTx(context.Background(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		privateAudit := &usersModels.StudentDataChangeRequest{
			StudentID:   chain.StudentID,
			SubmittedBy: otherAccount.ID,
			Target:      usersModels.DataChangeTargetGuardianProfile,
			FieldKey:    "email",
			OldValue:    json.RawMessage(`"other-old@example.test"`),
			NewValue:    json.RawMessage(`"other-new@example.test"`),
			Status:      usersModels.DataChangeStatusAutoApplied,
		}
		privateAudit.SetTenantID(chain.TenantID)
		if createErr := repos.StudentDataChangeRequest.Create(txCtx, privateAudit); createErr != nil {
			return createErr
		}

		visibleRequest := &usersModels.StudentDataChangeRequest{
			StudentID:   chain.StudentID,
			SubmittedBy: otherAccount.ID,
			Target:      usersModels.DataChangeTargetPerson,
			FieldKey:    "first_name",
			OldValue:    json.RawMessage(`"Felix"`),
			NewValue:    json.RawMessage(`"Max"`),
			Status:      usersModels.DataChangeStatusPending,
		}
		visibleRequest.SetTenantID(chain.TenantID)
		return repos.StudentDataChangeRequest.Create(txCtx, visibleRequest)
	})
	require.NoError(t, err)

	rows, err := svc.ListMyMasterDataRequests(context.Background(), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, usersModels.DataChangeTargetPerson, rows[0].Target)
	assert.JSONEq(t, `"Max"`, string(rows[0].NewValue))
}

func TestSubmitMasterDataChangeRequest_DuplicatePending(t *testing.T) {
	svc, db := buildRequestService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	change := []parentService.MasterDataFieldChange{
		{Target: usersModels.DataChangeTargetPerson, FieldKey: "first_name", Value: json.RawMessage(`"Maximilian"`)},
	}
	_, err := svc.SubmitMasterDataChangeRequest(context.Background(), chain.AccountID, chain.StudentID, change)
	require.NoError(t, err)

	_, err = svc.SubmitMasterDataChangeRequest(context.Background(), chain.AccountID, chain.StudentID, change)
	assert.ErrorIs(t, err, parentService.ErrMasterDataDuplicatePending)
}

func TestSubmitMasterDataChangeRequest_ConcurrentDuplicatePending(t *testing.T) {
	svc, db := buildRequestService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	change := []parentService.MasterDataFieldChange{
		{Target: usersModels.DataChangeTargetPerson, FieldKey: "first_name", Value: json.RawMessage(`"Maximilian"`)},
	}

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = svc.SubmitMasterDataChangeRequest(context.Background(), chain.AccountID, chain.StudentID, change)
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
	svc, db := buildRequestService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	// Submitting the current value is a no-op -> ErrMasterDataNoChanges.
	_, err := svc.SubmitMasterDataChangeRequest(context.Background(), chain.AccountID, chain.StudentID, []parentService.MasterDataFieldChange{
		{Target: usersModels.DataChangeTargetPerson, FieldKey: "first_name", Value: json.RawMessage(`"Felix"`)},
	})
	assert.ErrorIs(t, err, parentService.ErrMasterDataNoChanges)
}

func TestSubmitMasterDataChangeRequest_InvalidInputs(t *testing.T) {
	svc, db := buildRequestService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	_, err := svc.SubmitMasterDataChangeRequest(context.Background(), chain.AccountID, chain.StudentID, nil)
	assert.ErrorIs(t, err, parentService.ErrMasterDataNoChanges)

	_, err = svc.SubmitMasterDataChangeRequest(context.Background(), chain.AccountID, chain.StudentID, []parentService.MasterDataFieldChange{
		{Target: usersModels.DataChangeTargetStudent, FieldKey: "health_info", Value: json.RawMessage(`"x"`)},
	})
	assert.ErrorIs(t, err, parentService.ErrMasterDataFieldNotEditable)

	_, err = svc.SubmitMasterDataChangeRequest(context.Background(), chain.AccountID, chain.StudentID, []parentService.MasterDataFieldChange{
		{Target: usersModels.DataChangeTargetPerson, FieldKey: "birthday", Value: json.RawMessage(`"not-a-date"`)},
	})
	assert.ErrorIs(t, err, parentService.ErrMasterDataInvalidValue)

	_, err = svc.SubmitMasterDataChangeRequest(context.Background(), chain.AccountID, chain.StudentID, []parentService.MasterDataFieldChange{
		{Target: usersModels.DataChangeTargetDeparture, FieldKey: "allowed_departure_modes", Value: json.RawMessage(`{`)},
	})
	assert.ErrorIs(t, err, parentService.ErrMasterDataInvalidValue)

	_, err = svc.SubmitMasterDataChangeRequest(context.Background(), chain.AccountID, chain.StudentID, []parentService.MasterDataFieldChange{
		{Target: usersModels.DataChangeTargetDeparture, FieldKey: "allowed_departure_modes", Value: json.RawMessage(`{"sat":["pickup"]}`)},
	})
	assert.ErrorIs(t, err, parentService.ErrMasterDataInvalidValue)

	_, err = svc.SubmitMasterDataChangeRequest(context.Background(), chain.AccountID, chain.StudentID, []parentService.MasterDataFieldChange{
		{Target: usersModels.DataChangeTargetDeparture, FieldKey: "allowed_departure_modes", Value: json.RawMessage(`{"mon":["spaceship"]}`)},
	})
	assert.ErrorIs(t, err, parentService.ErrMasterDataInvalidValue)

	_, err = svc.SubmitMasterDataChangeRequest(context.Background(), chain.AccountID, chain.StudentID, []parentService.MasterDataFieldChange{
		{Target: usersModels.DataChangeTargetDeparture, FieldKey: "allowed_departure_modes", Value: json.RawMessage(`{"mon":["accompanied"]}`)},
	})
	assert.ErrorIs(t, err, parentService.ErrMasterDataInvalidValue)
}

// TestSubmitMasterDataChangeRequest_PerRowCreatedPills pins the P3 fix (#1803
// review follow-up): a multi-field submit emits ONE "Anfrage erstellt" pill PER
// created row, each referencing its own row — not a single pill tied to the
// first row. Per-row pills keep each field's created↔decision refs paired, so
// the thread timeline and the deep-link into the Änderungsanfragen queue can
// resolve the exact row for any field, not just the first.
func TestSubmitMasterDataChangeRequest_PerRowCreatedPills(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	settings := parentSettingsStub{boolDefault: true}
	emitter := parentmessaging.NewEmitter(
		db, repos.ParentMessageThread, repos.ParentMessage,
		settings, nil, slog.Default(),
	)
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:           repos.ParentChild,
		StudentRepo:         repos.Student,
		GuardianProfileRepo: repos.GuardianProfile,
		PersonRepo:          repos.Person,
		GuardianPhoneRepo:   repos.GuardianPhoneNumber,
		ChangeRequestRepo:   repos.StudentDataChangeRequest,
		Settings:            settings,
		Broadcaster:         testpkg.NewRecordingBroadcaster(),
		Emitter:             emitter,
		DB:                  db,
		Logger:              slog.Default(),
	})

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	// Two changed fields (current person is Felix Schneider) -> two rows.
	rows, err := svc.SubmitMasterDataChangeRequest(context.Background(), chain.AccountID, chain.StudentID, []parentService.MasterDataFieldChange{
		{Target: usersModels.DataChangeTargetPerson, FieldKey: "first_name", Value: json.RawMessage(`"Maximilian"`)},
		{Target: usersModels.DataChangeTargetPerson, FieldKey: "last_name", Value: json.RawMessage(`"Neumann"`)},
	})
	require.NoError(t, err)
	require.Len(t, rows, 2, "two distinct field changes create two request rows")

	// Every created row must have its OWN request_created pill (found by its id),
	// so the decision path (keyed on the decided row's id) can always resolve it.
	err = tenant.WithTenantTx(context.Background(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		thread, gErr := repos.ParentMessageThread.GetOrCreate(txCtx, chain.TenantID, chain.StudentID, chain.AccountID)
		if gErr != nil {
			return gErr
		}
		for _, row := range rows {
			pill, fErr := repos.ParentMessage.FindEventByRef(txCtx, thread.ID, "request_created", "users.student_data_change_requests", row.ID)
			if fErr != nil {
				return fErr
			}
			require.NotNilf(t, pill, "row %d must have its own request_created pill", row.ID)
		}
		return nil
	})
	require.NoError(t, err)
}
