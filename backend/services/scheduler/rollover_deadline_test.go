package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errRolloverDeadlineProbe = errors.New("rollover deadline probe failed")

type failingRolloverDeadlineProbe struct {
	repo        scheduleModels.TimeframeRepository
	description string
	calls       int
}

func (p *failingRolloverDeadlineProbe) RunDeadlineWorker(ctx context.Context, _ time.Time) (any, error) {
	p.calls++
	start := time.Date(2000, 1, 1, 8, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	row := &scheduleModels.Timeframe{
		StartTime:   start,
		EndTime:     &end,
		IsActive:    true,
		Description: fmt.Sprintf("%s-%d", p.description, tenant.FromContext(ctx)),
	}
	row.SetTenantID(tenant.FromContext(ctx))
	if err := p.repo.Create(ctx, row); err != nil {
		return nil, err
	}
	return nil, errRolloverDeadlineProbe
}

func TestRolloverDeadlineWorkerErrorRollsBackTenantTick(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	testpkg.EnsureTestTenant(t, db, testpkg.Tenant(t))
	repos := repositories.NewFactory(db)
	probe := &failingRolloverDeadlineProbe{
		repo:        repos.Timeframe,
		description: fmt.Sprintf("rollover-deadline-rollback-%d", time.Now().UnixNano()),
	}
	s := &Scheduler{
		db:                     db,
		schoolRepo:             repos.School,
		rolloverDeadlineRunner: probe,
		logger:                 slog.Default(),
	}

	s.checkAndRunRolloverDeadline(&ScheduledTask{})

	assert.GreaterOrEqual(t, probe.calls, 1)
	rows, err := repos.Timeframe.FindByDescription(testpkg.Ctx(t), probe.description)
	require.NoError(t, err)
	assert.Empty(t, rows,
		"returning the worker error must roll back writes made in that tenant tick")
}
