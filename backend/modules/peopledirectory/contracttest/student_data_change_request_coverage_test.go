package contracttest_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	peopleVocabulary "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func coverageChangeRequest(studentID, accountID, tenantID int64, target, field, status, value string) *userModels.StudentDataChangeRequest {
	row := &userModels.StudentDataChangeRequest{
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

func coverageContainsChangeRequest(rows []*userModels.StudentDataChangeRequest, id int64) bool {
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

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).StudentDataChangeRequest
	ctx := testpkg.TenantContext(chain.TenantID)

	pending := coverageChangeRequest(chain.StudentID, chain.AccountID, chain.TenantID,
		userModels.DataChangeTargetPerson, "first_name", userModels.DataChangeStatusPending, `"Maximilian"`)
	require.NoError(t, repo.Create(ctx, pending))
	rejected := coverageChangeRequest(chain.StudentID, chain.AccountID, chain.TenantID,
		userModels.DataChangeTargetPerson, "last_name", userModels.DataChangeStatusRejected, `"Müller"`)
	require.NoError(t, repo.Create(ctx, rejected))

	all, err := repo.ListByStudent(ctx, chain.StudentID, nil, 1)
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, rejected.ID, all[0].ID)

	onlyPending, err := repo.ListByStudent(ctx, chain.StudentID, []string{userModels.DataChangeStatusPending}, 0)
	require.NoError(t, err)
	require.Len(t, onlyPending, 1)
	assert.Equal(t, pending.ID, onlyPending[0].ID)

	has, err := repo.HasPendingForField(ctx, chain.StudentID, userModels.DataChangeTargetPerson, "first_name")
	require.NoError(t, err)
	assert.True(t, has)
	has, err = repo.HasPendingForField(ctx, chain.StudentID, userModels.DataChangeTargetPerson, "last_name")
	require.NoError(t, err)
	assert.False(t, has)

	reason := "not enough detail"
	require.NoError(t, repo.Decide(ctx, pending.ID, userModels.DataChangeStatusRejected, &reason, 0, false))
	decided, err := repo.FindByID(ctx, pending.ID)
	require.NoError(t, err)
	assert.Equal(t, userModels.DataChangeStatusRejected, decided.Status)
	assert.Nil(t, decided.ReviewedBy)
	assert.Nil(t, decided.AppliedAt)

	err = repo.Decide(ctx, pending.ID, userModels.DataChangeStatusApproved, nil, chain.AccountID, true)
	assert.ErrorIs(t, err, userModels.ErrChangeRequestNotPending)

	_, err = repo.FindPendingByIDForUpdate(ctx, pending.ID)
	assert.ErrorIs(t, err, userModels.ErrChangeRequestNotPending)
	_, err = repo.FindPendingByIDForUpdate(ctx, 999_999_999)
	assert.ErrorIs(t, err, userModels.ErrChangeRequestNotFound)
}

func TestStudentDataChangeRequestRepository_CoveragePendingQueueTenantIsolation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).StudentDataChangeRequest

	chainA := testpkg.CreateTestParentGuardianChain(t, db)
	ctxA := testpkg.TenantContext(chainA.TenantID)
	rowA := coverageChangeRequest(chainA.StudentID, chainA.AccountID, chainA.TenantID,
		userModels.DataChangeTargetPerson, "first_name", userModels.DataChangeStatusPending, `"Maximilian"`)
	require.NoError(t, repo.Create(ctxA, rowA))

	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantB)
	studentB := testpkg.CreateTestStudentForTenant(t, db, tenantB, "Other", "Child", "2b")
	accountB := testpkg.CreateTestAccount(t, db, "parent")
	ctxB := testpkg.TenantContext(tenantB)
	rowB := coverageChangeRequest(studentB.ID, accountB.ID, tenantB,
		userModels.DataChangeTargetPerson, "first_name", userModels.DataChangeStatusPending, `"Lena"`)
	require.NoError(t, repo.Create(ctxB, rowB))

	pendingA, err := repo.ListPendingForTenant(ctxA, peopleVocabulary.RequestQueueFilters{})
	require.NoError(t, err)
	assert.True(t, coverageContainsChangeRequest(pendingA, rowA.ID))
	assert.False(t, coverageContainsChangeRequest(pendingA, rowB.ID))

	pendingB, err := repo.ListPendingForTenant(ctxB, peopleVocabulary.RequestQueueFilters{})
	require.NoError(t, err)
	require.Len(t, pendingB, 1)
	assert.Equal(t, rowB.ID, pendingB[0].ID)

	require.NoError(t, repo.Decide(ctxB, rowB.ID, userModels.DataChangeStatusApproved, nil, accountB.ID, true))
	approved, err := repo.FindByID(ctxB, rowB.ID)
	require.NoError(t, err)
	assert.NotNil(t, approved.ReviewedBy)
	assert.NotNil(t, approved.AppliedAt)
}
