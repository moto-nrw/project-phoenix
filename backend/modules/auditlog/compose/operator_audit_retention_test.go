package compose

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The operator ledger keeps an entry until the end of the year after it was
// written (#4047), like the access log of #3956. The hourly cleanup runs the
// deletion in an administrative transaction, without a school.
func TestOperatorAuditLogLosesEntriesAfterTheFollowingYear(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	persistence, err := NewTenantPersistence(db)
	require.NoError(t, err)
	command, err := NewObservedCommand(persistence.AppendStore, slog.Default(), func(string, time.Duration, int, error) {})
	require.NoError(t, err)

	operator := testpkg.CreateTestOperator(t, db)
	insert := func(at time.Time) int64 {
		var id int64
		require.NoError(t, db.NewRaw(`INSERT INTO platform.operator_audit_log
			(operator_id, action, resource_type, request_ip, created_at)
			VALUES (?, 'login', 'operator', '10.0.0.1', ?) RETURNING id`, operator.ID, at).Scan(ctx, &id))
		return id
	}
	// Three years back is always before 1 January of last year; half a year
	// back never is. The exact cutoff is pinned in services/audit.
	now := time.Now()
	expired := insert(now.AddDate(-3, 0, 0))
	halfYear := insert(now.AddDate(0, -6, 0))
	recent := insert(now)

	var deleted int
	require.NoError(t, tenant.WithinAdmin(testpkg.WithTenantRuntime(t, ctx, db), func(adminCtx context.Context) error {
		deleted, err = command.DeleteExpiredOperatorAuditLog(adminCtx)
		return err
	}))
	assert.GreaterOrEqual(t, deleted, 1)

	var remaining []int64
	require.NoError(t, db.NewRaw(`SELECT id FROM platform.operator_audit_log WHERE operator_id = ? ORDER BY id`, operator.ID).
		Scan(ctx, &remaining))
	assert.Equal(t, []int64{halfYear, recent}, remaining, "only the entry before 1 January of last year goes")
	assert.NotContains(t, remaining, expired)

	_, err = command.DeleteExpiredOperatorAuditLog(ctx)
	require.Error(t, err, "outside a transaction the deletion is refused")
}
