package schedule

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalendarPeriodUsageRepository_HonorsTenantContextWithoutRepositoryRuntime(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := NewCalendarPeriodUsageRepository(db)
	tenantID := testpkg.Tenant(t)
	period := testpkg.CreateTestCalendarPeriod(t, db, "Mandant", testpkg.Date(2030, time.August, 1), testpkg.Date(2031, time.July, 31))
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
}
