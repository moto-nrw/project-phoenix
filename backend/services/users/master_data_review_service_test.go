package users_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func insertPendingChange(t *testing.T, db *bun.DB, repos *repositories.Factory, c testpkg.ParentChain, target, field string, oldVal, newVal string) *userModels.StudentDataChangeRequest {
	t.Helper()
	row := &userModels.StudentDataChangeRequest{
		StudentID:   c.StudentID,
		SubmittedBy: c.AccountID,
		Target:      target,
		FieldKey:    field,
		OldValue:    json.RawMessage(oldVal),
		NewValue:    json.RawMessage(newVal),
		Status:      userModels.DataChangeStatusPending,
	}
	row.SetTenantID(c.TenantID)
	err := tenant.WithTenantTx(context.Background(), db, c.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		return repos.StudentDataChangeRequest.Create(txCtx, row)
	})
	require.NoError(t, err)
	return row
}

func TestMasterDataReview_ApproveAppliesNameChange(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() {
		require.NoError(t, db.Close())
	}()
	repos := repositories.NewFactory(db)
	svc := userService.NewMasterDataReviewService(repos.StudentDataChangeRequest, repos.Student, repos.Person, slog.Default())

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	row := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Maximilian"`)

	var decided *userModels.StudentDataChangeRequest
	err := tenant.WithTenantTx(context.Background(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		d, e := svc.Decide(txCtx, userService.MasterDataReviewDecideInput{RequestID: row.ID, Approve: true})
		decided = d
		return e
	})
	require.NoError(t, err)
	assert.Equal(t, userModels.DataChangeStatusApproved, decided.Status)
	require.NotNil(t, decided.AppliedAt)

	person, err := repos.Person.FindByID(context.Background(), chain.PersonID)
	require.NoError(t, err)
	assert.Equal(t, "Maximilian", person.FirstName)
}

func TestMasterDataReview_RejectLeavesRecordUnchanged(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() {
		require.NoError(t, db.Close())
	}()
	repos := repositories.NewFactory(db)
	svc := userService.NewMasterDataReviewService(repos.StudentDataChangeRequest, repos.Student, repos.Person, slog.Default())

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	row := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Maximilian"`)

	err := tenant.WithTenantTx(context.Background(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, e := svc.Decide(txCtx, userService.MasterDataReviewDecideInput{RequestID: row.ID, Approve: false, Reason: "Bitte Nachweis"})
		return e
	})
	require.NoError(t, err)

	person, err := repos.Person.FindByID(context.Background(), chain.PersonID)
	require.NoError(t, err)
	assert.Equal(t, "Felix", person.FirstName, "rejected change must not touch the record")
}

func TestMasterDataReview_DecideNonPendingRejected(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() {
		require.NoError(t, db.Close())
	}()
	repos := repositories.NewFactory(db)
	svc := userService.NewMasterDataReviewService(repos.StudentDataChangeRequest, repos.Student, repos.Person, slog.Default())

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	row := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Maximilian"`)

	// First decision approves; the second must fail as not-pending.
	err := tenant.WithTenantTx(context.Background(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, e := svc.Decide(txCtx, userService.MasterDataReviewDecideInput{RequestID: row.ID, Approve: true})
		return e
	})
	require.NoError(t, err)

	err = tenant.WithTenantTx(context.Background(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, e := svc.Decide(txCtx, userService.MasterDataReviewDecideInput{RequestID: row.ID, Approve: true})
		return e
	})
	assert.ErrorIs(t, err, userService.ErrReviewNotPending)
}
