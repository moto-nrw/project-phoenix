package integration

import (
	"context"
	"errors"
	"fmt"
	"testing"

	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestGuardianAccountBackfillTenantScopeRollbackAndRetry(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := enrollmentCompose.New()
	account := testpkg.CreateTestAccount(t, db, "enrollment-backfill")
	email := account.Email
	tenantIDs := make([]int64, 0, 2)
	for _, name := range []string{"first school", "second school"} {
		t.Run(name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			phase := testpkg.CreateTestEnrollmentPhase(t, db)
			tenantID := testpkg.Tenant(t)
			tenantIDs = append(tenantIDs, tenantID)
			err := testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
				_, insertErr := tx.NewRaw(`
     INSERT INTO enrollment.requests
      (tenant_id, phase_id, guardian_first_name, guardian_last_name, guardian_email, status_token)
     VALUES (?, ?, 'Test', 'Guardian', ?, ?)
    `, tenantID, phase.ID, email, fmt.Sprintf("backfill-%d", phase.ID)).Exec(ctx)
				return insertErr
			})
			require.NoError(t, err)
		})
	}
	failAfterWrite := errors.New("injected failure after guardian attachment")
	err := testpkg.WithTenantTx(t, context.Background(), db, tenantIDs[0], func(ctx context.Context, _ bun.Tx) error {
		changed, commandErr := module.BackfillGuardianAccountID(ctx, account.ID, email)
		require.NoError(t, commandErr)
		require.Equal(t, 1, changed, "the other school's request must remain untouched")
		return failAfterWrite
	})
	require.ErrorIs(t, err, failAfterWrite)
	for _, expected := range []int{1, 0} {
		err = testpkg.WithTenantTx(t, context.Background(), db, tenantIDs[0], func(ctx context.Context, _ bun.Tx) error {
			changed, commandErr := module.BackfillGuardianAccountID(ctx, account.ID, email)
			require.NoError(t, commandErr)
			require.Equal(t, expected, changed, "rollback must restore the request and committed retries must be idempotent")
			return nil
		})
		require.NoError(t, err)
	}
	err = testpkg.WithTenantTx(t, context.Background(), db, tenantIDs[1], func(ctx context.Context, _ bun.Tx) error {
		changed, commandErr := module.BackfillGuardianAccountID(ctx, account.ID, email)
		require.NoError(t, commandErr)
		require.Equal(t, 1, changed, "the second school's request must still be unclaimed")
		return nil
	})
	require.NoError(t, err)
}
