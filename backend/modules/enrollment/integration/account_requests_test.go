package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"

	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestAccountRequestsOwnershipFallbackScopeAndFailure(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := enrollmentCompose.New()
	account := testpkg.CreateTestAccount(t, db, "account-requests")
	other := testpkg.CreateTestAccount(t, db, "account-requests-other")
	var schoolIDs []int64
	for _, school := range []string{"first", "second"} {
		t.Run(school, func(t *testing.T) {
			testpkg.OwnTenant(t)
			phase := testpkg.CreateTestEnrollmentPhase(t, db)
			schoolID := testpkg.Tenant(t)
			schoolIDs = append(schoolIDs, schoolID)
			err := testpkg.WithTenantTx(t, testpkg.Ctx(t), db, schoolID, func(ctx context.Context, tx bun.Tx) error {
				for i, owner := range []*int64{nil, &account.ID, &other.ID} {
					var requestID int64
					err := tx.NewRaw(`INSERT INTO enrollment.requests
					 (tenant_id, phase_id, guardian_account_id, guardian_first_name, guardian_last_name, guardian_email, status_token, consent_flags, custom_data)
					 VALUES (?, ?, ?, 'First', 'Last', ?, ?, '{}'::jsonb, '{}'::jsonb) RETURNING id`,
						schoolID, phase.ID, owner, "  "+strings.ToUpper(account.Email)+"  ", fmt.Sprintf("account-requests-%d-%d", schoolID, i)).Scan(ctx, &requestID)
					if err != nil {
						return err
					}
					_, err = tx.NewRaw(`INSERT INTO enrollment.request_children
					 (tenant_id, request_id, first_name, last_name, date_of_birth, status, activation_mode, sort_order, custom_data)
					 VALUES (?, ?, 'Child', 'Last', '2018-04-15', 'submitted', 'scheduled', 0, '{}'::jsonb)`, schoolID, requestID).Exec(ctx)
					if err != nil {
						return err
					}
				}
				return nil
			})
			require.NoError(t, err)
		})
	}
	_, err := module.AccountRequests(context.Background(), account.ID, account.Email)
	require.ErrorContains(t, err, "transaction is required")
	counter := testpkg.CaptureQueriesForContext(t, db)
	err = tenant.WithAdminTx(testpkg.Ctx(t), db, func(ctx context.Context, _ bun.Tx) error {
		rows, err := module.AccountRequests(counter.Context(ctx), account.ID, account.Email)
		require.NoError(t, err)
		require.Len(t, rows, 4, "linked and unclaimed matching requests from both schools, never another account's claimed request")
		require.Len(t, counter.Operation("SELECT"), 2, "request and child rows are batch loaded")
		for _, row := range rows {
			require.Len(t, row.Children, 1)
		}
		return nil
	})
	require.NoError(t, err)
	ctx := testpkg.ContextForTenant(testpkg.Ctx(t), schoolIDs[0])
	err = testpkg.WithTenantTx(t, ctx, db, schoolIDs[0], func(txCtx context.Context, _ bun.Tx) error {
		rows, err := module.AccountRequests(txCtx, account.ID, account.Email)
		require.NoError(t, err)
		require.Len(t, rows, 2)
		for _, row := range rows {
			require.Equal(t, schoolIDs[0], row.TenantID)
		}
		return nil
	})
	require.NoError(t, err)
	err = tenant.WithAdminTx(testpkg.Ctx(t), db, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.NewRaw("SELECT 1 / 0").Exec(ctx)
		require.Error(t, err)
		_, audienceErr := module.PendingAnnouncementApplicants(testpkg.ContextForTenant(ctx, schoolIDs[0]))
		require.ErrorContains(t, audienceErr, "list pending announcement applicants")
		_, err = module.AccountRequests(ctx, account.ID, account.Email)
		require.ErrorContains(t, err, "parent: list enrollment requests")
		return err
	})
	require.Error(t, err)
	err = tenant.WithAdminTx(testpkg.Ctx(t), db, func(ctx context.Context, _ bun.Tx) error {
		rows, err := module.AccountRequests(ctx, account.ID, account.Email)
		require.NoError(t, err)
		require.Len(t, rows, 4, "retry succeeds after transaction rollback")
		return nil
	})
	require.NoError(t, err)
	applicants, err := module.PendingAnnouncementApplicants(ctx)
	require.NoError(t, err)
	require.Len(t, applicants, 3)
	err = testpkg.WithTenantTx(t, ctx, db, schoolIDs[0], func(txCtx context.Context, tx bun.Tx) error {
		_, err := tx.NewUpdate().TableExpr("enrollment.requests").Set("withdrawn_at = NOW()").Where("guardian_account_id = ?", other.ID).Exec(txCtx)
		if err != nil {
			return err
		}
		_, err = tx.NewRaw(`UPDATE enrollment.request_children SET status = 'approved'
		 WHERE request_id IN (SELECT id FROM enrollment.requests WHERE guardian_account_id IS NULL)`).Exec(txCtx)
		return err
	})
	require.NoError(t, err)
	applicants, err = module.PendingAnnouncementApplicants(ctx)
	require.NoError(t, err)
	require.Len(t, applicants, 1, "withdrawn requests and decided children are excluded")
	require.Equal(t, &account.ID, applicants[0].GuardianAccountID)
	applicants, err = module.PendingAnnouncementApplicants(testpkg.ContextForTenant(testpkg.Ctx(t), schoolIDs[1]))
	require.NoError(t, err)
	require.Len(t, applicants, 3, "the other school's pending audience remains unchanged")
	err = tenant.WithAdminTx(testpkg.Ctx(t), db, func(adminCtx context.Context, _ bun.Tx) error {
		rows, err := module.PendingAnnouncementApplicantsForSchools(adminCtx, schoolIDs)
		require.NoError(t, err)
		require.Len(t, rows, 4)
		rows, err = module.PendingAnnouncementApplicantsForSchools(adminCtx, schoolIDs[:1])
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.Equal(t, schoolIDs[0], rows[0].TenantID)
		rows, err = module.PendingAnnouncementApplicantsForSchools(adminCtx, nil)
		require.NoError(t, err)
		require.Empty(t, rows, "empty school scope must not widen cross-school discovery")
		return nil
	})
	require.NoError(t, err)
	applicants, err = module.PendingAnnouncementApplicantsForSchools(ctx, schoolIDs)
	require.NoError(t, err)
	require.Len(t, applicants, 1, "tenant RLS still narrows a multi-school filter")
}
