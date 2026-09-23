package timetableplanning_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The lifecycle, deviation and move suites below run over one materialized
// template. The materialization itself is the Timetable owner's since slice
// S2 of #3424; its suites live in modules/timetable/compose/httpintegration
// with their own copy of this scenario.

// scenarioSetup bundles the Timetable owner's materialization plus convenient
// references to the common fixtures.
type scenarioSetup struct {
	svc           timetable.MaterializationCapability
	factory       *services.Factory
	db            *bun.DB
	ctx           context.Context
	tenantID      int64
	period        *scheduleModels.CalendarPeriod
	template      *activitiesModels.Group
	schedule      *activitiesModels.Schedule
	timeframe     *scheduleModels.Timeframe
	students      []int64
	staffID       int64
	categoryID    int64
	roomID        int64
	enrollmentIDs []int64
	supervisorIDs []int64
	extraCleanups []func()
}

func (s *scenarioSetup) runCleanup(tb testing.TB) {
	tb.Helper()
	for _, fn := range s.extraCleanups {
		fn()
	}
}

// makeScenario prepares a minimal end-to-end materialization scenario:
// one active school-year period (no A/B cycle), one template on
// the given weekday, one timeframe 14:00–15:00, one room, one staff, three
// students of which two have valid enrollments at `materializeDate` and one
// has expired.
func makeScenario(t *testing.T, weekday int, materializeDate timezone.Date) *scenarioSetup {
	t.Helper()
	db := testpkg.SetupTestDB(t)

	repoFactory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	serviceFactory, err := services.NewFactoryForTests(repoFactory, db, slog.Default(), func() time.Time {
		return time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	})
	require.NoError(t, err)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	ctx := testpkg.TenantContext(tenantID)
	suffix := time.Now().UnixNano()

	period := &scheduleModels.CalendarPeriod{
		Name:            fmt.Sprintf("Schuljahr-%d", suffix),
		PeriodType:      scheduleModels.PeriodTypeSchoolYear,
		StartDate:       scheduleModels.NewDate(materializeDate.Year()-1, 8, 1),
		EndDate:         scheduleModels.NewDate(materializeDate.Year()+1, 7, 31),
		WeekCycleLength: 1,
		IsActive:        true,
	}
	require.NoError(t, repoFactory.CalendarPeriod.Create(ctx, period))

	room := testpkg.CreateTestRoomForTenant(t, db, tenantID, fmt.Sprintf("Room-%d", suffix))
	staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Super", fmt.Sprintf("Visor-%d", suffix))
	student1 := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Alice", fmt.Sprintf("One-%d", suffix), "3a")
	student2 := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Bob", fmt.Sprintf("Two-%d", suffix), "3a")
	student3 := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Carol", fmt.Sprintf("Three-%d", suffix), "3a")

	category := testpkg.CreateTestActivityCategoryForTenant(t, db, tenantID, fmt.Sprintf("Cat-%d", suffix))
	template := &activitiesModels.Group{
		Name:            fmt.Sprintf("Malen-AG-%d", suffix),
		MaxParticipants: 20,
		IsOpen:          true,
		CategoryID:      category.ID,
		CreatedBy:       &staff.ID,
		PlannedRoomID:   &room.ID,
		IsTemplate:      true,
	}
	template.SetTenantID(tenantID)
	_, err = db.NewInsert().Model(template).ModelTableExpr(`activities.groups AS "group"`).Exec(ctx)
	require.NoError(t, err)

	timeframe := testpkg.CreateTestTimeframeForTenant(t, db, tenantID, fmt.Sprintf("Nachmittag-%d", suffix))
	sched := &activitiesModels.Schedule{
		Weekday:         weekday,
		TimeframeID:     &timeframe.ID,
		ActivityGroupID: template.ID,
		WeekPattern:     0,
	}
	sched.SetTenantID(tenantID)
	_, err = db.NewInsert().Model(sched).ModelTableExpr(`activities.schedules`).Exec(ctx)
	require.NoError(t, err)

	// Enrollments: student1 + student2 valid unbounded; student3 expired the
	// day before materializeDate.
	expiredUntil := activitiesModels.Date(materializeDate.AddDays(-1))
	validFrom := activitiesModels.Date(materializeDate.AddDays(-30))
	enrollments := []*activitiesModels.StudentEnrollment{
		{StudentID: student1.ID, ActivityGroupID: template.ID, ValidFrom: validFrom},
		{StudentID: student2.ID, ActivityGroupID: template.ID, ValidFrom: validFrom},
		{StudentID: student3.ID, ActivityGroupID: template.ID, ValidFrom: validFrom, ValidUntil: &expiredUntil},
	}
	enrollmentIDs := make([]int64, 0, len(enrollments))
	for _, enrollment := range enrollments {
		enrollment.SetTenantID(tenantID)
		_, err = db.NewInsert().Model(enrollment).ModelTableExpr(`activities.student_enrollments`).ExcludeColumn("selected_weekdays").Exec(ctx)
		require.NoError(t, err)
		enrollmentIDs = append(enrollmentIDs, enrollment.ID)
	}

	sup := &activitiesModels.SupervisorPlanned{StaffID: staff.ID, GroupID: template.ID, IsPrimary: true, ValidFrom: validFrom}
	sup.SetTenantID(tenantID)
	_, err = db.NewInsert().Model(sup).ModelTableExpr(`activities.supervisors`).Exec(ctx)
	require.NoError(t, err)

	return &scenarioSetup{
		svc:           serviceFactory.Materialization,
		factory:       serviceFactory,
		db:            db,
		ctx:           ctx,
		tenantID:      tenantID,
		period:        period,
		template:      template,
		schedule:      sched,
		timeframe:     timeframe,
		students:      []int64{student1.ID, student2.ID, student3.ID},
		staffID:       staff.ID,
		categoryID:    category.ID,
		roomID:        room.ID,
		enrollmentIDs: enrollmentIDs,
		supervisorIDs: []int64{sup.ID},
	}
}

func listInstancesForDate(tb testing.TB, db *bun.DB, templateID int64, date timezone.Date) []*scheduleModels.ActivityInstance {
	tb.Helper()
	var rows []*scheduleModels.ActivityInstance
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := db.NewSelect().
		Model(&rows).
		ModelTableExpr(`schedule.activity_instances AS "activity_instance"`).
		Where(`"activity_instance".activity_group_id = ?`, templateID).
		Where(`"activity_instance".date = ?`, date).
		Order("start_time ASC").
		Scan(ctx)
	require.NoError(tb, err)
	return rows
}

// splitInsertInstance inserts a raw schedule.activity_instances row for the
// given (possibly nil) activity group at a distinct start hour.
func splitInsertInstance(t *testing.T, s *scenarioSetup, groupID *int64, date timezone.Date, status string, spontaneous bool, startHour int) int64 {
	t.Helper()
	row := &scheduleModels.ActivityInstance{
		Date:            scheduleModels.Date(date),
		Title:           fmt.Sprintf("Split-%s-%d", status, time.Now().UnixNano()),
		StartTime:       time.Date(1, 1, 1, startHour, 0, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, startHour+1, 0, 0, 0, time.UTC),
		RoomID:          s.roomID,
		Status:          status,
		IsSpontaneous:   spontaneous,
		ActivityGroupID: groupID,
	}
	row.SetTenantID(s.tenantID)
	_, err := s.db.NewInsert().Model(row).ModelTableExpr(`schedule.activity_instances`).Exec(s.ctx)
	require.NoError(t, err)
	return row.ID
}
