package schedule_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type phaseCountQueries struct {
	counts map[int64]int
	err    error
}

func (q phaseCountQueries) PhaseCountsByCalendarPeriod(context.Context) (map[int64]int, error) {
	return q.counts, q.err
}

func TestCalendarPeriodUsageRepository_EnrollmentFailurePropagates(t *testing.T) {
	t.Parallel()
	failure := errors.New("enrollment read failed")
	repo := scheduleRepo.NewCalendarPeriodUsageRepository(nil, phaseCountQueries{err: failure}, phaseCountQueries{}.PhaseCountsByCalendarPeriod)
	_, err := repo.UsageCounts(context.Background())
	require.ErrorIs(t, err, failure)
}

func TestCalendarPeriodUsageRepository_HonorsTenantContextWithoutRepositoryRuntime(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	assignments := repositories.NewUnobservedTimetableDependencies(db).Capability
	tenantID := testpkg.Tenant(t)
	period := testpkg.CreateTestCalendarPeriod(t, db, "Mandant", testpkg.Date(2030, time.August, 1), testpkg.Date(2031, time.July, 31))
	repo := scheduleRepo.NewCalendarPeriodUsageRepository(db, phaseCountQueries{counts: map[int64]int{period.ID: 2}}, assignments.CountPlannedSupervisorsByCalendarPeriod)
	group := testpkg.CreateTestActivityGroup(t, db, "Mandanten-AG")
	_, err := db.NewUpdate().TableExpr("activities.groups").
		Set("calendar_period_id = ?", period.ID).
		Where("id = ?", group.ID).
		Exec(context.Background())
	require.NoError(t, err)

	otherTenantID, _ := testpkg.CreateTestTenant(t, db)
	var foreignPeriodID int64
	err = db.NewRaw(`
		INSERT INTO schedule.calendar_periods (
			tenant_id, name, period_type, start_date, end_date, week_cycle_length, is_active
		)
		SELECT ?, 'Fremder Mandant', period_type, start_date, end_date, week_cycle_length, is_active
		FROM schedule.calendar_periods
		WHERE id = ?
		RETURNING id
	`, otherTenantID, period.ID).Scan(context.Background(), &foreignPeriodID)
	require.NoError(t, err)
	foreignGroup := testpkg.CreateTestActivityGroupForTenant(t, db, otherTenantID, "Fremde AG")
	_, err = db.NewUpdate().TableExpr("activities.groups").
		Set("calendar_period_id = ?", foreignPeriodID).
		Where("id = ?", foreignGroup.ID).
		Exec(context.Background())
	require.NoError(t, err)

	usage, err := repo.UsageCounts(tenant.WithTenantID(context.Background(), tenantID))
	require.NoError(t, err)
	assert.Equal(t, 1, usage[period.ID].ActivityGroups)
	assert.Equal(t, 2, usage[period.ID].EnrollmentPhases)
	assert.NotContains(t, usage, foreignPeriodID)
}
