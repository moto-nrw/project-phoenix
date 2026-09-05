package schedule_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalendarPeriodMutationCareOfferingPreflightLeavesPeriodUnchanged(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, cleanupErr := db.NewDelete().
			TableExpr("schedule.calendar_periods").
			Where("tenant_id = ?", tenantID).
			Exec(cleanupCtx)
		require.NoError(t, cleanupErr)
		_, cleanupErr = db.NewDelete().TableExpr("platform.schools").Where("id = ?", tenantID).Exec(cleanupCtx)
		require.NoError(t, cleanupErr)
		_, cleanupErr = db.NewDelete().TableExpr("platform.organizations").Where("id = ?", tenantID).Exec(cleanupCtx)
		require.NoError(t, cleanupErr)
	})
	ctx := testpkg.TenantContext(tenantID)

	repo := repositories.NewFactory(db).CalendarPeriod
	period := &scheduleModel.CalendarPeriod{
		Name:            fmt.Sprintf("Care-link-preflight-%d", time.Now().UnixNano()),
		PeriodType:      scheduleModel.PeriodTypeSchoolYear,
		StartDate:       scheduleModel.NewDate(2026, time.August, 1),
		EndDate:         scheduleModel.NewDate(2027, time.July, 31),
		WeekCycleLength: 1,
		IsActive:        true,
	}
	period.SetTenantID(tenantID)
	require.NoError(t, repo.Create(ctx, period))

	var replacements []*scheduleModel.CalendarPeriod
	service := scheduleSvc.NewCalendarPeriodServiceWithConfig(scheduleSvc.CalendarPeriodServiceConfig{
		Repo: repo,
		DB:   db,
		ValidateCareOfferingChange: func(callCtx context.Context, gotPeriodID int64, replacement *scheduleModel.CalendarPeriod) error {
			require.Equal(t, period.ID, gotPeriodID)
			_, hasTx := tenant.TransactionFromContext(callCtx)
			require.True(t, hasTx, "preflight must run under the mutation transaction")
			replacements = append(replacements, replacement)
			return scheduleModel.ErrCalendarPeriodCareOfferingConflict
		},
	})

	updated := *period
	updated.StartDate = scheduleModel.NewDate(2026, time.September, 1)
	updated.EndDate = scheduleModel.NewDate(2027, time.June, 30)
	err := service.UpdatePeriod(ctx, &updated)
	require.ErrorIs(t, err, scheduleModel.ErrCalendarPeriodCareOfferingConflict)

	stored, err := repo.FindByID(ctx, period.ID)
	require.NoError(t, err)
	assert.Equal(t, period.StartDate, stored.StartDate)
	assert.Equal(t, period.EndDate, stored.EndDate)
	require.Len(t, replacements, 1)
	assert.NotNil(t, replacements[0], "update preflight receives the proposed replacement")

	err = service.DeletePeriod(ctx, period.ID)
	require.ErrorIs(t, err, scheduleModel.ErrCalendarPeriodCareOfferingConflict)
	stored, err = repo.FindByID(ctx, period.ID)
	require.NoError(t, err)
	assert.Equal(t, period.ID, stored.ID, "rejected delete must leave the period present")
	require.Len(t, replacements, 2)
	assert.Nil(t, replacements[1], "delete preflight is represented by a nil replacement")

	assert.False(t, errors.Is(
		&scheduleSvc.ScheduleError{Op: "probe", Err: errors.New("database unavailable")},
		scheduleModel.ErrCalendarPeriodCareOfferingConflict,
	), "infrastructure errors must not be classified as care-link conflicts")
}
