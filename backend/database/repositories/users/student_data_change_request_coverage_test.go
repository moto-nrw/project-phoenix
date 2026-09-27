package users_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func coverageChangeRequest(studentID, accountID, tenantID int64, target, field, status, value string) *testpkg.StudentDataChangeRequest {
	row := &testpkg.StudentDataChangeRequest{
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

func coverageContainsChangeRequest(rows []*testpkg.StudentDataChangeRequest, id int64) bool {
	for _, row := range rows {
		if row.ID == id {
			return true
		}
	}
	return false
}

func TestStudentDataChangeRequestRepository_CoverageFiltersAndDecisionBranches(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	chain := testpkg.CreateTestParentGuardianChain(t, db)

	repo := testutil.NewPeopleRepositorySuiteFactory(db).StudentDataChangeRequest
	ctx := testpkg.TenantContext(chain.TenantID)

	pending := coverageChangeRequest(chain.StudentID, chain.AccountID, chain.TenantID,
		testpkg.DataChangeTargetPerson, "first_name", testpkg.DataChangeStatusPending, `"Maximilian"`)
	require.NoError(t, repo.Create(ctx, pending))
	rejected := coverageChangeRequest(chain.StudentID, chain.AccountID, chain.TenantID,
		testpkg.DataChangeTargetPerson, "last_name", testpkg.DataChangeStatusRejected, `"Müller"`)
	require.NoError(t, repo.Create(ctx, rejected))

	all, err := repo.ListByStudent(ctx, chain.StudentID, nil, 1)
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, rejected.ID, all[0].ID)

	onlyPending, err := repo.ListByStudent(ctx, chain.StudentID, []string{testpkg.DataChangeStatusPending}, 0)
	require.NoError(t, err)
	require.Len(t, onlyPending, 1)
	assert.Equal(t, pending.ID, onlyPending[0].ID)

	has, err := repo.HasPendingForField(ctx, chain.StudentID, testpkg.DataChangeTargetPerson, "first_name")
	require.NoError(t, err)
	assert.True(t, has)
	has, err = repo.HasPendingForField(ctx, chain.StudentID, testpkg.DataChangeTargetPerson, "last_name")
	require.NoError(t, err)
	assert.False(t, has)

	reason := "not enough detail"
	require.NoError(t, repo.Decide(ctx, pending.ID, testpkg.DataChangeStatusRejected, &reason, 0, false))
	decided, err := repo.FindByID(ctx, pending.ID)
	require.NoError(t, err)
	assert.Equal(t, testpkg.DataChangeStatusRejected, decided.Status)
	assert.Nil(t, decided.ReviewedBy)
	assert.Nil(t, decided.AppliedAt)

	err = repo.Decide(ctx, pending.ID, testpkg.DataChangeStatusApproved, nil, chain.AccountID, true)
	assert.ErrorIs(t, err, testpkg.ErrChangeRequestNotPending)

	_, err = repo.FindPendingByIDForUpdate(ctx, pending.ID)
	assert.ErrorIs(t, err, testpkg.ErrChangeRequestNotPending)
	_, err = repo.FindPendingByIDForUpdate(ctx, 999_999_999)
	assert.ErrorIs(t, err, testpkg.ErrChangeRequestNotFound)
}

func TestStudentDataChangeRequestRepository_CoveragePendingQueueTenantIsolation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := testutil.NewPeopleRepositorySuiteFactory(db).StudentDataChangeRequest

	chainA := testpkg.CreateTestParentGuardianChain(t, db)
	ctxA := testpkg.TenantContext(chainA.TenantID)
	rowA := coverageChangeRequest(chainA.StudentID, chainA.AccountID, chainA.TenantID,
		testpkg.DataChangeTargetPerson, "first_name", testpkg.DataChangeStatusPending, `"Maximilian"`)
	require.NoError(t, repo.Create(ctxA, rowA))

	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantB)
	studentB := testpkg.CreateTestStudentForTenant(t, db, tenantB, "Other", "Child", "2b")
	accountB := testpkg.CreateTestAccount(t, db, "parent")
	ctxB := testpkg.TenantContext(tenantB)
	rowB := coverageChangeRequest(studentB.ID, accountB.ID, tenantB,
		testpkg.DataChangeTargetPerson, "first_name", testpkg.DataChangeStatusPending, `"Lena"`)
	require.NoError(t, repo.Create(ctxB, rowB))

	pendingA, err := repo.ListPendingForTenant(ctxA, testpkg.RequestQueueFilters{})
	require.NoError(t, err)
	assert.True(t, coverageContainsChangeRequest(pendingA, rowA.ID))
	assert.False(t, coverageContainsChangeRequest(pendingA, rowB.ID))

	pendingB, err := repo.ListPendingForTenant(ctxB, testpkg.RequestQueueFilters{})
	require.NoError(t, err)
	require.Len(t, pendingB, 1)
	assert.Equal(t, rowB.ID, pendingB[0].ID)

	require.NoError(t, repo.Decide(ctxB, rowB.ID, testpkg.DataChangeStatusApproved, nil, accountB.ID, true))
	approved, err := repo.FindByID(ctxB, rowB.ID)
	require.NoError(t, err)
	assert.NotNil(t, approved.ReviewedBy)
	assert.NotNil(t, approved.AppliedAt)
}
