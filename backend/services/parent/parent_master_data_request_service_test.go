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
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func buildRequestService(t *testing.T) (parentService.Service, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	repos := repositories.NewFactory(db)
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:           repos.ParentChild,
		StudentRepo:         repos.Student,
		GuardianProfileRepo: repos.GuardianProfile,
		PersonRepo:          repos.Person,
		GuardianPhoneRepo:   repos.GuardianPhoneNumber,
		ChangeRequestRepo:   repos.StudentDataChangeRequest,
		Settings:            masterDataStubSettings{requestEnabled: true},
		Broadcaster:         &captureBroadcaster{},
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
