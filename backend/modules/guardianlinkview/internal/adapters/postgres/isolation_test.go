package guardianlinkview_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/guardianlinkview/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type linkRow struct {
	bun.BaseModel     `bun:"alias:student_guardian"`
	ID                int64           `bun:"id"`
	StudentID         int64           `bun:"student_id"`
	GuardianProfileID int64           `bun:"guardian_profile_id"`
	CanPickup         bool            `bun:"can_pickup"`
	AccessAccountID   int64           `bun:"access_account_id"`
	Permissions       map[string]bool `bun:"permissions,type:jsonb"`
}

func TestGuardianProjectionJoinsPermissionsAndPickupWithoutCrossingTenants(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	var chains []testpkg.ParentChain
	for _, side := range []string{"own", "foreign"} {
		t.Run(side, func(t *testing.T) {
			testpkg.OwnTenant(t)
			chains = append(chains, testpkg.CreateTestParentGuardianChain(t, db))
		})
	}
	for _, chain := range chains {
		check := func(ctx context.Context, db bun.IDB) {
			t.Helper()
			var rows []linkRow
			require.NoError(t, compose.NewReader().GuardianLinkModelRows(db, chain.TenantID, &rows).Where("student_guardian.student_id IN (?, ?)", chains[0].StudentID, chains[1].StudentID).Scan(ctx))
			require.Len(t, rows, 1)
			require.Equal(t, chain.StudentID, rows[0].StudentID)
			require.Equal(t, chain.GuardianProfileID, rows[0].GuardianProfileID)
			require.Equal(t, chain.AccountID, rows[0].AccessAccountID)
			require.True(t, rows[0].CanPickup)
			require.True(t, rows[0].Permissions["parent_portal.access"])
			var ids []int64
			require.NoError(t, compose.NewReader().GuardianLinkRows(db, chain.TenantID).Column("id").Where("student_guardian.student_id IN (?, ?)", chains[0].StudentID, chains[1].StudentID).Scan(ctx, &ids))
			require.Equal(t, []int64{rows[0].ID}, ids)
		}
		check(testpkg.Ctx(t), db)
		require.NoError(t, testpkg.WithTenantTx(t, context.Background(), db, chain.TenantID, func(ctx context.Context, tx bun.Tx) error { check(ctx, tx); return nil }))
	}
}
