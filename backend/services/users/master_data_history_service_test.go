package users_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestMasterDataReview_ListHistory proves the decided-request history:
// decided rows (incl. auto_applied) come back newest-decision-first with the
// child's and the reviewer's names, pending rows stay out, and the keyset
// cursor pages without overlap.
func TestMasterDataReview_ListHistory(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	svc := userService.NewMasterDataReviewService(repos.StudentDataChangeRequest, repos.Student, repos.Person, nil, nil, slog.Default())

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	_, reviewerAccount := testpkg.CreateTestStaffWithAccount(t, db, "Rieke", "Reviewer")

	// One row per terminal state, plus a pending row that must never surface.
	rejected := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Max"`)
	autoApplied := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetPerson, "last_name", `"Schneider"`, `"Schmidt"`)
	pending := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetPerson, "language_preference", `"de"`, `"en"`)

	ctx := authorizedCtx(context.Background())
	err := tenant.WithTenantTx(ctx, db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, e := svc.Decide(txCtx, userService.MasterDataReviewDecideInput{
			RequestID:  rejected.ID,
			Approve:    false,
			Reason:     "unpassend",
			ReviewedBy: reviewerAccount.ID,
		})
		return e
	})
	require.NoError(t, err)
	// The auto-applied row never went through Decide — flip it directly, like
	// the auto-apply path does (no reviewer).
	_, err = db.NewUpdate().
		Model((*userModels.StudentDataChangeRequest)(nil)).
		ModelTableExpr(`users.student_data_change_requests AS "student_data_change_request"`).
		Set("status = ?", userModels.DataChangeStatusAutoApplied).
		Set("applied_at = NOW()").
		Set("updated_at = NOW() + INTERVAL '1 second'").
		Where(`"student_data_change_request".id = ?`, autoApplied.ID).
		Exec(context.Background())
	require.NoError(t, err)

	err = tenant.WithTenantTx(ctx, db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		items, next, e := svc.ListHistory(txCtx, time.Time{}, 0, 25)
		require.NoError(t, e)
		assert.Nil(t, next, "one page must not report more")

		byID := map[int64]*userService.MasterDataHistoryItem{}
		for _, item := range items {
			byID[item.Request.ID] = item
			assert.NotEqual(t, pending.ID, item.Request.ID, "pending rows must never appear in the history")
		}
		require.Contains(t, byID, rejected.ID)
		require.Contains(t, byID, autoApplied.ID)

		rej := byID[rejected.ID]
		assert.Equal(t, "Felix", rej.FirstName)
		assert.Equal(t, "Schneider", rej.LastName)
		assert.Equal(t, "Rieke Reviewer", rej.ReviewerName)
		require.NotNil(t, rej.Request.ReviewReason)
		assert.Equal(t, "unpassend", *rej.Request.ReviewReason)

		auto := byID[autoApplied.ID]
		assert.Equal(t, userModels.DataChangeStatusAutoApplied, auto.Request.Status)
		assert.Empty(t, auto.ReviewerName, "auto-applied rows carry no reviewer")

		// The auto-applied row was stamped one second later, so it must lead.
		assert.Equal(t, autoApplied.ID, items[0].Request.ID, "newest decision first")

		// Keyset pagination: page size 1 → two pages, disjoint, then done.
		page1, next1, e := svc.ListHistory(txCtx, time.Time{}, 0, 1)
		require.NoError(t, e)
		require.Len(t, page1, 1)
		require.NotNil(t, next1)
		page2, next2, e := svc.ListHistory(txCtx, next1.UpdatedAt, next1.ID, 1)
		require.NoError(t, e)
		require.Len(t, page2, 1)
		assert.NotEqual(t, page1[0].Request.ID, page2[0].Request.ID, "pages must not overlap")
		if next2 != nil {
			page3, next3, e := svc.ListHistory(txCtx, next2.UpdatedAt, next2.ID, 1)
			require.NoError(t, e)
			assert.Empty(t, page3)
			assert.Nil(t, next3)
		}
		return nil
	})
	require.NoError(t, err)
}

// TestMasterDataReview_ListHistoryScopedToWritableChildren proves the history
// applies the same per-child write gate as the pending queue.
func TestMasterDataReview_ListHistoryScopedToWritableChildren(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	svc := userService.NewMasterDataReviewService(repos.StudentDataChangeRequest, repos.Student, repos.Person, nil, nil, slog.Default())

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	row := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Max"`)

	err := tenant.WithTenantTx(authorizedCtx(context.Background()), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, e := svc.Decide(txCtx, userService.MasterDataReviewDecideInput{RequestID: row.ID, Approve: false, Reason: "nein"})
		return e
	})
	require.NoError(t, err)

	denyBase := context.WithValue(context.Background(), jwt.CtxPermissions, []string{"users:update"})
	err = tenant.WithTenantTx(denyBase, db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		items, _, e := svc.ListHistory(txCtx, time.Time{}, 0, 25)
		require.NoError(t, e)
		for _, item := range items {
			assert.NotEqual(t, row.ID, item.Request.ID, "a caller who cannot write the child must not see its decided request")
		}
		return nil
	})
	require.NoError(t, err)
}
