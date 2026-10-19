package parentrequestfeedprojection

import (
	"context"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestParentRequestFeedReturnsOnlyTheNamedSchoolsPendingRows(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	type side struct{ tenantID, requestID int64 }
	var sides []side
	instant := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	for _, name := range []string{"own", "foreign"} {
		t.Run(name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			chain := testpkg.CreateTestParentGuardianChain(t, db)
			value := side{tenantID: chain.TenantID}
			require.NoError(t, db.NewRaw(`INSERT INTO users.student_data_change_requests (tenant_id,student_id,submitted_by,target,field_key,new_value,status,created_at) VALUES (?,?,?,'student','school_class','"2a"'::jsonb,'pending',?) RETURNING id`, chain.TenantID, chain.StudentID, chain.AccountID, instant).Scan(testpkg.Ctx(t), &value.requestID))
			sides = append(sides, value)
		})
	}
	for _, side := range sides {
		check := func(ctx context.Context, db bun.IDB) {
			t.Helper()
			rows, err := ListNewParentRequests(ctx, db, side.tenantID, instant.Add(-time.Minute), true, true)
			require.NoError(t, err)
			require.Len(t, rows, 1)
			require.Equal(t, "master_data", rows[0].Kind)
			require.Equal(t, side.requestID, rows[0].ID)
			require.True(t, instant.Equal(rows[0].CreatedAt))
		}
		check(testpkg.Ctx(t), db)
		require.NoError(t, testpkg.WithTenantTx(t, context.Background(), db, side.tenantID, func(ctx context.Context, tx bun.Tx) error { check(ctx, tx); return nil }))
	}
}
