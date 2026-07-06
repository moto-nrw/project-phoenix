package schedule_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// GetUsageCounts Tests
// =============================================================================

func TestCalendarPeriodService_GetUsageCounts(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	svc := setupCalendarPeriodService(t, db)
	ctx := testpkg.TenantContext(1)

	suffix := time.Now().UnixNano()

	used := &scheduleModels.CalendarPeriod{
		Name:            fmt.Sprintf("Usage-Used-%d", suffix),
		PeriodType:      scheduleModels.PeriodTypeSemester,
		StartDate:       timezone.NewDate(2026, 8, 1),
		EndDate:         timezone.NewDate(2027, 1, 31),
		WeekCycleLength: 1,
		IsActive:        true,
	}
	require.NoError(t, svc.CreatePeriod(ctx, used))

	unused := &scheduleModels.CalendarPeriod{
		Name:            fmt.Sprintf("Usage-Unused-%d", suffix),
		PeriodType:      scheduleModels.PeriodTypeSemester,
		StartDate:       timezone.NewDate(2027, 2, 1),
		EndDate:         timezone.NewDate(2027, 7, 31),
		WeekCycleLength: 1,
		IsActive:        false,
	}
	require.NoError(t, svc.CreatePeriod(ctx, unused))
	defer testpkg.CleanupTableRecords(t, db, "schedule.calendar_periods", used.ID, unused.ID)

	bg := context.Background()

	phase := &enrollmentModels.Phase{
		Name:                      fmt.Sprintf("Usage-Phase-%d", suffix),
		Kind:                      enrollmentModels.PhaseKindSchoolYear,
		ServiceStartDate:          timezone.NewDate(2026, 8, 1),
		ServiceEndDate:            timezone.NewDate(2027, 1, 31),
		CareOverflowMode:          enrollmentModels.PhaseCareOverflowWaitlist,
		CareOfferingSelectionMode: enrollmentModels.PhaseCareOfferingSelectionOptional,
		IsActive:                  true,
		CalendarPeriodID:          &used.ID,
	}
	phase.SetTenantID(1)
	_, err := db.NewInsert().
		Model(phase).
		ModelTableExpr("enrollment.phases").
		Returning("id").
		Exec(bg)
	require.NoError(t, err, "Failed to create test phase")
	defer testpkg.CleanupTableRecords(t, db, "enrollment.phases", phase.ID)

	group := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Usage-Group-%d", suffix))
	defer testpkg.CleanupActivityFixtures(t, db, group.ID)

	sched := &activitiesModels.Schedule{
		Weekday:          activitiesModels.WeekdayMonday,
		ActivityGroupID:  group.ID,
		WeekPattern:      0,
		CalendarPeriodID: &used.ID,
	}
	sched.SetTenantID(1)
	_, err = db.NewInsert().
		Model(sched).
		ModelTableExpr("activities.schedules").
		Returning("id").
		Exec(bg)
	require.NoError(t, err, "Failed to create test schedule")
	defer testpkg.CleanupTableRecords(t, db, "activities.schedules", sched.ID)

	usage, err := svc.GetUsageCounts(ctx)
	require.NoError(t, err)

	assert.Equal(t, scheduleModels.CalendarPeriodUsage{
		EnrollmentPhases: 1,
		Schedules:        1,
	}, usage[used.ID], "used period must report one phase and one schedule")

	_, ok := usage[unused.ID]
	assert.False(t, ok, "period without references must be omitted from the map")
}
