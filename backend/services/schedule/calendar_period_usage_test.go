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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	svc := setupCalendarPeriodService(t, db)
	ctx := testpkg.Ctx(t)

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

	groupOnly := &scheduleModels.CalendarPeriod{
		Name:            fmt.Sprintf("Usage-Group-Only-%d", suffix),
		PeriodType:      scheduleModels.PeriodTypeSemester,
		StartDate:       timezone.NewDate(2027, 8, 1),
		EndDate:         timezone.NewDate(2028, 1, 31),
		WeekCycleLength: 1,
		IsActive:        false,
	}
	require.NoError(t, svc.CreatePeriod(ctx, groupOnly))

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
	phase.SetTenantID(testpkg.Tenant(t))
	_, err := db.NewInsert().
		Model(phase).
		ModelTableExpr("enrollment.phases").
		Returning("id").
		Exec(bg)
	require.NoError(t, err, "Failed to create test phase")

	group := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Usage-Group-%d", suffix))
	student := testpkg.CreateTestStudent(t, db, "Usage", fmt.Sprintf("Student-%d", suffix), "1a")
	staff := testpkg.CreateTestStaff(t, db, "Usage", fmt.Sprintf("Staff-%d", suffix))
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Usage-Room-%d", suffix))
	_, err = db.NewUpdate().
		Model(group).
		ModelTableExpr(`activities.groups AS "group"`).
		Set("calendar_period_id = ?", used.ID).
		Where(`"group".id = ?`, group.ID).
		Exec(bg)
	require.NoError(t, err, "Failed to link test activity group to calendar period")

	groupOnlyFixture := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Usage-Group-Only-Fixture-%d", suffix))
	_, err = db.NewUpdate().
		Model(groupOnlyFixture).
		ModelTableExpr(`activities.groups AS "group"`).
		Set("calendar_period_id = ?", groupOnly.ID).
		Where(`"group".id = ?`, groupOnlyFixture.ID).
		Exec(bg)
	require.NoError(t, err, "Failed to create group-only calendar-period usage")

	sched := &activitiesModels.Schedule{
		Weekday:          activitiesModels.WeekdayMonday,
		ActivityGroupID:  group.ID,
		WeekPattern:      0,
		CalendarPeriodID: &used.ID,
	}
	sched.SetTenantID(testpkg.Tenant(t))
	_, err = db.NewInsert().
		Model(sched).
		ModelTableExpr("activities.schedules").
		Returning("id").
		Exec(bg)
	require.NoError(t, err, "Failed to create test schedule")

	enrollment := &activitiesModels.StudentEnrollment{
		StudentID:        student.ID,
		ActivityGroupID:  group.ID,
		ValidFrom:        timezone.NewDate(2026, 8, 1),
		CalendarPeriodID: &used.ID,
	}
	enrollment.SetTenantID(testpkg.Tenant(t))
	_, err = db.NewInsert().
		Model(enrollment).
		ModelTableExpr("activities.student_enrollments").
		Returning("id").
		Exec(bg)
	require.NoError(t, err, "Failed to create test student enrollment")

	supervisor := &activitiesModels.SupervisorPlanned{
		StaffID:          staff.ID,
		GroupID:          group.ID,
		ValidFrom:        timezone.NewDate(2026, 8, 1),
		CalendarPeriodID: &used.ID,
	}
	supervisor.SetTenantID(testpkg.Tenant(t))
	_, err = db.NewInsert().
		Model(supervisor).
		ModelTableExpr("activities.supervisors").
		Returning("id").
		Exec(bg)
	require.NoError(t, err, "Failed to create test supervisor")

	instance := testpkg.CreateTestActivityInstance(t, db,
		timezone.NewDate(2026, 8, 3),
		room.ID,
		testpkg.ActivityInstanceOpts{ActivityGroupID: &group.ID},
	)
	_, err = db.NewUpdate().
		Model(instance).
		ModelTableExpr("schedule.activity_instances").
		Set("calendar_period_id = ?", used.ID).
		Where("id = ?", instance.ID).
		Exec(bg)
	require.NoError(t, err, "Failed to link test activity instance to calendar period")

	usage, err := svc.GetUsageCounts(ctx)
	require.NoError(t, err)

	assert.Equal(t, scheduleModels.CalendarPeriodUsage{
		EnrollmentPhases:   1,
		ActivityGroups:     1,
		Schedules:          1,
		StudentEnrollments: 1,
		Supervisors:        1,
		ActivityInstances:  1,
	}, usage[used.ID], "used period must report every calendar-period reference")
	assert.Equal(t, scheduleModels.CalendarPeriodUsage{
		ActivityGroups: 1,
	}, usage[groupOnly.ID], "a group-level reference alone must make the period used")

	_, ok := usage[unused.ID]
	assert.False(t, ok, "period without references must be omitted from the map")
}
