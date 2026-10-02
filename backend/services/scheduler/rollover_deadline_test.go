package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

var errRolloverDeadlineProbe = errors.New("rollover deadline probe failed")

// failingRolloverDeadlineProbe writes a timeframe row in the tenant
// transaction the scheduler opened and then fails, like a deadline worker
// that wrote part of its tick.
type failingRolloverDeadlineProbe struct {
	description string
	calls       int
}

func (p *failingRolloverDeadlineProbe) RunDeadlineWorker(ctx context.Context, _ time.Time) (any, error) {
	p.calls++
	transaction, ok := testpkg.TransactionFromContext(ctx)
	if !ok {
		return nil, errors.New("rollover deadline probe runs outside a tenant transaction")
	}
	tx, ok := transaction.(bun.IDB)
	if !ok {
		return nil, fmt.Errorf("tenant transaction has type %T", transaction)
	}
	tenantID := testpkg.TenantIDFromContext(ctx)
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schedule.timeframes (tenant_id, start_time, end_time, is_active, description)
		 VALUES (?, '08:00:00', '09:00:00', TRUE, ?)`,
		tenantID, fmt.Sprintf("%s-%d", p.description, tenantID)); err != nil {
		return nil, err
	}
	return nil, errRolloverDeadlineProbe
}

func TestRolloverDeadlineWorkerErrorRollsBackTenantTick(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	testpkg.EnsureTestTenant(t, db, testpkg.Tenant(t))
	probe := &failingRolloverDeadlineProbe{
		description: fmt.Sprintf("rollover-deadline-rollback-%d", time.Now().UnixNano()),
	}
	s := unitScheduler(&Scheduler{
		schoolRepo:             dbTenantDirectory{db: db},
		tenantRuntime:          dbTenantRuntime(t, db),
		rolloverDeadlineRunner: probe,
		logger:                 slog.Default()})

	s.checkAndRunRolloverDeadline(context.Background(), &ScheduledTask{})

	assert.GreaterOrEqual(t, probe.calls, 1)
	var rows int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM schedule.timeframes WHERE description LIKE ?`,
		probe.description+"%").Scan(context.Background(), &rows))
	assert.Zero(t, rows,
		"returning the worker error must roll back writes made in that tenant tick")
}
