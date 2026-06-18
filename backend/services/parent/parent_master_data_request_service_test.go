package parent_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
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
