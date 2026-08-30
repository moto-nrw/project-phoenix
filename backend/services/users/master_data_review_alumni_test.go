// Graduated children drop out of the master-data review surface (#405 review).
//
// Graduation is a soft delete: the guardian link and the pending request survive
// it (the hard delete it replaced cascaded them away). The staff queue hydrated
// through the unfiltered FindByIDs and decisions authorized through the
// unfiltered FindByID, so a departed child's request stayed reviewable — and
// approving it would have rewritten an alumnus' name, departure modes or
// companion links.
package users_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestMasterDataReview_GraduatedChildLeavesQueueAndRefusesDecisions(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	svc := userService.NewMasterDataReviewServiceWithAuditAndPolicy(
		repos.StudentDataChangeRequest, repos.Student, repos.Person, nil, nil, nil, testpkg.RequestReviewPolicy{}, nil, slog.Default())

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	row := insertPendingChange(t, db, repos, chain,
		userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Max"`)

	err := testpkg.WithTenantTx(t, authorizedCtx(context.Background()), db, chain.TenantID,
		func(txCtx context.Context, _ bun.Tx) error {
			items, _, e := svc.ListPending(txCtx, modelBase.RequestQueueFilters{})
			require.NoError(t, e)
			require.Len(t, items, 1, "the pending request must start out visible")
			return nil
		})
	require.NoError(t, err)

	// The child graduates (grade-transition apply).
	_, err = db.NewUpdate().
		TableExpr("users.students").
		Set("status = ?", string(userModels.StudentStatusAlumnus)).
		Where("id = ?", chain.StudentID).
		Exec(context.Background())
	require.NoError(t, err)

	err = testpkg.WithTenantTx(t, authorizedCtx(context.Background()), db, chain.TenantID,
		func(txCtx context.Context, _ bun.Tx) error {
			items, _, e := svc.ListPending(txCtx, modelBase.RequestQueueFilters{})
			require.NoError(t, e)
			assert.Empty(t, items, "a graduated child's request must leave the queue and the badge")

			// Neither direction may still act on the alumnus.
			_, e = svc.Decide(txCtx, userService.MasterDataReviewDecideInput{
				RequestID: row.ID, Approve: true, ReviewedBy: chain.AccountID,
			})
			assert.ErrorIs(t, e, userService.ErrReviewNotFound)

			_, e = svc.Decide(txCtx, userService.MasterDataReviewDecideInput{
				RequestID: row.ID, Approve: false, Reason: "child has left", ReviewedBy: chain.AccountID,
			})
			assert.ErrorIs(t, e, userService.ErrReviewNotFound)
			return nil
		})
	require.NoError(t, err)

	// The request is untouched, so a transition revert restores a coherent state.
	var status string
	require.NoError(t, db.NewSelect().
		TableExpr("users.student_data_change_requests").
		Column("status").
		Where("id = ?", row.ID).
		Scan(context.Background(), &status))
	assert.Equal(t, userModels.DataChangeStatusPending, status)
}
