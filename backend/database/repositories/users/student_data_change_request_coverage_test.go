package users_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func coverageChangeRequest(studentID, accountID, tenantID int64, target, field, status, value string) *usersModels.StudentDataChangeRequest {
	row := &usersModels.StudentDataChangeRequest{
		StudentID:   studentID,
		SubmittedBy: accountID,
		Target:      target,
		FieldKey:    field,
		NewValue:    json.RawMessage(value),
		Status:      status,
	}
	row.SetTenantID(tenantID)
	return row
}

func coverageContainsChangeRequest(rows []*usersModels.StudentDataChangeRequest, id int64) bool {
	for _, row := range rows {
		if row.ID == id {
			return true
		}
	}
	return false
}

func TestStudentDataChangeRequestRepository_CoverageFiltersAndDecisionBranches(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	repo := usersRepo.NewStudentDataChangeRequestRepository(db)
	ctx := tenant.WithTenantID(context.Background(), chain.TenantID)

	pending := coverageChangeRequest(chain.StudentID, chain.AccountID, chain.TenantID,
		usersModels.DataChangeTargetPerson, "first_name", usersModels.DataChangeStatusPending, `"Maximilian"`)
	require.NoError(t, repo.Create(ctx, pending))
	rejected := coverageChangeRequest(chain.StudentID, chain.AccountID, chain.TenantID,
		usersModels.DataChangeTargetPerson, "last_name", usersModels.DataChangeStatusRejected, `"Müller"`)
	require.NoError(t, repo.Create(ctx, rejected))

	all, err := repo.ListByStudent(ctx, chain.StudentID, nil, 1)
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, rejected.ID, all[0].ID)

	onlyPending, err := repo.ListByStudent(ctx, chain.StudentID, []string{usersModels.DataChangeStatusPending}, 0)
	require.NoError(t, err)
	require.Len(t, onlyPending, 1)
	assert.Equal(t, pending.ID, onlyPending[0].ID)

	has, err := repo.HasPendingForField(ctx, chain.StudentID, usersModels.DataChangeTargetPerson, "first_name")
	require.NoError(t, err)
	assert.True(t, has)
	has, err = repo.HasPendingForField(ctx, chain.StudentID, usersModels.DataChangeTargetPerson, "last_name")
	require.NoError(t, err)
	assert.False(t, has)

	reason := "not enough detail"
	require.NoError(t, repo.Decide(ctx, pending.ID, usersModels.DataChangeStatusRejected, &reason, 0, false))
	decided, err := repo.FindByID(ctx, pending.ID)
	require.NoError(t, err)
	assert.Equal(t, usersModels.DataChangeStatusRejected, decided.Status)
	assert.Nil(t, decided.ReviewedBy)
	assert.Nil(t, decided.AppliedAt)

	err = repo.Decide(ctx, pending.ID, usersModels.DataChangeStatusApproved, nil, chain.AccountID, true)
	assert.ErrorIs(t, err, usersRepo.ErrChangeRequestNotPending)

	_, err = repo.FindPendingByIDForUpdate(ctx, pending.ID)
	assert.ErrorIs(t, err, usersRepo.ErrChangeRequestNotPending)
	_, err = repo.FindPendingByIDForUpdate(ctx, 999_999_999)
	assert.ErrorIs(t, err, usersRepo.ErrChangeRequestNotFound)
}

func TestStudentDataChangeRequestRepository_CoveragePendingQueueTenantIsolation(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := usersRepo.NewStudentDataChangeRequestRepository(db)

	chainA := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chainA)
	ctxA := tenant.WithTenantID(context.Background(), chainA.TenantID)
	rowA := coverageChangeRequest(chainA.StudentID, chainA.AccountID, chainA.TenantID,
		usersModels.DataChangeTargetPerson, "first_name", usersModels.DataChangeStatusPending, `"Maximilian"`)
	require.NoError(t, repo.Create(ctxA, rowA))

	const tenantB int64 = 90213
	testpkg.CleanupTenantTestData(t, db, tenantB)
	t.Cleanup(func() { testpkg.CleanupTenantTestData(t, db, tenantB) })
	testpkg.EnsureTestTenant(t, db, tenantB)
	studentB := testpkg.CreateTestStudentForTenant(t, db, tenantB, "Other", "Child", "2b")
	accountB := testpkg.CreateTestAccount(t, db, "parent")
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, db, "auth.accounts", accountB.ID) })
	ctxB := tenant.WithTenantID(context.Background(), tenantB)
	rowB := coverageChangeRequest(studentB.ID, accountB.ID, tenantB,
		usersModels.DataChangeTargetPerson, "first_name", usersModels.DataChangeStatusPending, `"Lena"`)
	require.NoError(t, repo.Create(ctxB, rowB))

	pendingA, err := repo.ListPendingForTenant(ctxA)
	require.NoError(t, err)
	assert.True(t, coverageContainsChangeRequest(pendingA, rowA.ID))
	assert.False(t, coverageContainsChangeRequest(pendingA, rowB.ID))

	pendingB, err := repo.ListPendingForTenant(ctxB)
	require.NoError(t, err)
	require.Len(t, pendingB, 1)
	assert.Equal(t, rowB.ID, pendingB[0].ID)

	require.NoError(t, repo.Decide(ctxB, rowB.ID, usersModels.DataChangeStatusApproved, nil, accountB.ID, true))
	approved, err := repo.FindByID(ctxB, rowB.ID)
	require.NoError(t, err)
	assert.NotNil(t, approved.ReviewedBy)
	assert.NotNil(t, approved.AppliedAt)
}
