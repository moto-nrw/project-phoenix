package absence_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	absenceSvc "github.com/moto-nrw/project-phoenix/services/absence"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestListHistory_DecidedExcusedRequests proves the staff history: decided
// requests come back with the child's and the reviewer's names, pending rows
// stay out, withdrawals carry no reviewer, and the keyset cursor pages without
// overlap.
func TestListHistory_DecidedExcusedRequests(t *testing.T) {
	svc, _, db := buildAbsenceService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() { testpkg.CleanupParentGuardianChain(t, db, chain) })
	staff, staffAccount := testpkg.CreateTestStaffWithAccount(t, db, "Erika", "Entscheider")
	t.Cleanup(func() { testpkg.CleanupStaffFixtures(t, db, staff.ID) })

	day := timezone.TodayDate().AddDays(3)
	rejected := createPending(t, svc, db, chain, []timezone.Date{day}, "Zahnarzt")
	withdrawn := createPending(t, svc, db, chain, []timezone.Date{day.AddDays(1)}, "Familienfeier")
	pending := createPending(t, svc, db, chain, []timezone.Date{day.AddDays(2)}, "Ausflug")

	err := tenant.WithTenantTx(adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		if _, e := svc.Decide(txCtx, absenceSvc.ExcusedRequestDecideInput{
			RequestID: rejected.ID, Approve: false, Reason: "bitte anrufen", ReviewedBy: staffAccount.ID,
		}); e != nil {
			return e
		}
		_, e := svc.WithdrawRequest(txCtx, withdrawn.ID, chain.StudentID, chain.AccountID)
		return e
	})
	require.NoError(t, err)

	err = tenant.WithTenantTx(adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		items, next, e := svc.ListHistory(txCtx, time.Time{}, 0, 25)
		require.NoError(t, e)
		assert.Nil(t, next)

		byID := map[int64]*absenceSvc.ExcusedRequestHistoryItem{}
		for _, item := range items {
			byID[item.Request.ID] = item
			assert.NotEqual(t, pending.ID, item.Request.ID, "pending rows must never appear in the history")
		}
		require.Contains(t, byID, rejected.ID)
		require.Contains(t, byID, withdrawn.ID)

		rej := byID[rejected.ID]
		assert.Equal(t, activeModels.ExcusedRequestStatusRejected, rej.Request.Status)
		assert.Equal(t, "Felix", rej.FirstName)
		assert.Equal(t, "Schneider", rej.LastName)
		assert.Equal(t, "Erika Entscheider", rej.ReviewerName)
		require.NotNil(t, rej.Request.DecisionReason)
		assert.Equal(t, "bitte anrufen", *rej.Request.DecisionReason)

		wd := byID[withdrawn.ID]
		assert.Equal(t, activeModels.ExcusedRequestStatusWithdrawn, wd.Request.Status)
		assert.Empty(t, wd.ReviewerName, "withdrawals carry no reviewer")

		// Keyset pagination: two decided rows, page size 1 → disjoint pages.
		page1, next1, e := svc.ListHistory(txCtx, time.Time{}, 0, 1)
		require.NoError(t, e)
		require.Len(t, page1, 1)
		cursor := next1
		require.NotNil(t, cursor)
		page2, next2, e := svc.ListHistory(txCtx, cursor.UpdatedAt, cursor.ID, 1)
		require.NoError(t, e)
		require.Len(t, page2, 1)
		assert.NotEqual(t, page1[0].Request.ID, page2[0].Request.ID, "pages must not overlap")
		assert.Nil(t, next2)
		return nil
	})
	require.NoError(t, err)
}
