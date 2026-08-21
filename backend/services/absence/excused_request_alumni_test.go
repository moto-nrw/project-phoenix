// Graduated children drop out of the excused-absence review surface (#405
// review).
//
// Graduation is a soft delete: the guardian link and the pending request survive
// it (the hard delete it replaced cascaded them away). The staff side hydrated
// the queue and the planning badge through the unfiltered FindByIDs and
// authorized decisions through the unfiltered FindByID, so a departed child's
// request stayed reviewable — and approving it would have written excused status
// days for an alumnus.
package absence_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	absenceSvc "github.com/moto-nrw/project-phoenix/services/absence"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestExcusedRequest_GraduatedChildLeavesQueueAndRefusesDecisions(t *testing.T) {
	t.Parallel()

	svc, _, db := buildAbsenceService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	day := timezone.TodayDate().AddDays(3)
	req := createPending(t, svc, db, chain, []timezone.Date{day}, "Arzttermin")

	err := tenant.WithTenantTx(adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		items, _, e := svc.ListPending(txCtx, modelBase.RequestQueueFilters{})
		require.NoError(t, e)
		require.Len(t, items, 1, "the pending request must start out visible")

		badges, e := svc.PendingByStudentForDate(txCtx, day)
		require.NoError(t, e)
		require.Contains(t, badges, chain.StudentID)
		return nil
	})
	require.NoError(t, err)

	// The child graduates (grade-transition apply).
	_, err = db.NewUpdate().
		TableExpr("users.students").
		Set("status = ?", string(usersModels.StudentStatusAlumnus)).
		Where("id = ?", chain.StudentID).
		Exec(context.Background())
	require.NoError(t, err)

	err = tenant.WithTenantTx(adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		items, _, e := svc.ListPending(txCtx, modelBase.RequestQueueFilters{})
		require.NoError(t, e)
		assert.Empty(t, items, "a graduated child's request must leave the queue and the badge")

		badges, e := svc.PendingByStudentForDate(txCtx, day)
		require.NoError(t, e)
		assert.NotContains(t, badges, chain.StudentID,
			"a graduated child must not raise a pending-approval marker on the planning surface")

		// Neither direction may still act on the alumnus.
		_, e = svc.Decide(txCtx, absenceSvc.ExcusedRequestDecideInput{
			RequestID: req.ID, Approve: true, ReviewedBy: chain.AccountID,
		})
		assert.ErrorIs(t, e, activeModels.ErrExcusedRequestNotFound)

		_, e = svc.Decide(txCtx, absenceSvc.ExcusedRequestDecideInput{
			RequestID: req.ID, Approve: false, Reason: "child has left", ReviewedBy: chain.AccountID,
		})
		assert.ErrorIs(t, e, activeModels.ErrExcusedRequestNotFound)
		return nil
	})
	require.NoError(t, err)

	// The request is untouched, so a transition revert restores a coherent state.
	var status string
	require.NoError(t, db.NewSelect().
		TableExpr("active.excused_absence_requests").
		Column("status").
		Where("id = ?", req.ID).
		Scan(context.Background(), &status))
	assert.Equal(t, activeModels.ExcusedRequestStatusPending, status)
}
