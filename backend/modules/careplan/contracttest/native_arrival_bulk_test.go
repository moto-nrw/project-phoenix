package contracttest_test

import (
	"context"
	"maps"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type arrivalClassStudents struct {
	compose.ArrivalBulkStudents
	rows   []compose.ArrivalBulkStudent
	events *[]string
	locked []int64
}

func (d *arrivalClassStudents) ByClass(context.Context, string, calendar.Date) ([]compose.ArrivalBulkStudent, error) {
	return d.rows, nil
}
func (d *arrivalClassStudents) LockByIDs(_ context.Context, ids []int64, _ calendar.Date) (map[int64]compose.ArrivalBulkStudent, error) {
	*d.events = append(*d.events, "students")
	d.locked = append([]int64(nil), ids...)
	rows := make(map[int64]compose.ArrivalBulkStudent)
	for _, row := range d.rows {
		rows[row.ID] = row
	}
	return rows, nil
}

type arrivalClassPlansFixture struct {
	row    compose.ArrivalClassPlan
	events *[]string
}

func (p *arrivalClassPlansFixture) LockClass(context.Context, string) error {
	*p.events = append(*p.events, "class")
	return nil
}
func (p *arrivalClassPlansFixture) FindByClasses(context.Context, []string) ([]*compose.ArrivalClassPlan, error) {
	*p.events = append(*p.events, "read")
	copy := p.row
	copy.ArrivalTimes = maps.Clone(copy.ArrivalTimes)
	return []*compose.ArrivalClassPlan{&copy}, nil
}
func (p *arrivalClassPlansFixture) Upsert(_ context.Context, row *compose.ArrivalClassPlan) error {
	*p.events = append(*p.events, "write")
	p.row = *row
	return nil
}

func TestNativeArrivalClassUpdateAuthorizesBeforeClassLockAndPreservesDeviations(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	owner := careplantest.NewCarePlan(t, db)
	first := testpkg.CreateTestStudent(t, db, "Class", "First", "1a")
	second := testpkg.CreateTestStudent(t, db, "Class", "Second", "1a")
	staff := testpkg.CreateTestStaff(t, db, "Class", "Staff")
	clock := time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC)
	_, err := owner.CreateArrivalSchedule(ctx, careplan.ArrivalSchedule{StudentID: first.ID, Weekday: 1, ExpectedArrival: clock, CreatedBy: staff.ID})
	require.NoError(t, err)
	var events []string
	directory := &arrivalClassStudents{events: &events, rows: []compose.ArrivalBulkStudent{
		{ScheduleStudent: careplan.ScheduleStudent{ID: second.ID, TenantID: testpkg.Tenant(t)}, SchoolClass: "1a"},
		{ScheduleStudent: careplan.ScheduleStudent{ID: first.ID, TenantID: testpkg.Tenant(t)}, SchoolClass: " 1A "},
	}}
	classes := &arrivalClassPlansFixture{events: &events, row: compose.ArrivalClassPlan{SchoolClass: "1a", ArrivalTimes: map[string]string{"mon": "13:00", "tue": "12:45", "wed": "12:15"}}}
	service, err := compose.NewArrivalSchedules(db, owner, nil, nil, compose.ArrivalScheduleDependencies{Students: directory, Classes: classes})
	require.NoError(t, err)
	filter := careplan.ArrivalScheduleBulkFilter{SchoolClass: " 1a ", Authorize: func(_ context.Context, row careplan.ScheduleStudent) (bool, error) {
		events = append(events, "authorize")
		return row.ID == first.ID, nil
	}}
	patch := []careplan.ArrivalScheduleInput{{Weekday: 1, ArrivalTime: " 11:30 "}, {Weekday: 2, ArrivalTime: " "}}
	_, err = service.BulkUpsertArrivalSchedules(ctx, filter, patch, staff.ID)
	require.ErrorIs(t, err, careplan.ErrBulkStudentUnauthorized)
	require.Equal(t, []string{"students", "authorize", "authorize"}, events)
	require.Equal(t, "13:00", classes.row.ArrivalTimes["mon"])
	events = nil
	filter.Authorize = func(context.Context, careplan.ScheduleStudent) (bool, error) {
		events = append(events, "authorize")
		return true, nil
	}
	result, err := service.BulkUpsertArrivalSchedules(ctx, filter, patch, staff.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"students", "authorize", "authorize", "class", "read", "write"}, events)
	require.Equal(t, []int64{first.ID, second.ID}, directory.locked)
	require.Equal(t, 2, result.StudentsAffected)
	require.Equal(t, map[string]string{"mon": "11:30", "wed": "12:15"}, classes.row.ArrivalTimes)
	require.Equal(t, &staff.ID, classes.row.UpdatedBy)
	rows, err := owner.ListArrivalSchedules(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{first.ID, second.ID}})
	require.NoError(t, err)
	require.Len(t, rows, 1, "a class edit must not create per-child schedule rows")
	require.Equal(t, "14:00", rows[0].ExpectedArrival.Format("15:04"), "an existing deviation remains unchanged")
}
